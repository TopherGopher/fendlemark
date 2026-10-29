package kubeflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/adapter/conformance"
)

func fixture(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/v2beta1/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	mux.HandleFunc("/apis/v2beta1/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"runs":[{"run_id":"kfp-run-77","display_name":"train nightly","namespace":"kubeflow","state":"SUCCEEDED","created_at":"2026-09-30T08:59:00Z","scheduled_at":"2026-09-30T09:00:00Z","finished_at":"2026-09-30T09:21:00Z",
		  "pipeline_version_reference":{"pipeline_id":"p-1","pipeline_version_id":"pv-3"},"runtime_config":{"parameters":{"epochs":10,"mlflow.run_id":"0123456789abcdef0123456789abcdef"}},
		  "run_details":{"task_details":[{"display_name":"train","state":"SUCCEEDED","start_time":"2026-09-30T09:00:05Z","end_time":"2026-09-30T09:20:00Z"}]}}]}`))
	})
	mux.HandleFunc("/apis/v2beta1/pipelines", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"pipelines":[{"pipeline_id":"p-1","display_name":"train","namespace":"kubeflow","created_at":"2026-01-20T00:00:00Z"}]}`))
	})
	mux.HandleFunc("/apis/v2beta1/recurringruns", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"recurringRuns":[{"recurring_run_id":"rr-1","display_name":"nightly","namespace":"kubeflow","status":"ENABLED","created_at":"2026-01-20T00:00:00Z","trigger":{"cron_schedule":{"cron":"0 2 * * *"}}}]}`))
	})
	// The Kubernetes API, for Katib and Notebooks.
	mux.HandleFunc("/apis/kubeflow.org/v1beta1/namespaces/team-ml/experiments", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{"metadata":{"name":"lr-sweep","namespace":"team-ml","creationTimestamp":"2026-10-01T10:00:00Z"},
		  "spec":{"objective":{"type":"maximize","goal":0.95,"objectiveMetricName":"accuracy"},"algorithm":{"algorithmName":"tpe"},"maxTrialCount":12},
		  "status":{"startTime":"2026-10-01T10:00:02Z","completionTime":"2026-10-01T11:40:00Z","trials":12,"trialsSucceeded":11,"trialsFailed":1,
		    "conditions":[{"type":"Created","status":"True"},{"type":"Running","status":"False"},{"type":"Succeeded","status":"True"}],
		    "currentOptimalTrial":{"bestTrialName":"lr-sweep-7xk2p","parameterAssignments":[{"name":"lr","value":"0.0031"}],"observation":{"metrics":[{"name":"accuracy","latest":"0.951"}]}}}}]}`))
	})
	mux.HandleFunc("/apis/kubeflow.org/v1beta1/namespaces/team-ml/trials", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{"metadata":{"name":"lr-sweep-7xk2p","namespace":"team-ml","labels":{"katib.kubeflow.org/experiment":"lr-sweep"}},"status":{"conditions":[{"type":"Succeeded","status":"True"}]}}]}`))
	})
	mux.HandleFunc("/apis/kubeflow.org/v1/namespaces/team-ml/notebooks", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{"metadata":{"name":"marit-scratch","namespace":"team-ml","creationTimestamp":"2026-09-02T08:00:00Z"},
		  "spec":{"template":{"spec":{"containers":[{"image":"kubeflownotebookswg/jupyter-scipy:v1.10.0","resources":{"requests":{"cpu":"500m","memory":"1Gi"}}}]}}},
		  "status":{"readyReplicas":1}}]}`))
	})
	return httptest.NewServer(mux)
}

// noKubernetes keeps the adapter away from any real cluster the machine
// running the tests can reach.
func noKubernetes(t *testing.T) {
	t.Helper()
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "absent"))
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
}

// kubeconfigFor points KUBECONFIG at the fixture, as a cluster with no auth.
func kubeconfigFor(t *testing.T, server string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	kc := "current-context: lab\ncontexts:\n- name: lab\n  context: {cluster: lab, user: me}\nclusters:\n- name: lab\n  cluster: {server: \"" + server + "\"}\nusers:\n- name: me\n  user: {}\n"
	if err := os.WriteFile(path, []byte(kc), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
}

func TestConformance(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	noKubernetes(t)
	conformance.Run(t, New, conformance.Options{Config: adapter.Config{"pipelines_url": srv.URL, "poll_interval": "50ms"}, MinFacets: 3})
}

func TestParametersBecomeKeys(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	noKubernetes(t)
	a := New()
	if err := a.Connect(context.Background(), adapter.Config{"pipelines_url": srv.URL, "dashboard_url": "https://kf.example"}); err != nil {
		t.Fatal(err)
	}
	facets, err := a.Facets(context.Background(), adapter.Query{Kinds: []adapter.Kind{adapter.KindWorkflow}})
	if err != nil {
		t.Fatal(err)
	}
	if len(facets) != 1 {
		t.Fatalf("want 1 run, got %d", len(facets))
	}
	k := facets[0].Keys
	if k[adapter.KeyKFPRunID] != "kfp-run-77" || k[adapter.KeyMLflowRunID] != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("keys: %+v", k)
	}
	if _, ok := k[adapter.KeyArgoWorkflow]; ok {
		t.Fatalf("a KFP run does not know its Argo Workflow's name; it meets the workflow on kfp.run_id: %+v", k)
	}
	if facets[0].Links[0].URL != "https://kf.example/pipeline/#/runs/details/kfp-run-77" {
		t.Fatalf("deep link: %+v", facets[0].Links)
	}
}

func TestKatibAndNotebooks(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	kubeconfigFor(t, srv.URL)
	a := New()
	cfg := adapter.Config{"pipelines_url": srv.URL, "profiles": []any{"team-ml"}, "dashboard_url": "https://kf.example"}
	if err := a.Connect(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	facets, err := a.Facets(context.Background(), adapter.Query{})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]adapter.Facet{}
	for _, f := range facets {
		byID[f.SourceID] = f
	}
	e, ok := byID["katib:team-ml/lr-sweep"]
	if !ok || e.Kind != adapter.KindWorkflow || e.Status != adapter.Succeeded || e.EndedAt.IsZero() {
		t.Fatalf("katib experiment facet: %+v", e)
	}
	if e.Fields["objective"] != "maximize accuracy (goal 0.95)" || e.Fields["algorithm"] != "tpe" {
		t.Fatalf("objective and algorithm: %+v", e.Fields)
	}
	best := e.Fields["best_trial"].(map[string]any)
	if best["name"] != "lr-sweep-7xk2p" || best["parameters"].(map[string]string)["lr"] != "0.0031" || best["metrics"].(map[string]string)["accuracy"] != "0.951" {
		t.Fatalf("best trial: %+v", best)
	}
	if ts := e.Fields["trial_phases"].([]map[string]string); len(ts) != 1 || ts[0]["phase"] != "Succeeded" {
		t.Fatalf("trials: %+v", ts)
	}
	n, ok := byID["notebook:team-ml/marit-scratch"]
	if !ok || n.Kind != adapter.KindNotebook || n.Status != adapter.Running || n.Fields["status"] != "running" {
		t.Fatalf("notebook facet: %+v", n)
	}
	if n.Fields["image"] != "kubeflownotebookswg/jupyter-scipy:v1.10.0" || n.Links[0].URL != "https://kf.example/notebook/team-ml/marit-scratch/" {
		t.Fatalf("notebook image and link: %+v %+v", n.Fields, n.Links)
	}

	// Turned off, neither is read.
	b := New()
	cfg["katib"], cfg["notebooks"] = false, false
	if err := b.Connect(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	facets, _ = b.Facets(context.Background(), adapter.Query{})
	for _, f := range facets {
		if f.Kind == adapter.KindNotebook || f.Fields["type"] == "KatibExperiment" {
			t.Fatalf("katib and notebooks are off, got %s", f.SourceID)
		}
	}
}

func TestNoClusterIsNotAnError(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	noKubernetes(t)
	a := New()
	if err := a.Connect(context.Background(), adapter.Config{"pipelines_url": srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Facets(context.Background(), adapter.Query{Kinds: []adapter.Kind{adapter.KindNotebook}}); err != nil {
		t.Fatalf("no Kubernetes credentials should mean no Katib or Notebook facets, not an error: %v", err)
	}
}
