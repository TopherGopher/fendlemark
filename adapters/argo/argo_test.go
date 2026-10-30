package argo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/adapter/conformance"
)

// TestMain keeps every test away from any real cluster or credential plugin
// on the machine running them; token discovery would otherwise read the
// default kubeconfig. Tests that want a kubeconfig set their own.
func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "argo-test")
	os.Setenv("KUBECONFIG", filepath.Join(dir, "absent"))
	os.Setenv("KUBERNETES_SERVICE_HOST", "")
	os.Unsetenv("ARGO_TOKEN")
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// kubeconfigFor points KUBECONFIG at server with a bearer token.
func kubeconfigFor(t *testing.T, server, token string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	kc := fmt.Sprintf("current-context: lab\ncontexts:\n- name: lab\n  context: {cluster: lab, user: me}\nclusters:\n- name: lab\n  cluster: {server: %q}\nusers:\n- name: me\n  user: {token: %q}\n", server, token)
	if err := os.WriteFile(path, []byte(kc), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)
	return path
}

func fixture(t *testing.T) *httptest.Server {
	t.Helper()
	return fixtureWithArchive(t, `{"items":[],"metadata":{}}`)
}

func fixtureWithArchive(t *testing.T, archive string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/info", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"managedNamespace":"ml-training"}`)) })
	mux.HandleFunc("/api/v1/workflows/ml-training", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{
			{"metadata": map[string]any{"name": "train-abc12", "namespace": "ml-training", "uid": "u-1", "creationTimestamp": "2026-09-30T09:00:00Z",
				"labels": map[string]string{"pipeline/runid": "kfp-run-77", "app.kubernetes.io/version": "3e1f9c0"}},
				"spec": map[string]any{"entrypoint": "train"},
				"status": map[string]any{"phase": "Succeeded", "startedAt": "2026-09-30T09:00:00Z", "finishedAt": "2026-09-30T09:20:00Z",
					"nodes": map[string]any{"train-abc12": map[string]any{"name": "train-abc12", "displayName": "train", "type": "Pod", "phase": "Succeeded", "templateName": "train", "startedAt": "2026-09-30T09:00:05Z", "finishedAt": "2026-09-30T09:19:50Z"}}}},
			{"metadata": map[string]any{"name": "train-xyz99", "namespace": "ml-training", "uid": "u-2", "creationTimestamp": "2026-09-30T11:00:00Z"},
				"status": map[string]any{"phase": "Running", "startedAt": "2026-09-30T11:00:00Z", "nodes": map[string]any{}}},
		}, "metadata": map[string]any{}})
	})
	mux.HandleFunc("/api/v1/archived-workflows", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(archive)) })
	mux.HandleFunc("/api/v1/workflow-templates/ml-training", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{"metadata":{"name":"train-template","uid":"t-1","creationTimestamp":"2026-01-20T00:00:00Z"}}]}`))
	})
	mux.HandleFunc("/api/v1/cron-workflows/ml-training", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{"metadata":{"name":"nightly","creationTimestamp":"2026-01-20T00:00:00Z"},"spec":{"schedule":"0 2 * * *"},"status":{"active":[]}}]}`))
	})
	return httptest.NewServer(mux)
}

func TestConformance(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	conformance.Run(t, New, conformance.Options{Config: adapter.Config{"server": srv.URL, "namespaces": []any{"ml-training"}, "poll_interval": "50ms"}, MinFacets: 4})
}

func TestKFPLabelBecomesKey(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	a := New()
	if err := a.Connect(context.Background(), adapter.Config{"server": srv.URL, "namespaces": []any{"ml-training"}}); err != nil {
		t.Fatal(err)
	}
	facets, err := a.Facets(context.Background(), adapter.Query{Kinds: []adapter.Kind{adapter.KindWorkflow}})
	if err != nil {
		t.Fatal(err)
	}
	if len(facets) != 2 {
		t.Fatalf("want 2 workflow facets, got %d", len(facets))
	}
	f := facets[0]
	if f.Keys[adapter.KeyKFPRunID] != "kfp-run-77" || f.Keys[adapter.KeyGitSHA] != "3e1f9c0" || f.Keys[adapter.KeyArgoWorkflow] != "train-abc12" {
		t.Fatalf("keys: %+v", f.Keys)
	}
	nodes := f.Fields["nodes"].([]map[string]string)
	if len(nodes) != 1 || nodes[0]["name"] != "train" {
		t.Fatalf("nodes: %+v", nodes)
	}
}

func TestLiveWorkflowWinsOverItsArchivedCopy(t *testing.T) {
	// The archive returns summaries without spec or nodes. A workflow that is
	// both live and archived must keep its live copy, and appear once.
	srv := fixtureWithArchive(t, `{"items":[
		{"metadata":{"name":"train-abc12","namespace":"ml-training","uid":"u-1","creationTimestamp":"2026-09-30T09:00:00Z"},"status":{"phase":"Succeeded","startedAt":"2026-09-30T09:00:00Z","finishedAt":"2026-09-30T09:20:00Z"}},
		{"metadata":{"name":"train-old01","namespace":"ml-training","uid":"u-0","creationTimestamp":"2026-08-01T09:00:00Z"},"status":{"phase":"Succeeded","startedAt":"2026-08-01T09:00:00Z","finishedAt":"2026-08-01T09:30:00Z"}}],"metadata":{}}`)
	defer srv.Close()
	a := New()
	if err := a.Connect(context.Background(), adapter.Config{"server": srv.URL, "namespaces": []any{"ml-training"}}); err != nil {
		t.Fatal(err)
	}
	facets, err := a.Facets(context.Background(), adapter.Query{Kinds: []adapter.Kind{adapter.KindWorkflow}})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string][]adapter.Facet{}
	for _, f := range facets {
		byID[f.SourceID] = append(byID[f.SourceID], f)
	}
	if len(facets) != 3 || len(byID["ml-training/train-old01"]) != 1 {
		t.Fatalf("want the two live workflows and the archived-only one, got %d facets", len(facets))
	}
	live := byID["ml-training/train-abc12"]
	if len(live) != 1 || len(live[0].Fields["nodes"].([]map[string]string)) != 1 || live[0].Keys[adapter.KeyKFPRunID] != "kfp-run-77" {
		t.Fatalf("the live copy should win over the archived summary: %+v", live)
	}
}

// A Kubernetes API that serves Workflows in ml-training: a list, and a watch
// that sends two events, then holds the reconnection open.
func kubeFixture(t *testing.T, resumedFrom chan<- string) *httptest.Server {
	t.Helper()
	wf := func(phase, rv, finished string) string {
		return `{"metadata":{"name":"train-abc12","namespace":"ml-training","uid":"u-1","resourceVersion":"` + rv + `","labels":{"pipeline/runid":"kfp-run-77"}},` +
			`"status":{"phase":"` + phase + `","startedAt":"2026-09-30T09:00:00Z","finishedAt":"` + finished + `","nodes":{}}}`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer kube-tok" {
			http.Error(w, "unauthorised", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/apis/argoproj.io/v1alpha1/namespaces/ml-training/workflows" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("watch") != "true" {
			fmt.Fprintf(w, `{"items":[%s],"metadata":{"resourceVersion":"100"}}`, wf("Running", "100", ""))
			return
		}
		if rv := r.URL.Query().Get("resourceVersion"); rv != "" {
			resumedFrom <- rv
			<-r.Context().Done()
			return
		}
		fmt.Fprintf(w, `{"type":"ADDED","object":%s}`+"\n", wf("Running", "100", ""))
		fmt.Fprintf(w, `{"type":"BOOKMARK","object":{"metadata":{"resourceVersion":"105"}}}`+"\n")
		fmt.Fprintf(w, `{"type":"MODIFIED","object":%s}`+"\n", wf("Succeeded", "110", "2026-09-30T09:20:00Z"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestKubeWatchWithoutArgoServer(t *testing.T) {
	resumed := make(chan string, 1)
	srv := kubeFixture(t, resumed)
	kubeconfigFor(t, srv.URL, "kube-tok")
	a := New()
	if err := a.Connect(context.Background(), adapter.Config{"namespaces": []any{"ml-training"}, "kube_watch": true}); err != nil {
		t.Fatal(err)
	}
	if st := a.Health(context.Background()); st.State != adapter.Healthy {
		t.Fatalf("health through the Kubernetes API: %+v", st)
	}
	facets, err := a.Facets(context.Background(), adapter.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(facets) != 1 || facets[0].Keys[adapter.KeyKFPRunID] != "kfp-run-77" || facets[0].Links != nil {
		t.Fatalf("want the one Workflow, with no Argo UI link and no templates: %+v", facets)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := a.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var phases []adapter.Phase
	for len(phases) < 2 {
		select {
		case ev := <-ch:
			if ev.Op != adapter.Upsert {
				t.Fatalf("unexpected event %+v", ev)
			}
			phases = append(phases, ev.Facet.Status)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out after %v", phases)
		}
	}
	if phases[0] != adapter.Running || phases[1] != adapter.Succeeded {
		t.Fatalf("phases: %v", phases)
	}
	select {
	case rv := <-resumed:
		if rv != "110" {
			t.Fatalf("the watch should resume from the last resource version, 110, not %s", rv)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the watch did not reconnect")
	}
	cancel()
	for range ch {
	}
}

func TestServerRequiredWithoutKubeWatch(t *testing.T) {
	if err := New().Connect(context.Background(), adapter.Config{"namespaces": []any{"ml-training"}}); err == nil {
		t.Fatal("without kube_watch, server is required")
	}
}

func TestTokenDiscovery(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	kubeconfigFor(t, "https://kubernetes.example:6443", "from-kubeconfig")
	a := New()
	if err := a.Connect(context.Background(), adapter.Config{"server": srv.URL, "namespaces": []any{"ml-training"}}); err != nil {
		t.Fatal(err)
	}
	a.Health(context.Background())
	if got != "Bearer from-kubeconfig" {
		t.Fatalf("without ARGO_TOKEN, the kubeconfig context's token should be used, got %q", got)
	}
	t.Setenv("ARGO_TOKEN", "Bearer explicit")
	if err := a.Connect(context.Background(), adapter.Config{"server": srv.URL, "namespaces": []any{"ml-training"}}); err != nil {
		t.Fatal(err)
	}
	a.Health(context.Background())
	if got != "Bearer explicit" {
		t.Fatalf("ARGO_TOKEN wins over the kubeconfig, got %q", got)
	}
}
