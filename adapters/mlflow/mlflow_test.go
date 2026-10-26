package mlflow

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
	mux.HandleFunc("/api/2.0/mlflow/experiments/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"experiments": []map[string]any{{"experiment_id": "1", "name": "resnet", "lifecycle_stage": "active"}}})
	})
	mux.HandleFunc("/api/2.0/mlflow/runs/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"runs": []map[string]any{
			{"info": map[string]any{"run_id": "0123456789abcdef0123456789abcdef", "run_name": "sweep-7", "experiment_id": "1", "status": "FINISHED", "start_time": 1759222800000, "end_time": 1759223940000, "lifecycle_stage": "active"},
				"data": map[string]any{"metrics": []map[string]any{{"key": "acc", "value": 0.91}}, "params": []map[string]any{{"key": "lr", "value": "0.001"}},
					"tags": []map[string]any{{"key": "argo.workflow.name", "value": "train-abc12"}, {"key": "mlflow.source.git.commit", "value": "3e1f9c0"}, {"key": "mlflow.source.git.repoURL", "value": "git@example:ml/train"}}}},
			{"info": map[string]any{"run_id": "fedcba9876543210fedcba9876543210", "run_name": "sweep-8", "experiment_id": "1", "status": "RUNNING", "start_time": 1759226400000},
				"data": map[string]any{}},
		}})
	})
	mux.HandleFunc("/api/2.0/mlflow/model-versions/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"model_versions": []map[string]any{{"name": "resnet50", "version": "4", "current_stage": "Production", "status": "READY", "run_id": "0123456789abcdef0123456789abcdef", "creation_timestamp": 1759224000000}}})
	})
	return httptest.NewServer(mux)
}

func TestConformance(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	conformance.Run(t, New, conformance.Options{Config: adapter.Config{"tracking_uri": srv.URL}, MinFacets: 3})
}

func TestCorrelationKeys(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	a := New()
	if err := a.Connect(context.Background(), adapter.Config{"tracking_uri": srv.URL}); err != nil {
		t.Fatal(err)
	}
	facets, err := a.Facets(context.Background(), adapter.Query{})
	if err != nil {
		t.Fatal(err)
	}
	var run, model *adapter.Facet
	for i := range facets {
		switch facets[i].SourceID {
		case "0123456789abcdef0123456789abcdef":
			run = &facets[i]
		case "model:resnet50:v4":
			model = &facets[i]
		}
	}
	if run == nil || model == nil {
		t.Fatalf("missing facets: %+v", facets)
	}
	if run.Keys[adapter.KeyArgoWorkflow] != "train-abc12" || run.Keys[adapter.KeyGitSHA] != "3e1f9c0" {
		t.Fatalf("correlation tags not mapped: %+v", run.Keys)
	}
	if run.Status != adapter.Succeeded || run.EndedAt.IsZero() {
		t.Fatalf("FINISHED should map to Succeeded with an end time: %+v", run)
	}
	if model.Fields["source_run_id"] != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("model should carry its source run: %+v", model.Fields)
	}
}

func TestNoMatchingExperiment(t *testing.T) {
	// MLflow rejects runs/search with no experiment ids. A filter that matches
	// nothing means there are no runs to read, and the registry is still read.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/2.0/mlflow/experiments/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"experiments": []map[string]any{{"experiment_id": "1", "name": "resnet", "lifecycle_stage": "active"}}})
	})
	mux.HandleFunc("/api/2.0/mlflow/runs/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error_code":"INVALID_PARAMETER_VALUE","message":"experiment_ids must be a non-empty list"}`))
	})
	mux.HandleFunc("/api/2.0/mlflow/model-versions/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"model_versions": []map[string]any{{"name": "resnet50", "version": "4", "status": "READY", "creation_timestamp": 1759224000000}}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	a := New()
	if err := a.Connect(context.Background(), adapter.Config{"tracking_uri": srv.URL, "experiments": []any{"churn"}}); err != nil {
		t.Fatal(err)
	}
	facets, err := a.Facets(context.Background(), adapter.Query{})
	if err != nil {
		t.Fatalf("a filter that matches no experiment should not fail the read: %v", err)
	}
	if len(facets) != 1 || facets[0].Kind != adapter.KindModel {
		t.Fatalf("want only the model version, got %+v", facets)
	}
}
