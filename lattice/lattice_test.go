package lattice

import (
	"testing"
	"time"

	"fendlemark.app/fendlemark/adapter"
)

func fixture() []adapter.Facet {
	t0 := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	return []adapter.Facet{
		{Source: "argo", SourceID: "train-abc12", Kind: adapter.KindWorkflow, Title: "train-abc12", Status: adapter.Succeeded,
			StartedAt: t0, EndedAt: t0.Add(20 * time.Minute),
			Keys: map[string]string{adapter.KeyArgoWorkflow: "train-abc12", adapter.KeyGitSHA: "3e1f9c0", adapter.KeyGitRepo: "git@example:ml/train"}},
		{Source: "mlflow", SourceID: "0123456789abcdef0123456789abcdef", Kind: adapter.KindRun, Title: "resnet-50 sweep 7", Status: adapter.Succeeded,
			StartedAt: t0.Add(40 * time.Second), EndedAt: t0.Add(19 * time.Minute),
			Keys: map[string]string{adapter.KeyMLflowRunID: "0123456789abcdef0123456789abcdef", adapter.KeyArgoWorkflow: "train-abc12", adapter.KeyGitSHA: "3e1f9c0", adapter.KeyGitRepo: "git@example:ml/train"}},
		{Source: "mlflow", SourceID: "model:resnet50:v4", Kind: adapter.KindModel, Title: "resnet50 v4", Status: adapter.Succeeded,
			StartedAt: t0.Add(25 * time.Minute), Fields: map[string]any{"source_run_id": "0123456789abcdef0123456789abcdef"},
			Keys: map[string]string{"mlflow.model_version": "resnet50:4"}},
		{Source: "argo", SourceID: "train-xyz99", Kind: adapter.KindWorkflow, Title: "train-xyz99", Status: adapter.Running,
			StartedAt: t0.Add(2 * time.Hour), Keys: map[string]string{adapter.KeyArgoWorkflow: "train-xyz99", adapter.KeyGitSHA: "3e1f9c0", adapter.KeyGitRepo: "git@example:ml/train"}},
	}
}

func TestMergeWithinWindow(t *testing.T) {
	l := New(Options{})
	d := l.Apply(fixture())
	if got := len(l.Query(Query{})); got != 3 {
		t.Fatalf("want 3 entities (merged run+workflow, model, second workflow), got %d", got)
	}
	runs := l.Query(Query{Kinds: []adapter.Kind{adapter.KindRun}})
	if len(runs) != 1 || len(runs[0].Facets) != 2 {
		t.Fatalf("want one run entity with two facets, got %+v", runs)
	}
	if runs[0].Title != "resnet-50 sweep 7" {
		t.Fatalf("mlflow title should win, got %q", runs[0].Title)
	}
	ex, _ := l.Explain(runs[0].ID)
	if len(ex.Merges) != 1 || ex.Merges[0].Key != adapter.KeyArgoWorkflow {
		t.Fatalf("expected one merge on argo.workflow.name, got %+v", ex.Merges)
	}
	if d.Generation != 1 || len(d.Upserted) != 3 {
		t.Fatalf("unexpected delta %+v", d)
	}
}

func TestDeterministicIDs(t *testing.T) {
	a, b := New(Options{}), New(Options{})
	fx := fixture()
	a.Apply(fx)
	// reverse order for b
	rev := make([]adapter.Facet, len(fx))
	for i := range fx {
		rev[len(fx)-1-i] = fx[i]
	}
	b.Apply(rev)
	qa, qb := a.Query(Query{}), b.Query(Query{})
	for i := range qa {
		if qa[i].ID != qb[i].ID || qa[i].Status != qb[i].Status || len(qa[i].Related) != len(qb[i].Related) {
			t.Fatalf("lattices differ at %d: %+v vs %+v", i, qa[i], qb[i])
		}
	}
}

func TestRelations(t *testing.T) {
	l := New(Options{})
	l.Apply(fixture())
	models := l.Query(Query{Kinds: []adapter.Kind{adapter.KindModel}})
	if len(models) != 1 {
		t.Fatal("want one model")
	}
	var derived bool
	for _, r := range models[0].Related {
		if r.Rel == DerivedFrom {
			derived = true
		}
	}
	if !derived {
		t.Fatalf("model should be DerivedFrom its source run: %+v", models[0].Related)
	}
	wf := l.Query(Query{Keys: map[string]string{adapter.KeyArgoWorkflow: "train-xyz99"}})
	if len(wf) != 1 || len(wf[0].Related) == 0 || wf[0].Related[0].Rel != SharesCommit {
		t.Fatalf("second workflow should share a commit with the first: %+v", wf)
	}
}

func TestExplainKeepsEveryMerge(t *testing.T) {
	// kubeflow+mlflow merge first (on a.shared), then argo joins (on b.shared)
	// and becomes the component's root. Explain must still report both steps.
	t0 := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	l := New(Options{})
	l.Apply([]adapter.Facet{
		{Source: "argo", SourceID: "a", Kind: adapter.KindWorkflow, StartedAt: t0, Keys: map[string]string{"b.shared": "2"}},
		{Source: "kubeflow", SourceID: "k", Kind: adapter.KindPipeline, StartedAt: t0.Add(10 * time.Second), Keys: map[string]string{"a.shared": "1"}},
		{Source: "mlflow", SourceID: "m", Kind: adapter.KindRun, StartedAt: t0.Add(20 * time.Second), Keys: map[string]string{"a.shared": "1", "b.shared": "2"}},
	})
	ents := l.Query(Query{})
	if len(ents) != 1 || len(ents[0].Facets) != 3 {
		t.Fatalf("want one entity with three facets, got %+v", ents)
	}
	ex, _ := l.Explain(ents[0].ID)
	if len(ex.Merges) != 2 || ex.Merges[0].Key != "a.shared" || ex.Merges[1].Key != "b.shared" {
		t.Fatalf("want merges on a.shared then b.shared, got %+v", ex.Merges)
	}
}

func TestWindowRespected(t *testing.T) {
	l := New(Options{CorrelationWindow: 10 * time.Second})
	l.Apply(fixture())
	if got := len(l.Query(Query{})); got != 4 {
		t.Fatalf("with a 10s window nothing should merge; got %d entities", got)
	}
}

func TestIncrementalDelta(t *testing.T) {
	l := New(Options{})
	l.Apply(fixture())
	fx := fixture()
	wf := fx[3]
	wf.Status = adapter.Succeeded
	d := l.ApplyEvent(adapter.Event{Op: adapter.Upsert, Facet: wf})
	if len(d.Upserted) != 1 || d.Generation != 2 {
		t.Fatalf("expected one upsert at generation 2, got %+v", d)
	}
	d = l.ApplyEvent(adapter.Event{Op: adapter.Delete, Facet: wf})
	if len(d.Removed) != 1 {
		t.Fatalf("expected one removal, got %+v", d)
	}
}

func TestEmptyKeyValuesAreIgnored(t *testing.T) {
	t0 := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	jobs := []adapter.Facet{
		{Source: "example", SourceID: "job-1", Kind: adapter.KindRun, StartedAt: t0,
			Keys: map[string]string{"example.job_id": "job-1", adapter.KeyMLflowRunID: ""}},
		{Source: "example", SourceID: "job-2", Kind: adapter.KindRun, StartedAt: t0.Add(time.Second),
			Keys: map[string]string{"example.job_id": "job-2", adapter.KeyMLflowRunID: ""}},
	}
	l := New(Options{})
	l.Apply(jobs)
	ents := l.Query(Query{})
	if len(ents) != 2 {
		t.Fatalf("two jobs without an MLflow run must stay apart and both be kept, got %+v", ents)
	}
	for _, e := range ents {
		if _, ok := e.Facets[0].Keys[adapter.KeyMLflowRunID]; ok {
			t.Fatalf("empty value should be dropped: %+v", e.Facets[0].Keys)
		}
	}
	if jobs[0].Keys[adapter.KeyMLflowRunID] != "" || len(jobs[0].Keys) != 2 {
		t.Fatal("the adapter's own map must not be modified")
	}
}
