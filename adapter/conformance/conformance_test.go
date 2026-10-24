package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"fendlemark.app/fendlemark/adapter"
)

// example is the minimal adapter from Writing an adapter, with a client.
type example struct{ client *trackerClient }

func newExample() adapter.Adapter { return &example{} }

func (e *example) Name() string { return "example" }

func (e *example) Describe() adapter.Descriptor {
	return adapter.Descriptor{
		DisplayName: "Example Tracker",
		Kinds:       []adapter.Kind{adapter.KindRun},
		Keys:        []string{"mlflow.run_id", "example.job_id"},
		Maturity:    adapter.Provisional,
	}
}

func (e *example) Connect(ctx context.Context, cfg adapter.Config) error {
	e.client = &trackerClient{url: cfg.String("url"), token: cfg.Secret("token")}
	return e.client.Ping(ctx)
}

func (e *example) Facets(ctx context.Context, q adapter.Query) ([]adapter.Facet, error) {
	jobs, err := e.client.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]adapter.Facet, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, adapter.Facet{
			Source: "example", SourceID: j.ID, Kind: adapter.KindRun,
			Title: j.Name, Status: phaseOf(j.State), StartedAt: j.Started, EndedAt: j.Ended,
			// Not every job has an MLflow run, so the value is sometimes empty.
			Keys:   map[string]string{"example.job_id": j.ID, "mlflow.run_id": j.Labels["mlflow_run"]},
			Fields: map[string]any{"queue": j.Queue, "gpus": j.GPUs},
			Links:  []adapter.Link{{Label: "Open in Example", URL: e.client.url + "/jobs/" + j.ID}},
		})
	}
	return out, nil
}

func (e *example) Watch(ctx context.Context) (<-chan adapter.Event, error) {
	return nil, adapter.ErrWatchUnsupported
}

func (e *example) Health(ctx context.Context) adapter.Status {
	if err := e.client.Ping(ctx); err != nil {
		return adapter.Status{State: adapter.Unreachable, Detail: err.Error()}
	}
	return adapter.Status{State: adapter.Healthy, Detail: "ok"}
}

type job struct {
	ID, Name, State, Queue string
	GPUs                   int
	Started, Ended         time.Time
	Labels                 map[string]string
}

func phaseOf(state string) adapter.Phase {
	switch state {
	case "done":
		return adapter.Succeeded
	case "failed":
		return adapter.Failed
	case "running":
		return adapter.Running
	}
	return adapter.Unknown
}

type trackerClient struct{ url, token string }

func (c *trackerClient) get(ctx context.Context, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/api/jobs", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET /api/jobs: %s", resp.Status)
	}
	if v == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *trackerClient) Ping(ctx context.Context) error { return c.get(ctx, nil) }

func (c *trackerClient) ListJobs(ctx context.Context) ([]job, error) {
	var body struct{ Jobs []job }
	err := c.get(ctx, &body)
	return body.Jobs, err
}

// TestConformance is the example from Writing an adapter.
func TestConformance(t *testing.T) {
	Run(t, newExample, Options{
		Fixture:   "testdata/jobs.json",
		Config:    adapter.Config{"url": "http://fixture", "token": "x"},
		MinFacets: 3,
	})
}

func TestPointAt(t *testing.T) {
	got := pointAt(adapter.Config{"url": "http://fixture", "api": "http://fixture/v1?x=1", "other": "http://fixtures.example", "n": 3}, "http://127.0.0.1:9")
	want := adapter.Config{"url": "http://127.0.0.1:9", "api": "http://127.0.0.1:9/v1?x=1", "other": "http://fixtures.example", "n": 3}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %v, want %v", k, got[k], v)
		}
	}
}
