# Writing a Fendlemark adapter, step by step

Source: https://fendlemark.app/blog/writing-a-fendlemark-adapter/

By [the Fendlemark Keepers](https://fendlemark.app/governance/#keepers) · 24 October 2026

The reference page for [writing an adapter](https://fendlemark.app/docs/adapters/writing-an-adapter/) describes the interface. This post uses it: we build a complete adapter for Kubernetes Jobs, test it against a recorded fixture with the conformance suite, and run it inside a Fendlemark binary. Every line of code below compiles against `fendlemark.app/fendlemark` v0.9.5.

## Why Kubernetes Jobs

Not every training job goes through a workflow engine. On many platforms some of them are a `batch/v1` Job, created by a script, a CI pipeline, or a person with `kubectl`, and those jobs are invisible to the four built-in adapters. They are a good first adapter: the API is stable and well documented, one Job maps cleanly onto one facet, and the interesting decisions, how to map status onto phases and where to find correlation keys, are the same decisions every adapter makes.

The adapter will:

- list the Jobs in the namespaces you configure, page by page, and contribute each one as a facet of kind `run`;
- map Job status onto Fendlemark's five phases, with correct end times for failed Jobs;
- read correlation keys from annotations on the Job, so that a Job and the MLflow run it logged become one entity;
- report health by checking the one permission it needs, in every namespace it reads.

It will not watch. Polling is a perfectly good first version, and the control plane polls any adapter that says so.

## Set up the module

An adapter is an ordinary Go package. It does not need to live in the Fendlemark repository, and this one does not:

```shell
$ mkdir k8sjobs && cd k8sjobs
$ go mod init example.com/k8sjobs
$ go get fendlemark.app/fendlemark@v0.9.5
```

You need Go 1.23 or later. By the end, the module looks like this:

```text
k8sjobs/
├── go.mod
├── k8sjobs.go             the adapter
├── k8sjobs_test.go        conformance and behaviour tests
├── testdata/jobs.json     a recorded JobList
└── cmd/platform/main.go   Fendlemark with the adapter compiled in
```

## 1. Register the adapter and choose its keys

The Go code in sections 1 to 8 is `k8sjobs.go`, in order. It starts with registration and with the one decision that matters most, which correlation keys the adapter provides:

```go
// Package k8sjobs is a Fendlemark adapter for Kubernetes batch/v1 Jobs.
// It lists Jobs in the namespaces you configure and contributes each one
// to the lattice as a run. Like every adapter, it never writes.
package k8sjobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"fendlemark.app/fendlemark/adapter"
)

func init() { adapter.Register("k8sjobs", New) }

// Annotations maps the Job annotations this adapter reads to the shared
// correlation key vocabulary.
var Annotations = map[string]string{
	"fendlemark.io/mlflow-run-id": adapter.KeyMLflowRunID,
	"fendlemark.io/argo-workflow": adapter.KeyArgoWorkflow,
	"fendlemark.io/git-sha":       adapter.KeyGitSHA,
	"fendlemark.io/git-repo":      adapter.KeyGitRepo,
}
```

`adapter.Register` takes a name and a constructor. The name is what users write under `adapters:` in `fendlemark.yaml`, and it must be the same string the adapter returns from `Name()` and puts in every facet's `Source`; the conformance suite checks the last of those.

The keys come from the shared vocabulary in the `adapter` package rather than being invented: a Job that names an MLflow run should say so with exactly the key the mlflow adapter uses, `mlflow.run_id`, or the two will never meet. The reference page's rule is to invent a key only when none applies, and to prefix it with your adapter's name. None needed inventing here.

They are read from annotations rather than labels because label values are limited to 63 characters from a small alphabet, and a repository URL such as `git@git.example.internal:ml/vision.git` is neither. The `fendlemark.io/` prefix matches the `fendlemark.io/git-sha` and `fendlemark.io/git-repo` annotations the argo adapter already reads from Workflows. Whatever creates the Job sets them:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  generateName: train-resnet50-
  namespace: ml-training
  annotations:
    fendlemark.io/mlflow-run-id: "6f1d2c9a8b7e4f30a1c5d9e8b2a47c13"
    fendlemark.io/git-sha: "a41c9e07b2d3"
    fendlemark.io/git-repo: "git@git.example.internal:ml/vision.git"
spec:
  backoffLimit: 6
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: train
          image: registry.example.internal/vision/train:2026.10
          envFrom:
            - configMapRef:
                name: mlflow-client
          env:
            - name: MLFLOW_RUN_ID
              value: "6f1d2c9a8b7e4f30a1c5d9e8b2a47c13"
```

The MLflow run id has to exist before the Job does, so whatever submits the Job creates the run first, with `MlflowClient().create_run(...)`, and passes its id twice: to the annotation, and to `MLFLOW_RUN_ID`, which makes the training code's `mlflow.start_run()` resume that run rather than start another. It is the same technique the evaluation step uses in [Running MLflow and Argo Workflows together](https://fendlemark.app/blog/running-mlflow-and-argo-workflows-together/#resume). Submit the manifest with `kubectl create -f`; `kubectl apply` does not accept `generateName`.

## 2. The type, the constructor, and the descriptor

```go
// Jobs is the adapter.
type Jobs struct {
	mu         sync.RWMutex
	up         *upstream
	namespaces []string
}

// upstream is a minimal client for the Kubernetes API server.
type upstream struct {
	client *http.Client
	api    string // base URL, for example http://127.0.0.1:8001 behind kubectl proxy
	token  string // bearer token; empty behind kubectl proxy
}

// New returns an unconnected adapter. It is the constructor passed to Register.
func New() adapter.Adapter { return &Jobs{} }

func (j *Jobs) Name() string { return "k8sjobs" }

func (j *Jobs) Describe() adapter.Descriptor {
	return adapter.Descriptor{
		DisplayName: "Kubernetes Jobs",
		Kinds:       []adapter.Kind{adapter.KindRun},
		Keys:        []string{adapter.KeyMLflowRunID, adapter.KeyArgoWorkflow, adapter.KeyGitSHA, adapter.KeyGitRepo},
		Watchable:   false,
		Maturity:    adapter.Provisional,
		ConfigSchema: json.RawMessage(`{"type":"object","required":["api_url","namespaces"],"properties":{` +
			`"api_url":{"type":"string"},"namespaces":{"type":"array","items":{"type":"string"}},"token":{"type":"string"}}}`),
	}
}
```

The constructor returns an unconnected adapter and does no work; the control plane may construct an adapter it never connects. The descriptor is static, and the conformance suite calls `Describe` twice and fails if the results differ, so it must not read the clock, the environment, or the network.

- **`Kinds`** lists what the adapter contributes: runs only.
- **`Keys`** must name every key a facet can carry. A facet that emits an undeclared key fails conformance, which is a useful check on typos in the annotation map.
- **`Watchable: false`** tells the control plane to call `Facets` every `poll_interval` (30 seconds unless configured) instead of calling `Watch`.
- **`Maturity: adapter.Provisional`** is where every new adapter starts on the [maturity ladder](https://fendlemark.app/governance/#maturity).
- **`ConfigSchema`** is a JSON Schema for the adapter's block in `fendlemark.yaml`: `api_url`, `namespaces`, and an optional `token`.

## 3. Connect

```go
func (j *Jobs) Connect(ctx context.Context, cfg adapter.Config) error {
	api := strings.TrimRight(cfg.String("api_url"), "/")
	if api == "" {
		return errors.New("k8sjobs: api_url is required")
	}
	if u, err := url.Parse(api); err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("k8sjobs: api_url %q is not an absolute URL", api)
	}
	namespaces := cfg.Strings("namespaces")
	if len(namespaces) == 0 {
		return errors.New("k8sjobs: namespaces must list at least one namespace")
	}
	up := &upstream{
		client: &http.Client{Timeout: cfg.Duration("timeout", 10*time.Second)},
		api:    api,
		token:  cfg.Secret("token"),
	}
	// One cheap request, so a wrong URL fails here rather than on the first poll.
	var version struct {
		GitVersion string `json:"gitVersion"`
	}
	if err := up.get(ctx, "/version", &version); err != nil {
		return fmt.Errorf("k8sjobs: %w", err)
	}
	j.mu.Lock()
	j.up, j.namespaces = up, namespaces
	j.mu.Unlock()
	return nil
}
```

`Connect` receives the adapter's block from `fendlemark.yaml` as an `adapter.Config`. Its helpers do the type juggling: `Strings` accepts a YAML list, `Duration` accepts `"10s"` or a number of seconds, and `Secret` is for values that must never appear in logs. The control plane adds the common keys, so `cfg.Duration("timeout", …)` sees `adapters.k8sjobs.timeout`, which defaults to ten seconds.

The rule for `Connect` is to validate, build clients, and return quickly; it must not fetch facets, and the conformance suite fails it if it takes more than two seconds against the fixture. One request to `/version` is cheap and turns a wrong URL into an error at start rather than a confusing first poll. The new client is stored only once that request succeeds, so a failed `Connect` leaves the adapter unconnected rather than half connected.

For development, `kubectl proxy` serves the Kubernetes API on `http://127.0.0.1:8001` with your kubeconfig's credentials, which is why the token is optional. Inside a cluster, the API server's certificate is signed by the cluster's own CA, which this adapter does not load: either run `kubectl proxy` next to Fendlemark, or extend `Connect` to read the service account's `ca.crt` and `token` from `/var/run/secrets/kubernetes.io/serviceaccount/`. Either way, the identity needs `list` on `jobs` in the `batch` API group, in those namespaces, and nothing more.

## 4. Decode only what you use

```go
// jobList and job decode the parts of the batch/v1 JobList this adapter uses.
type jobList struct {
	Metadata struct {
		Continue string `json:"continue"`
	} `json:"metadata"`
	Items []job `json:"items"`
}

type job struct {
	Metadata struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		UID               string            `json:"uid"`
		CreationTimestamp time.Time         `json:"creationTimestamp"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
	} `json:"metadata"`
	Status struct {
		StartTime      time.Time `json:"startTime"`
		CompletionTime time.Time `json:"completionTime"`
		Active         int       `json:"active"`
		Succeeded      int       `json:"succeeded"`
		Failed         int       `json:"failed"`
		Conditions     []struct {
			Type               string    `json:"type"`
			Status             string    `json:"status"`
			Reason             string    `json:"reason"`
			LastTransitionTime time.Time `json:"lastTransitionTime"`
		} `json:"conditions"`
	} `json:"status"`
}
```

A Job has many more fields than these. Decoding only the ones the adapter reads keeps the adapter honest about what it depends on, and makes it indifferent to fields that newer Kubernetes versions add. Kubernetes writes times in RFC 3339, which `time.Time` decodes directly; an absent time decodes to the zero value, which the code below relies on.

## 5. Facets

```go
func (j *Jobs) Facets(ctx context.Context, q adapter.Query) ([]adapter.Facet, error) {
	j.mu.RLock()
	up, namespaces := j.up, j.namespaces
	j.mu.RUnlock()
	if up == nil {
		return nil, adapter.ErrNotConnected
	}
	if !q.Wants(adapter.KindRun) {
		return nil, nil
	}
	var out []adapter.Facet
	for _, ns := range namespaces {
		next := ""
		for {
			v := url.Values{"limit": {"500"}}
			if next != "" {
				v.Set("continue", next)
			}
			var list jobList
			path := "/apis/batch/v1/namespaces/" + url.PathEscape(ns) + "/jobs?" + v.Encode()
			if err := up.get(ctx, path, &list); err != nil {
				return nil, fmt.Errorf("k8sjobs: jobs in %s: %w", ns, err)
			}
			for _, jb := range list.Items {
				f := facetOf(jb)
				if !q.Since.IsZero() && f.StartedAt.Before(q.Since) {
					continue
				}
				out = append(out, f)
				if q.Limit > 0 && len(out) >= q.Limit {
					return out, nil
				}
			}
			if next = list.Metadata.Continue; next == "" {
				break
			}
		}
	}
	return out, nil
}
```

`Facets` is called once at start and then on every poll, and returns everything that matches the query. Three details:

- **Paging.** The list endpoint returns at most `limit` items and a `continue` token when there are more. The loop follows the token until it is empty, so a namespace with ten thousand Jobs costs twenty requests, not one enormous one.
- **`q.Since`** asks for facets that started after a time. The Jobs API cannot filter on start time, so the adapter filters after decoding.
- **Errors are whole.** If one namespace fails, `Facets` returns an error rather than a partial list. The control plane then keeps the facets it already has and marks the adapter degraded, which is better than reporting every Job in that namespace as gone.

## 6. From a Job to a facet

```go
func facetOf(jb job) adapter.Facet {
	m, s := jb.Metadata, jb.Status
	keys := map[string]string{}
	for annotation, key := range Annotations {
		if v := m.Annotations[annotation]; v != "" { // empty values fail Facet.Validate
			keys[key] = v
		}
	}
	started := s.StartTime
	if started.IsZero() {
		started = m.CreationTimestamp // a suspended Job has no startTime yet
	}
	phase, ended := phaseOf(jb)
	return adapter.Facet{
		Source:    "k8sjobs",
		SourceID:  m.Namespace + "/" + m.Name,
		Kind:      adapter.KindRun,
		Title:     m.Name,
		Status:    phase,
		StartedAt: started,
		EndedAt:   ended,
		Keys:      keys,
		Fields: map[string]any{
			"namespace": m.Namespace, "uid": m.UID, "labels": m.Labels,
			"active": s.Active, "succeeded": s.Succeeded, "failed": s.Failed,
		},
	}
}

// phaseOf maps a Job onto Fendlemark's five phases and returns when it ended.
// completionTime is only set when a Job succeeds, so a failed Job's end is
// the time its Failed condition became true.
func phaseOf(jb job) (adapter.Phase, time.Time) {
	for _, c := range jb.Status.Conditions {
		if c.Status != "True" {
			continue
		}
		switch c.Type {
		case "Complete":
			return adapter.Succeeded, jb.Status.CompletionTime
		case "Failed":
			return adapter.Failed, c.LastTransitionTime
		case "Suspended":
			return adapter.Pending, time.Time{}
		}
	}
	if jb.Status.Active > 0 || !jb.Status.StartTime.IsZero() {
		return adapter.Running, time.Time{}
	}
	return adapter.Pending, time.Time{}
}
```

This is where an adapter earns its keep, because it is where the upstream tool's vocabulary becomes Fendlemark's.

- **`SourceID`** is the namespace and name, which is unique per facet as the reconciler requires, and is how people refer to a Job.
- **`StartedAt`** must never be zero: `Facet.Validate` rejects it, and the correlation window is measured from it. A Job created suspended has no `startTime` until it is resumed, so the adapter falls back to `creationTimestamp`.
- **Keys** are added only when the annotation has a value. An empty key value also fails `Validate`, and a facet that fails validation is dropped by the lattice rather than shown.
- **Phases** come from the Job's conditions. `Complete` and `Failed` are terminal, and a Job never has both. The subtle part is the end time: Kubernetes sets `completionTime` only when a Job succeeds, so for a failed Job the adapter uses the time the `Failed` condition became true. Without that, a failed Job would have no end time, and a zero `EndedAt` means a run that is still going. Newer Kubernetes versions can also set a `FailureTarget` condition before `Failed`; the switch ignores it and waits for `Failed`.
- **`Fields`** is for display only and never used for correlation. The console renders it as a key-value table under the adapter's display name until you write a [facet panel](https://fendlemark.app/docs/ui/#panels).

## 7. Watch and Health

```go
func (j *Jobs) Watch(ctx context.Context) (<-chan adapter.Event, error) {
	return nil, adapter.ErrWatchUnsupported
}

// Health lists one Job per namespace: it proves the permission the adapter
// actually needs, not just that the API server answers.
func (j *Jobs) Health(ctx context.Context) adapter.Status {
	j.mu.RLock()
	up, namespaces := j.up, j.namespaces
	j.mu.RUnlock()
	if up == nil {
		return adapter.Status{State: adapter.Disabled, Detail: "not connected"}
	}
	start := time.Now()
	var failed []string
	var lastErr error
	for _, ns := range namespaces {
		var list jobList
		if err := up.get(ctx, "/apis/batch/v1/namespaces/"+url.PathEscape(ns)+"/jobs?limit=1", &list); err != nil {
			failed, lastErr = append(failed, ns), err
		}
	}
	lat := time.Since(start)
	switch {
	case len(failed) == len(namespaces):
		return adapter.Status{State: adapter.Unreachable, Detail: lastErr.Error(), Latency: lat}
	case len(failed) > 0:
		return adapter.Status{State: adapter.Degraded, Detail: "cannot list jobs in " + strings.Join(failed, ", "), Latency: lat}
	}
	return adapter.Status{State: adapter.Healthy, Detail: "listing jobs in " + strings.Join(namespaces, ", "), Latency: lat}
}
```

`Watch` returns `adapter.ErrWatchUnsupported`, which, with `Watchable: false`, is the whole of polling. The conformance suite checks that the two agree in both directions.

`Health` is what `fendlemark adapters check` and the console's adapter view show. It should be cheap and bounded by its context, and it is more useful when it checks what the adapter actually does: here, listing one Job in each namespace. A service account that can reach the API server but cannot list Jobs in one namespace is reported as `degraded` with the namespace named, instead of `healthy` followed by an empty view.

## 8. The HTTP client

```go
func (u *upstream) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.api+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if u.token != "" {
		req.Header.Set("Authorization", "Bearer "+u.token)
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", strings.SplitN(path, "?", 2)[0], resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
```

The standard library is enough for an adapter this size. The client's timeout and the request's context together bound every call, and errors name the path without its query string.

## 9. Test it with the conformance suite

Every adapter, built in or not, must pass `adapter/conformance`. It drives the adapter through its whole lifecycle against a fixture and checks the rules the reconciler depends on: a non-empty name, a pure descriptor, `Connect` within two seconds, valid facets with unique `Source` and `SourceID` and only declared keys, `Watch` agreeing with `Watchable`, and `Health` within its bound. It also flags a 32-hex-character value under any key but `mlflow.run_id` or `kfp.run_id`, which is almost always an MLflow run id filed under the wrong name.

The fixture is a recorded `JobList`, the response body of `GET /apis/batch/v1/namespaces/ml-training/jobs`, trimmed to the fields that matter. It holds three Jobs: a training Job that succeeded, an evaluation Job that failed, and a backfill that is still running. Save it as `testdata/jobs.json`:

```json
{
  "kind": "JobList",
  "apiVersion": "batch/v1",
  "metadata": {"resourceVersion": "48211903"},
  "items": [
    {
      "metadata": {
        "name": "train-resnet50-7xk2p",
        "namespace": "ml-training",
        "uid": "0b6f3d1e-58a2-4c7e-9f14-2d8e6a1b3c55",
        "creationTimestamp": "2026-10-22T08:59:58Z",
        "labels": {"team": "vision"},
        "annotations": {
          "fendlemark.io/mlflow-run-id": "6f1d2c9a8b7e4f30a1c5d9e8b2a47c13",
          "fendlemark.io/git-sha": "a41c9e07b2d3",
          "fendlemark.io/git-repo": "git@git.example.internal:ml/vision.git"
        }
      },
      "status": {
        "startTime": "2026-10-22T09:00:00Z",
        "completionTime": "2026-10-22T09:41:12Z",
        "succeeded": 1,
        "conditions": [
          {"type": "Complete", "status": "True", "lastTransitionTime": "2026-10-22T09:41:12Z"}
        ]
      }
    },
    {
      "metadata": {
        "name": "eval-resnet50-q9d4m",
        "namespace": "ml-training",
        "uid": "7c2a9e40-1f6b-4d38-8a5c-e93b0d7f2a61",
        "creationTimestamp": "2026-10-22T09:45:01Z",
        "annotations": {"fendlemark.io/git-sha": "a41c9e07b2d3", "fendlemark.io/git-repo": "git@git.example.internal:ml/vision.git"}
      },
      "status": {
        "startTime": "2026-10-22T09:45:02Z",
        "failed": 7,
        "conditions": [
          {"type": "FailureTarget", "status": "True", "reason": "BackoffLimitExceeded", "lastTransitionTime": "2026-10-22T09:52:40Z"},
          {"type": "Failed", "status": "True", "reason": "BackoffLimitExceeded", "lastTransitionTime": "2026-10-22T09:52:41Z"}
        ]
      }
    },
    {
      "metadata": {
        "name": "features-backfill-x4n7r",
        "namespace": "ml-training",
        "uid": "e4d8b6a2-93c1-47f0-b25e-6a1c8d0f9e37",
        "creationTimestamp": "2026-10-22T10:15:00Z",
        "annotations": {"fendlemark.io/argo-workflow": "nightly-features-l2v8c"}
      },
      "status": {"startTime": "2026-10-22T10:15:01Z", "active": 1, "ready": 1}
    }
  ]
}
```

The test serves it from an `httptest.Server` the way the API server would, and points the adapter's configuration at it. `conformance.Run` takes the constructor and a `conformance.Options` with the `Config` to pass to `Connect` and the minimum number of facets to expect:

```go
package k8sjobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/adapter/conformance"
	"fendlemark.app/fendlemark/lattice"
)

// fixture serves a recorded JobList the way the API server would.
func fixture(t *testing.T) *httptest.Server {
	t.Helper()
	jobs, err := os.ReadFile("testdata/jobs.json")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"major":"1","minor":"31","gitVersion":"v1.31.2"}`))
	})
	mux.HandleFunc("GET /apis/batch/v1/namespaces/ml-training/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jobs)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func config(srv *httptest.Server) adapter.Config {
	return adapter.Config{"api_url": srv.URL, "namespaces": []any{"ml-training"}}
}

func TestConformance(t *testing.T) {
	conformance.Run(t, New, conformance.Options{Config: config(fixture(t)), MinFacets: 3})
}
```

Conformance checks the contract, not the meaning. Two more tests check the meaning: that the phases and end times come out right, and that a Job and its MLflow run really do reconcile into one entity. The second builds a lattice in-process with the `lattice` package and an MLflow facet written by hand:

```go
func TestFacets(t *testing.T) {
	a := New()
	if err := a.Connect(context.Background(), config(fixture(t))); err != nil {
		t.Fatal(err)
	}
	facets, err := a.Facets(context.Background(), adapter.Query{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]adapter.Facet{}
	for _, f := range facets {
		got[f.SourceID] = f
	}
	train := got["ml-training/train-resnet50-7xk2p"]
	if train.Status != adapter.Succeeded || train.Keys[adapter.KeyMLflowRunID] != "6f1d2c9a8b7e4f30a1c5d9e8b2a47c13" {
		t.Errorf("train: %+v", train)
	}
	eval := got["ml-training/eval-resnet50-q9d4m"]
	if eval.Status != adapter.Failed || !eval.EndedAt.Equal(time.Date(2026, 10, 22, 9, 52, 41, 0, time.UTC)) {
		t.Errorf("eval: status %s, ended %s", eval.Status, eval.EndedAt)
	}
	if backfill := got["ml-training/features-backfill-x4n7r"]; backfill.Status != adapter.Running || !backfill.EndedAt.IsZero() {
		t.Errorf("backfill: %+v", backfill)
	}
}

// TestCorrelates checks the point of the exercise: a Job and the MLflow run
// it logs to become one entity. The run was created just before the Job.
func TestCorrelates(t *testing.T) {
	a := New()
	if err := a.Connect(context.Background(), config(fixture(t))); err != nil {
		t.Fatal(err)
	}
	facets, err := a.Facets(context.Background(), adapter.Query{})
	if err != nil {
		t.Fatal(err)
	}
	run := adapter.Facet{
		Source: "mlflow", SourceID: "6f1d2c9a8b7e4f30a1c5d9e8b2a47c13", Kind: adapter.KindRun,
		Title: "resnet50 lr=3e-4", Status: adapter.Succeeded,
		StartedAt: time.Date(2026, 10, 22, 8, 59, 57, 0, time.UTC),
		Keys:      map[string]string{adapter.KeyMLflowRunID: "6f1d2c9a8b7e4f30a1c5d9e8b2a47c13"},
	}
	l := lattice.New(lattice.Options{})
	l.Apply(append(facets, run))
	ents := l.Query(lattice.Query{Keys: map[string]string{adapter.KeyMLflowRunID: run.SourceID}})
	if len(ents) != 1 || len(ents[0].Facets) != 2 {
		t.Fatalf("want one entity with two facets, got %+v", ents)
	}
	ex, _ := l.Explain(ents[0].ID)
	if len(ex.Merges) != 1 || ex.Merges[0].Key != adapter.KeyMLflowRunID || ex.Merges[0].Gap != 3*time.Second {
		t.Fatalf("unexpected merges: %+v", ex.Merges)
	}
	t.Logf("%s %q: merged on %s, gap %s", ents[0].ID, ents[0].Title, ex.Merges[0].Key, ex.Merges[0].Gap)
}
```

```shell
$ go mod tidy
$ go test -v .
=== RUN   TestConformance
--- PASS: TestConformance (0.02s)
=== RUN   TestFacets
--- PASS: TestFacets (0.00s)
=== RUN   TestCorrelates
    k8sjobs_test.go:97: lt_074076a9a9bb28f9 "resnet50 lr=3e-4": merged on mlflow.run_id, gap 3s
--- PASS: TestCorrelates (0.00s)
PASS
ok  	example.com/k8sjobs	0.157s
```

The merged entity takes the MLflow run's title, because facets are ordered with the built-in adapters first and an entity is titled after its first facet. The training and evaluation Jobs are not merged, because they share only the weak keys `git.sha` and `git.repo`; the lattice relates them with a `SharesCommit` edge instead.

## 10. Run it

Fendlemark loads no plugins at runtime; adapters are compiled in. You do not need to change Fendlemark's own source to do that. A small `main` package that imports the adapter for its side effect, and then does what [embedding](https://fendlemark.app/docs/api/#embedding) describes, is a complete Fendlemark with one more adapter. Save it as `cmd/platform/main.go`:

```go
// Command platform is Fendlemark with one more adapter compiled in.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"fendlemark.app/fendlemark"
	"fendlemark.app/fendlemark/config"

	_ "example.com/k8sjobs" // registers "k8sjobs"
)

func main() {
	// Configuration validation accepts only adapters it knows about (DOC-110).
	config.KnownAdapters = append(config.KnownAdapters, "k8sjobs")

	cfg, _, err := config.Load("fendlemark.yaml")
	if err != nil {
		log.Fatal(err)
	}
	fm, err := fendlemark.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := fm.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
```

Importing `fendlemark.app/fendlemark` registers the four built-in adapters too, so this binary can read MLflow, Metaflow, Argo, and Kubeflow as well as Jobs. Configuration validation rejects an `adapters.<name>` block for an adapter it does not know, with `DOC-110`, so `main` adds the new name to `config.KnownAdapters` before loading the file.

Point it at the API through `kubectl proxy`:

```yaml
adapters:
  k8sjobs:
    enabled: true
    api_url: "http://127.0.0.1:8001"
    namespaces: ["ml-training"]
```

```shell
$ kubectl proxy &
$ go build -o platform ./cmd/platform
$ ./platform
INFO fendlemark starting version=0.9.5 adapters=k8sjobs
INFO adapter connected adapter=k8sjobs took=19ms
INFO lattice reconciled entities=3 facets=3 generation=1 took=0s
INFO console listening addr=127.0.0.1:7411
```

(The log's timestamps are trimmed here.) The console is on [http://localhost:7411](http://localhost:7411) as usual, and the API reports the new adapter alongside any others you enable:

```shell
$ curl -s http://127.0.0.1:7411/api/v1/adapters | jq '.adapters[0] | {name, state, detail, maturity}'
{
  "name": "k8sjobs",
  "state": "healthy",
  "detail": "listing jobs in ml-training · 3 facets",
  "maturity": "Provisional"
}
```

Enable the `mlflow` adapter in the same file, and every Job annotated with `fendlemark.io/mlflow-run-id` appears once, with both facets attached, as `TestCorrelates` showed. That holds as long as the run and the Job start within the correlation window of each other, which they do when the run is created just before the Job.

## Where to go from here

- **Watch.** The Kubernetes API supports watching Jobs. To use it, set `Watchable: true` and return a channel of `adapter.Event` values that closes when the context passed to `Watch` is cancelled. The control plane calls `Facets` once for the initial load and reconnects with backoff when the channel closes, and the conformance suite checks the channel closes within two seconds of cancellation.
- **A facet panel.** The default key-value table is fine for `Fields` this small. A Job with many pods deserves a panel; [The console](https://fendlemark.app/docs/ui/#panels) explains how panels are written and registered.
- **Upstream.** If your adapter would be useful to everyone, the [Fendlemark Contribution Protocol](https://fendlemark.app/governance/contributing/) describes how to propose it. It would enter at Provisional, and Provisional adapters are not compiled into release binaries; the [maturity ladder](https://fendlemark.app/governance/#maturity) describes what it takes to reach Settled, when they are.

## Further reading

- [Writing an adapter](https://fendlemark.app/docs/adapters/writing-an-adapter/): the interface, facets, lifecycle, and conformance rules
- [Adapters](https://fendlemark.app/docs/adapters/): what the four built-in adapters read and which keys they provide
- [Architecture: correlation keys](https://fendlemark.app/docs/architecture/#correlation)
- [Go API reference: package adapter](https://fendlemark.app/docs/api/#adapter)
