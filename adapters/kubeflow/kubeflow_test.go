package kubeflow

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	return httptest.NewServer(mux)
}

func TestConformance(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	conformance.Run(t, New, conformance.Options{Config: adapter.Config{"pipelines_url": srv.URL, "poll_interval": "50ms"}, MinFacets: 3})
}

func TestParametersBecomeKeys(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
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
