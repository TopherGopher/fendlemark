package argo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/adapter/conformance"
)

func fixture(t *testing.T) *httptest.Server {
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
	mux.HandleFunc("/api/v1/archived-workflows", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"items":[],"metadata":{}}`)) })
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
