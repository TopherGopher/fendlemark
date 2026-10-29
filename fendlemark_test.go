package fendlemark_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fendlemark.app/fendlemark"
	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/lattice"
)

// One training job as four tools see it: a KFP run executes an Argo Workflow
// whose step launches a Metaflow run that logs to MLflow.
func platform(t *testing.T) (mlflow, metaflow, argo, kfp *httptest.Server) {
	t.Helper()
	ml := http.NewServeMux()
	ml.HandleFunc("/api/2.0/mlflow/experiments/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"experiments":[{"experiment_id":"1","name":"resnet"}]}`))
	})
	ml.HandleFunc("/api/2.0/mlflow/runs/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"runs":[{"info":{"run_id":"0123456789abcdef0123456789abcdef","run_name":"resnet-50 sweep 7","experiment_id":"1","status":"FINISHED","start_time":1759222840000,"end_time":1759223940000},
		  "data":{"tags":[{"key":"argo.workflow.name","value":"train-abc12"},{"key":"metaflow.run_id","value":"TrainFlow/1824"}]}}]}`))
	})
	ml.HandleFunc("/api/2.0/mlflow/model-versions/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"model_versions":[{"name":"resnet50","version":"4","status":"READY","run_id":"0123456789abcdef0123456789abcdef","creation_timestamp":1759224300000}]}`))
	})
	mf := http.NewServeMux()
	mf.HandleFunc("/flows", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"flow_id":"TrainFlow","ts_epoch":1700000000000}]`))
	})
	mf.HandleFunc("/flows/TrainFlow/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"flow_id":"TrainFlow","run_number":1824,"user_name":"marit","ts_epoch":1759222820000,"system_tags":["argo-workflows:train-abc12"]}]`))
	})
	ar := http.NewServeMux()
	ar.HandleFunc("/api/v1/info", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	ar.HandleFunc("/api/v1/workflows/ml-training", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{"metadata":{"name":"train-abc12","namespace":"ml-training","uid":"u-1","labels":{"pipeline/runid":"kfp-run-77"}},"status":{"phase":"Succeeded","startedAt":"2025-09-30T09:00:00Z","finishedAt":"2025-09-30T09:20:00Z","nodes":{}}}],"metadata":{}}`))
	})
	ar.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"items":[],"metadata":{}}`)) })
	kf := http.NewServeMux()
	kf.HandleFunc("/apis/v2beta1/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	kf.HandleFunc("/apis/v2beta1/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"runs":[{"run_id":"kfp-run-77","display_name":"train nightly","state":"SUCCEEDED","scheduled_at":"2025-09-30T08:59:30Z","finished_at":"2025-09-30T09:21:00Z"}]}`))
	})
	kf.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	return httptest.NewServer(ml), httptest.NewServer(mf), httptest.NewServer(ar), httptest.NewServer(kf)
}

func TestIntegrateEasy(t *testing.T) {
	ml, mf, ar, kf := platform(t)
	defer ml.Close()
	defer mf.Close()
	defer ar.Close()
	defer kf.Close()
	t.Setenv("MLFLOW_TRACKING_URI", ml.URL)
	t.Setenv("METAFLOW_SERVICE_URL", mf.URL)
	t.Setenv("ARGO_SERVER", ar.URL)
	t.Setenv("ARGO_NAMESPACE", "ml-training")
	t.Setenv("KUBEFLOW_PIPELINES_URL", kf.URL)
	noKubernetes(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	fm, err := fendlemark.IntegrateEasy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fm.Close()

	if got := fm.Adapters(); len(got) != 4 {
		t.Fatalf("expected all four adapters, got %v", got)
	}
	runs := fm.Lattice().Query(lattice.Query{Kinds: []adapter.Kind{adapter.KindRun}})
	if len(runs) != 1 {
		t.Fatalf("one training job should be one entity; got %d: %+v", len(runs), runs)
	}
	sources := map[string]bool{}
	for _, f := range runs[0].Facets {
		sources[f.Source] = true
	}
	for _, s := range []string{"mlflow", "metaflow", "argo", "kubeflow"} {
		if !sources[s] {
			t.Fatalf("run entity is missing the %s facet: %v", s, sources)
		}
	}
	if runs[0].Title != "resnet-50 sweep 7" || runs[0].Status != adapter.Succeeded {
		t.Fatalf("unexpected entity: %+v", runs[0])
	}
	ex, _ := fm.Lattice().Explain(runs[0].ID)
	if len(ex.Merges) != 3 {
		t.Fatalf("four facets need exactly three merges, got %d: %+v", len(ex.Merges), ex.Merges)
	}
	models := fm.Lattice().Query(lattice.Query{Kinds: []adapter.Kind{adapter.KindModel}})
	if len(models) != 1 || len(models[0].Related) == 0 || models[0].Related[0].To != runs[0].ID {
		t.Fatalf("model version should be DerivedFrom the run: %+v", models)
	}

	// The HTTP API serves the same lattice.
	srv := httptest.NewServer(fm.Server().Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/entities?kind=run")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Entities []lattice.Entity `json:"entities"`
		Total    int              `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || len(body.Entities[0].Facets) != 4 {
		t.Fatalf("API disagrees with lattice: %+v", body)
	}
	st, err := http.Get(srv.URL + "/api/v1/adapters")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Body.Close()
	var ad struct {
		Adapters []struct {
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"adapters"`
	}
	json.NewDecoder(st.Body).Decode(&ad)
	for _, a := range ad.Adapters {
		if a.State != "healthy" {
			t.Fatalf("adapter %s is %s", a.Name, a.State)
		}
	}
}

// noKubernetes keeps the kubeflow adapter away from any real cluster the
// machine running the tests can reach.
func noKubernetes(t *testing.T) {
	t.Helper()
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "absent"))
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
}

// A listing that fails keeps the adapter degraded even though its health
// check, which needs no credentials, still passes.
func TestRejectedTokenStaysDegraded(t *testing.T) {
	kf := http.NewServeMux()
	kf.HandleFunc("/apis/v2beta1/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	kf.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"Unauthenticated"}`, http.StatusUnauthorized)
	})
	srv := httptest.NewServer(kf)
	defer srv.Close()
	for _, k := range []string{"MLFLOW_TRACKING_URI", "METAFLOW_SERVICE_URL", "ARGO_SERVER"} {
		t.Setenv(k, "")
	}
	t.Setenv("KUBEFLOW_PIPELINES_URL", srv.URL)
	noKubernetes(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	fm, err := fendlemark.IntegrateEasy(ctx)
	if fm == nil {
		t.Fatal(err)
	}
	defer fm.Close()
	for _, st := range fm.AdapterStatuses(ctx) {
		if st.Name != "kubeflow" {
			continue
		}
		if st.State != adapter.Degraded || !strings.Contains(st.Detail, "HTTP 401") {
			t.Fatalf("kubeflow should be degraded with the 401, got %s: %s", st.State, st.Detail)
		}
		return
	}
	t.Fatal("no kubeflow adapter")
}
