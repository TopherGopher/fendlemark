# Writing an adapter

Source: https://fendlemark.develops.guru/docs/adapters/writing-an-adapter/

An adapter is a Go package that implements one interface with six methods. If you can list things from your tool's API and say what they are, you can write one in an afternoon. This page walks through the interface, the conformance suite, and how an adapter ends up in the binary.

## The interface

```go
package adapter // fendlemark.develops.guru/fendlemark/adapter

type Adapter interface {
    // Name returns the adapter's stable identifier, used in config and facets.
    Name() string

    // Describe returns static metadata: facet kinds, correlation keys,
    // maturity, and whether Watch is supported.
    Describe() Descriptor

    // Connect validates configuration and establishes upstream clients.
    // It must not fetch facets.
    Connect(ctx context.Context, cfg Config) error

    // Facets returns all facets matching q. Called on start and on each poll.
    Facets(ctx context.Context, q Query) ([]Facet, error)

    // Watch streams incremental facet events. Return ErrWatchUnsupported
    // and the control plane will poll Facets instead.
    Watch(ctx context.Context) (<-chan Event, error)

    // Health reports current status for the console and `adapters check`.
    Health(ctx context.Context) Status
}
```

That is the whole contract. Everything else on this page is detail.

## Facets

```go
type Facet struct {
    Source    string            // adapter name
    SourceID  string            // identifier in the upstream tool
    Kind      Kind              // KindRun, KindWorkflow, KindArtifact, KindModel, KindPipeline, KindNotebook
    Title     string
    Status    Phase             // Pending, Running, Succeeded, Failed, Unknown
    StartedAt time.Time
    EndedAt   time.Time         // zero if still running
    Keys      map[string]string // correlation keys, e.g. "argo.workflow.name": "train-abc12"
    Fields    map[string]any    // everything else; rendered by the facet panel
    Links     []Link            // deep links into the upstream tool
}
```

Rules the reconciler relies on:

- `Source` plus `SourceID` is unique per facet. Emitting the same pair twice in one `Facets` call is a conformance failure.
- `Keys` use the shared vocabulary (`argo.workflow.name`, `mlflow.run_id`, …) wherever one applies. Invent a new key only when none applies, and prefix it with your adapter name.
- `StartedAt` must be set. The correlation window is measured from it.
- `Fields` is for display. Nothing in it is used for correlation, so it can be as rich or as sparse as your tool allows.

## Descriptor

```go
func (a *Adapter) Describe() adapter.Descriptor {
    return adapter.Descriptor{
        DisplayName:  "Example Tracker",
        Kinds:        []adapter.Kind{adapter.KindRun, adapter.KindArtifact},
        Keys:         []string{"mlflow.run_id", "git.sha", "example.job_id"},
        Watchable:    false,
        Maturity:     adapter.Provisional,
        ConfigSchema: exampleSchema, // JSON Schema; validated before Connect
    }
}
```

The control plane uses the descriptor to validate configuration before calling `Connect`, to decide whether to poll or watch, to populate the console's adapter panel, and to attach your facet panel to the right views.

## Lifecycle

1. **Construct.** Your package registers a constructor with `adapter.Register("example", New)` in an `init` function. The binary's `cmd/fendlemark` imports the package for side effects.
2. **Describe.** Called once. Must be pure.
3. **Connect.** Called once with validated config. Build clients, check credentials, return quickly. If you need more than a few seconds, you are doing `Facets` work here; stop.
4. **Facets** or **Watch.** If `Watchable`, the control plane calls `Facets` once for the initial load, then `Watch` for the rest of the process lifetime, reconnecting with backoff if the channel closes. Otherwise it calls `Facets` every `poll_interval`.
5. **Health.** Called on demand. Should be cheap and should not block on upstream for more than `timeout`.
6. **Shutdown.** The context passed to `Watch` is cancelled. Return.

## A minimal adapter

```go
package example

import (
    "context"
    "fendlemark.develops.guru/fendlemark/adapter"
)

func init() { adapter.Register("example", New) }

type Example struct{ client *trackerClient }

func New() adapter.Adapter { return &Example{} }

func (e *Example) Name() string { return "example" }

func (e *Example) Connect(ctx context.Context, cfg adapter.Config) error {
    e.client = newTrackerClient(cfg.String("url"), cfg.Secret("token"))
    return e.client.Ping(ctx)
}

func (e *Example) Facets(ctx context.Context, q adapter.Query) ([]adapter.Facet, error) {
    jobs, err := e.client.ListJobs(ctx, q.Since)
    if err != nil { return nil, err }
    out := make([]adapter.Facet, 0, len(jobs))
    for _, j := range jobs {
        out = append(out, adapter.Facet{
            Source: "example", SourceID: j.ID, Kind: adapter.KindRun,
            Title: j.Name, Status: phaseOf(j.State), StartedAt: j.Started, EndedAt: j.Ended,
            Keys: map[string]string{"example.job_id": j.ID, "mlflow.run_id": j.Labels["mlflow_run"]},
            Fields: map[string]any{"queue": j.Queue, "gpus": j.GPUs},
            Links: []adapter.Link{{Label: "Open in Example", URL: e.client.URLFor(j)}},
        })
    }
    return out, nil
}

func (e *Example) Watch(ctx context.Context) (<-chan adapter.Event, error) {
    return nil, adapter.ErrWatchUnsupported
}

func (e *Example) Health(ctx context.Context) adapter.Status {
    if err := e.client.Ping(ctx); err != nil {
        return adapter.Status{State: adapter.Unreachable, Detail: err.Error()}
    }
    return adapter.Status{State: adapter.Healthy, Detail: "ok"}
}
```

## The conformance suite

Every adapter, built-in or not, must pass `adapter/conformance`. It drives your adapter against a recorded fixture and checks the rules above plus a few that are only obvious after someone has broken them:

```go
func TestConformance(t *testing.T) {
    conformance.Run(t, New, conformance.Options{
        Fixture:  "testdata/jobs.json",
        Config:   adapter.Config{"url": "http://fixture", "token": "x"},
        MinFacets: 3,
    })
}
```

- Unique `Source` + `SourceID` per call.
- Non-zero `StartedAt` on every facet.
- `Describe` is pure and returns the same value twice.
- `Connect` completes within two seconds against the fixture.
- `Health` returns within `timeout` when upstream is unreachable.
- If `Watchable`, the channel closes cleanly on context cancellation.
- Keys use the shared vocabulary when they match a known pattern (a 32-hex-character key named anything other than `mlflow.run_id` is flagged).

## Getting it into the binary

Fendlemark does not load plugins at runtime. Adapters are Go packages compiled in, which keeps the binary static, the startup instant, and the attack surface small. To run your adapter:

```shell
$ git clone https://fendlemark.develops.guru/fendlemark   # resolves via go-import
$ cd fendlemark
$ # add  _ "yourmodule/example"  to cmd/fendlemark/adapters.go
$ go build ./cmd/fendlemark
```

To propose your adapter for inclusion in the official binary, so that everyone gets it, follow the [Fendlemark Contribution Protocol](https://fendlemark.develops.guru/governance/contributing/). Adapters enter at Provisional maturity and advance on the schedule described in [Governance](https://fendlemark.develops.guru/governance/#maturity).

## The facet panel

By default the console renders your facet's `Fields` as a key-value table under your adapter's display name. For a custom panel, add a Vue component to `console/src/panels/example/` and register it in `console/src/panels/index.ts`. The component receives the facet as a prop and is rendered inside `<FmFacetPanel>`, which provides the header, the deep links, and the collapse state. See [The console](https://fendlemark.develops.guru/docs/ui/#panels).
