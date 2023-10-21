package metaflow

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
	mux.HandleFunc("/flows", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"flow_id":"TrainFlow","user_name":"marit","ts_epoch":1700000000000}]`))
	})
	mux.HandleFunc("/flows/TrainFlow/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"flow_id":"TrainFlow","run_number":1824,"user_name":"marit","ts_epoch":1759222800000,"tags":["git.sha:3e1f9c0","git.repo:git@example:ml/train"],"system_tags":["argo-workflows:train-abc12","runtime:kubernetes"]},
		                 {"flow_id":"TrainFlow","run_number":0,"run_id":"argo-trainflow-xyz99","user_name":"dov","ts_epoch":1759230000000,"tags":[],"system_tags":["argo-workflows:train-xyz99"]}]`))
	})
	return httptest.NewServer(mux)
}

func TestConformance(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	conformance.Run(t, New, conformance.Options{Config: adapter.Config{"metadata_url": srv.URL}, MinFacets: 3})
}

func TestSystemTagsBecomeKeys(t *testing.T) {
	srv := fixture(t)
	defer srv.Close()
	a := New()
	if err := a.Connect(context.Background(), adapter.Config{"metadata_url": srv.URL, "ui_url": "https://mf.example"}); err != nil {
		t.Fatal(err)
	}
	facets, err := a.Facets(context.Background(), adapter.Query{Kinds: []adapter.Kind{adapter.KindRun}})
	if err != nil {
		t.Fatal(err)
	}
	if len(facets) != 2 {
		t.Fatalf("want 2 runs, got %d", len(facets))
	}
	if facets[0].Keys[adapter.KeyArgoWorkflow] != "train-abc12" || facets[0].Keys[adapter.KeyGitSHA] != "3e1f9c0" || facets[0].Keys[adapter.KeyMetaflowRunID] != "TrainFlow/1824" {
		t.Fatalf("keys: %+v", facets[0].Keys)
	}
	if facets[1].Keys[adapter.KeyMetaflowRunID] != "TrainFlow/argo-trainflow-xyz99" {
		t.Fatalf("argo-launched run id should be used verbatim: %+v", facets[1].Keys)
	}
	if facets[0].Links[0].URL != "https://mf.example/TrainFlow/1824" {
		t.Fatalf("deep link: %+v", facets[0].Links)
	}
}
