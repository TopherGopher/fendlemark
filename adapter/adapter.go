// Package adapter defines the contract between Fendlemark's control plane and
// the systems it reads from. An Adapter translates one external system
// (an MLflow tracking server, a Metaflow metadata service, an Argo Server, a
// Kubeflow Pipelines API) into Facets. Adapters are strictly read-only.
//
// The interface has six methods and has been stable since 0.6.0 (FDR-027).
package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Kind classifies the real-world thing a facet describes.
type Kind string

// Facet kinds. The lattice resolves a merged entity's kind by precedence in
// this order: Run, Workflow, Pipeline, Model, Artifact, Notebook.
const (
	KindRun      Kind = "run"
	KindWorkflow Kind = "workflow"
	KindPipeline Kind = "pipeline"
	KindModel    Kind = "model"
	KindArtifact Kind = "artifact"
	KindNotebook Kind = "notebook"
)

// Phase is a tool-agnostic lifecycle state. Every adapter maps its upstream
// vocabulary (FINISHED, Succeeded, SUCCEEDED, completed) onto these five.
type Phase string

const (
	Pending   Phase = "Pending"
	Running   Phase = "Running"
	Succeeded Phase = "Succeeded"
	Failed    Phase = "Failed"
	Unknown   Phase = "Unknown"
)

// State is an adapter's health.
type State string

const (
	Healthy     State = "healthy"
	Degraded    State = "degraded"
	Unreachable State = "unreachable"
	Disabled    State = "disabled"
)

// Maturity is an adapter's position on the ladder defined in Governance.
type Maturity string

const (
	Provisional Maturity = "Provisional"
	Settled     Maturity = "Settled"
	Established Maturity = "Established"
	Canonical   Maturity = "Canonical"
)

// Op is the kind of change an Event carries.
type Op string

const (
	Upsert Op = "upsert"
	Delete Op = "delete"
)

// Link is a deep link into the upstream tool. Fendlemark shows; the tools operate.
type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Facet is one tool's view of one thing. Facets are never merged with each
// other; they are attached to entities by the lattice.
type Facet struct {
	Source    string            `json:"source"`
	SourceID  string            `json:"source_id"`
	Kind      Kind              `json:"kind"`
	Title     string            `json:"title"`
	Status    Phase             `json:"status"`
	StartedAt time.Time         `json:"started_at"`
	EndedAt   time.Time         `json:"ended_at,omitempty"`
	Keys      map[string]string `json:"keys,omitempty"`
	Fields    map[string]any    `json:"fields,omitempty"`
	Links     []Link            `json:"links,omitempty"`
}

// Ref is the unique identity of a facet within the lattice.
func (f Facet) Ref() string { return f.Source + "\x00" + f.SourceID }

// Validate reports the first rule the facet breaks, or nil.
func (f Facet) Validate() error {
	switch {
	case f.Source == "":
		return errors.New("facet: Source is empty")
	case f.SourceID == "":
		return fmt.Errorf("facet %s: SourceID is empty", f.Source)
	case f.Kind == "":
		return fmt.Errorf("facet %s/%s: Kind is empty", f.Source, f.SourceID)
	case f.StartedAt.IsZero():
		return fmt.Errorf("facet %s/%s: StartedAt is zero", f.Source, f.SourceID)
	}
	for k, v := range f.Keys {
		if k == "" || v == "" {
			return fmt.Errorf("facet %s/%s: empty correlation key or value", f.Source, f.SourceID)
		}
	}
	return nil
}

// Event is an incremental change from a watching adapter.
type Event struct {
	Op    Op    `json:"op"`
	Facet Facet `json:"facet"`
}

// Query narrows a Facets call. A zero Query means everything.
type Query struct {
	Since time.Time
	Kinds []Kind
	Limit int
}

// Wants reports whether the query accepts a facet of the given kind.
func (q Query) Wants(k Kind) bool {
	if len(q.Kinds) == 0 {
		return true
	}
	for _, kk := range q.Kinds {
		if kk == k {
			return true
		}
	}
	return false
}

// Status is an adapter's self-reported health.
type Status struct {
	State   State         `json:"state"`
	Detail  string        `json:"detail"`
	Latency time.Duration `json:"latency"`
}

// Descriptor is static metadata about an adapter. Describe must be pure.
type Descriptor struct {
	DisplayName  string          `json:"display_name"`
	Kinds        []Kind          `json:"kinds"`
	Keys         []string        `json:"keys"`
	Watchable    bool            `json:"watchable"`
	Maturity     Maturity        `json:"maturity"`
	ConfigSchema json.RawMessage `json:"config_schema,omitempty"`
}

// Config is the adapter-specific block from fendlemark.yaml, already merged
// with environment overrides. Secrets resolved through Secret are never logged.
type Config map[string]any

// String returns a string option or "".
func (c Config) String(key string) string {
	switch v := c[key].(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// Secret returns a string option that must never appear in logs or output.
func (c Config) Secret(key string) string { return c.String(key) }

// Bool returns a boolean option, or def when unset.
func (c Config) Bool(key string, def bool) bool {
	if v, ok := c[key].(bool); ok {
		return v
	}
	return def
}

// Int returns an integer option, or def when unset or not numeric.
func (c Config) Int(key string, def int) int {
	switch v := c[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return def
}

// Duration returns a duration option, or def when unset or unparsable.
func (c Config) Duration(key string, def time.Duration) time.Duration {
	switch v := c[key].(type) {
	case time.Duration:
		return v
	case string:
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	case int:
		return time.Duration(v) * time.Second
	case float64:
		return time.Duration(v * float64(time.Second))
	}
	return def
}

// Strings returns a list option.
func (c Config) Strings(key string) []string {
	switch v := c[key].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			out = append(out, fmt.Sprint(e))
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	}
	return nil
}

// Adapter is the whole contract. See Writing an adapter in the documentation.
type Adapter interface {
	// Name returns the adapter's stable identifier, used in config and facets.
	Name() string
	// Describe returns static metadata. It must be pure.
	Describe() Descriptor
	// Connect validates configuration and establishes upstream clients.
	// It must not fetch facets.
	Connect(ctx context.Context, cfg Config) error
	// Facets returns all facets matching q.
	Facets(ctx context.Context, q Query) ([]Facet, error)
	// Watch streams incremental events, or returns ErrWatchUnsupported.
	Watch(ctx context.Context) (<-chan Event, error)
	// Health reports current status.
	Health(ctx context.Context) Status
}

// Closer is optionally implemented by adapters holding resources.
type Closer interface{ Close() error }

var (
	// ErrWatchUnsupported is returned by Watch when the adapter must be polled.
	ErrWatchUnsupported = errors.New("adapter: watch unsupported")
	// ErrNotConnected is returned when Facets or Watch is called before Connect.
	ErrNotConnected = errors.New("adapter: not connected")
)

var (
	regMu    sync.RWMutex
	registry = map[string]func() Adapter{}
)

// Register makes an adapter constructor available by name. It is called from
// the adapter package's init; cmd/fendlemark imports adapters for side effects.
func Register(name string, ctor func() Adapter) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := registry[name]; dup {
		panic("adapter: duplicate registration of " + name)
	}
	registry[name] = ctor
}

// Registered returns the registered adapter names, sorted.
func Registered() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Lookup returns a new instance of the named adapter.
func Lookup(name string) (Adapter, bool) {
	regMu.RLock()
	ctor, ok := registry[name]
	regMu.RUnlock()
	if !ok {
		return nil, false
	}
	return ctor(), true
}

// Shared correlation key vocabulary. Adapters use these names wherever one
// applies and prefix adapter-specific keys with their own name.
const (
	KeyMLflowRunID      = "mlflow.run_id"
	KeyMLflowExperiment = "mlflow.experiment_id"
	KeyMetaflowRunID    = "metaflow.run_id"
	KeyMetaflowFlow     = "metaflow.flow_name"
	KeyArgoWorkflow     = "argo.workflow.name"
	KeyArgoUID          = "argo.workflow.uid"
	KeyKFPRunID         = "kfp.run_id"
	KeyKFPPipelineID    = "kfp.pipeline_id"
	KeyGitSHA           = "git.sha"
	KeyGitRepo          = "git.repo"
)

// IsWeak reports whether a key may only create related-to edges, never merges.
func IsWeak(key string) bool { return key == KeyGitSHA || key == KeyGitRepo }
