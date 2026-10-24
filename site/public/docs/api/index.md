# Go API reference

Source: https://fendlemark.app/docs/api/

Module path `fendlemark.app/fendlemark`. Go 1.23 or later. These are the packages you import to embed Fendlemark, write an adapter, or query a lattice from your own code. Full documentation is available with `go doc`.

## Packages

| Import path | Purpose |
| --- | --- |
| `fendlemark.app/fendlemark` | Top-level: `Version`, `New`, `Run`. Embedding entry point. |
| `…/fendlemark/adapter` | The `Adapter` interface, `Facet`, `Descriptor`, registration. |
| `…/fendlemark/adapter/conformance` | Test harness for adapters. |
| `…/fendlemark/adapters/mlflow`, `…/metaflow`, `…/argo`, `…/kubeflow` | The built-in adapters. Import for side effects to register. |
| `…/fendlemark/lattice` | Entities, correlations, the reconciler, and queries. |
| `…/fendlemark/config` | Loading, validation, precedence. |
| `…/fendlemark/server` | HTTP server, API, SSE, embedded console. |
| `…/fendlemark/auth/fri` | Respiratory Identity: signature matching and enrolment store. |

## package fendlemark

```go
const Version = "0.9.5"

// New constructs a Fendlemark instance from configuration. Adapters
// must already be registered (import their packages for side effects).
func New(cfg *config.Config, opts ...Option) (*Fendlemark, error)

// Run connects adapters, reconciles, serves, and blocks until ctx is done.
func (f *Fendlemark) Run(ctx context.Context) error

// Lattice returns the live lattice for in-process queries.
func (f *Fendlemark) Lattice() *lattice.Lattice

type Option func(*options)
func WithLogger(l *slog.Logger) Option
func WithAdapters(names ...string) Option   // restrict to a subset
func WithConsole(fs fs.FS) Option           // replace the embedded console
```

### Embedding

```go
import (
    "fendlemark.app/fendlemark"
    "fendlemark.app/fendlemark/config"
    _ "fendlemark.app/fendlemark/adapters/mlflow"
    _ "fendlemark.app/fendlemark/adapters/argo"
)

func main() {
    cfg, err := config.Load("fendlemark.yaml")
    if err != nil { log.Fatal(err) }
    fm, err := fendlemark.New(cfg)
    if err != nil { log.Fatal(err) }
    log.Fatal(fm.Run(context.Background()))
}
```

## package adapter

Documented in full on [Writing an adapter](https://fendlemark.app/docs/adapters/writing-an-adapter/). Summary of exports:

```go
type Adapter interface { Name; Describe; Connect; Facets; Watch; Health }
type Facet struct { Source, SourceID string; Kind Kind; Title string; Status Phase; StartedAt, EndedAt time.Time; Keys map[string]string; Fields map[string]any; Links []Link }
type Descriptor struct { DisplayName string; Kinds []Kind; Keys []string; Watchable bool; Maturity Maturity; ConfigSchema json.RawMessage }
type Event struct { Op Op; Facet Facet }            // Op: Upsert, Delete
type Query struct { Since time.Time; Kinds []Kind; Limit int }
type Status struct { State State; Detail string; Latency time.Duration }
type Config map[string]any
func (c Config) String(key string) string
func (c Config) Secret(key string) string   // resolved, never logged
func (c Config) Duration(key string) time.Duration

type Kind string      // KindRun, KindWorkflow, KindArtifact, KindModel, KindPipeline, KindNotebook
type Phase string     // Pending, Running, Succeeded, Failed, Unknown
type State string     // Healthy, Degraded, Unreachable, Disabled
type Maturity string  // Provisional, Settled, Established, Canonical

func Register(name string, ctor func() Adapter)
func Registered() []string

var ErrWatchUnsupported = errors.New("adapter: watch unsupported")
var ErrNotConnected    = errors.New("adapter: not connected")
```

## package lattice

```go
type Lattice struct { /* unexported */ }

func New(opts Options) *Lattice
func (l *Lattice) Apply(facets []adapter.Facet) Delta        // full reconciliation
func (l *Lattice) ApplyEvent(ev adapter.Event) Delta         // incremental
func (l *Lattice) Get(id ID) (Entity, bool)
func (l *Lattice) Query(q Query) []Entity
func (l *Lattice) Explain(id ID) Explanation
func (l *Lattice) Generation() uint64
func (l *Lattice) Subscribe(ctx context.Context) <-chan Delta

type ID string                                              // "lt_" + 16 hex
type Entity struct { ID ID; Kind adapter.Kind; Title string; Status adapter.Phase; Facets []adapter.Facet; Related []Relation }
type Relation struct { To ID; Via string; Rel RelKind }    // RelKind: ProducedBy, DerivedFrom, DeployedAs, SharesCommit
type Query struct { Kinds []adapter.Kind; Keys map[string]string; Status []adapter.Phase; Since, Until time.Time; Limit, Offset int }
type Delta struct { Upserted, Removed []ID; Generation uint64 }
type Explanation struct { Entity Entity; Merges []MergeStep; Relations []RelationStep }
type Options struct { CorrelationWindow time.Duration; WeakKeys bool; CustomKeys []KeyDef; RetainOrphans time.Duration; MaxEntities int }
```

The reconciler is deterministic: `Apply` with the same facets on two fresh lattices yields identical entities and identical ids. Ids are derived from the sorted set of `(Source, SourceID)` pairs that merged, so an entity keeps its id as long as the same facets describe it.

## package server

```go
func New(l *lattice.Lattice, cfg *config.Config, opts ...Option) *Server
func (s *Server) Handler() http.Handler    // mount in your own mux
func (s *Server) ListenAndServe(ctx context.Context) error
```

### HTTP API

| Method and path | Returns |
| --- | --- |
| `GET /api/v1/entities?kind=&status=&key=&since=&until=&limit=&offset=` | Paged entities, with the total and the lattice generation. `kind`, `status`, and `key` take comma-separated lists; `limit` defaults to 50, at most 5000. |
| `GET /api/v1/entities/{id}` | One entity with facets and relations. |
| `GET /api/v1/entities/{id}/explain` | Why it was reconciled the way it was. |
| `GET /api/v1/adapters` | Status per adapter. |
| `GET /api/v1/lattice/stats` | Counts and generation. |
| `GET /api/v1/events?entity=&kind=` | Server-sent events stream of deltas. |
| `GET /api/v1/console` | Console settings (title, theme, default view, page size, and the like), the auth mode, and the server version. |
| `GET /healthz`, `GET /readyz`, `GET /metrics` | Liveness, readiness, Prometheus. |
| `POST /api/v1/auth/fri/signin`, `POST /api/v1/auth/fri/enrol/{token}` | Respiratory Identity sign-in and enrolment. Browser only. |
| `GET /api/v1/auth/session`, `POST /api/v1/auth/signout` | The current session, and signing out. |

There are no `POST`, `PUT`, or `DELETE` routes against entities. The API is a view of the lattice, and the lattice is a view of your tools.

The full machine-readable description is [openapi.json](https://fendlemark.app/openapi.json) (OpenAPI 3.1).

## package auth/fri

```go
type Signature [212]float32
type Enrolment struct { User string; Role Role; Centroid Signature; Samples int; UpdatedAt time.Time }
type Store interface { Get(user string) (Enrolment, error); Put(Enrolment) error; Delete(user string) error; List() ([]Enrolment, error) }
func Match(candidate Signature, e Enrolment, sensitivity float32) (score float32, ok bool)
func Classify(candidate Signature) Dominance   // Nasal, Mouth, Mixed

var ErrNotRecognised = Error{Code: "FRI-0042"}
var ErrExcited       = Error{Code: "FRI-0031"}
```

## Stability

Fendlemark is pre-1.0. The `adapter` package has been stable since 0.6.0 and the Keepers have committed, in [FDR-027](https://fendlemark.app/governance/#decisions), not to change it before 1.0 "or after". Other packages may change between minor versions; every change is listed in the [changelog](https://fendlemark.app/changelog/) and, where a key or field is renamed, in a [migration guide](https://fendlemark.app/docs/migration/).
