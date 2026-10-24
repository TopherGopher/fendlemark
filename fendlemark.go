// Package fendlemark is the control plane: it constructs adapters from
// configuration, connects them, reconciles their facets into the lattice,
// keeps the lattice current by polling or watching, and serves the console.
//
// Most embedders want every adapter they have credentials for, connected and
// reconciled, without writing configuration. That is IntegrateEasy.
package fendlemark

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/auth/fri"
	"fendlemark.app/fendlemark/config"
	"fendlemark.app/fendlemark/internal/schema"
	"fendlemark.app/fendlemark/lattice"
	"fendlemark.app/fendlemark/server"

	// The four Canonical adapters register themselves on import.
	_ "fendlemark.app/fendlemark/adapters/argo"
	_ "fendlemark.app/fendlemark/adapters/kubeflow"
	_ "fendlemark.app/fendlemark/adapters/metaflow"
	_ "fendlemark.app/fendlemark/adapters/mlflow"
)

// Version is the release version, shared with @fendlemark/ui.
const Version = "0.9.6"

// Option configures New and IntegrateEasy.
type Option func(*options)

type options struct {
	logger   *slog.Logger
	adapters []string
	console  fs.FS
	noServe  bool
}

// WithLogger sets the logger. Default: slog.Default().
func WithLogger(l *slog.Logger) Option { return func(o *options) { o.logger = l } }

// WithAdapters restricts the instance to a subset of enabled adapters.
func WithAdapters(names ...string) Option { return func(o *options) { o.adapters = names } }

// WithConsole replaces the embedded console (for example a Vite build).
func WithConsole(f fs.FS) Option { return func(o *options) { o.console = f } }

type runtimeAdapter struct {
	a      adapter.Adapter
	cfg    config.Adapter
	desc   adapter.Descriptor
	mu     sync.RWMutex
	status adapter.Status
	facets int
	lastOK time.Time
}

// Fendlemark is a running (or runnable) instance.
type Fendlemark struct {
	cfg      *config.Config
	opts     options
	log      *slog.Logger
	lat      *lattice.Lattice
	adapters map[string]*runtimeAdapter
	srv      *server.Server
	store    fri.Store
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.Mutex
	started  bool
}

// New constructs an instance from configuration. Adapters are constructed
// but not connected; call Connect, or use Run or IntegrateEasy.
func New(cfg *config.Config, opts ...Option) (*Fendlemark, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	o := options{logger: slog.Default()}
	for _, opt := range opts {
		opt(&o)
	}
	f := &Fendlemark{cfg: cfg, opts: o, log: o.logger, adapters: map[string]*runtimeAdapter{}}
	f.lat = lattice.New(lattice.Options{
		CorrelationWindow: cfg.Lattice.Correlation.Window,
		NoWeakKeys:        !cfg.Lattice.Correlation.WeakKeys,
		RetainOrphans:     cfg.Lattice.RetainOrphans,
		MaxEntities:       cfg.Lattice.MaxEntities,
	})
	for _, k := range cfg.Lattice.Correlation.Custom {
		_ = k // custom keys are honoured by adapters that read them; the lattice treats unknown keys as strong unless Weak
	}
	only := map[string]bool{}
	for _, n := range o.adapters {
		only[n] = true
	}
	for _, name := range cfg.Enabled() {
		if len(only) > 0 && !only[name] {
			continue
		}
		a, ok := adapter.Lookup(name)
		if !ok {
			return nil, fmt.Errorf("fendlemark: adapter %q is enabled but not compiled into this binary", name)
		}
		desc := a.Describe()
		// config.Load has usually checked this already; a Config built in code has not.
		if err := schema.Check(desc.ConfigSchema, cfg.Adapters[name].Options); err != nil {
			return nil, fmt.Errorf("fendlemark: adapters.%s: %w", name, err)
		}
		f.adapters[name] = &runtimeAdapter{a: a, cfg: cfg.Adapters[name], desc: desc, status: adapter.Status{State: adapter.Disabled, Detail: "not connected"}}
	}
	if cfg.Auth.Mode == "respiratory" {
		st, err := fri.NewFileStore(cfg.Server.DataDir)
		if err != nil {
			return nil, fmt.Errorf("fendlemark: respiratory identity store: %w", err)
		}
		f.store = st
	}
	sopts := []server.Option{server.WithLogger(f.log), server.WithVersion(Version), server.WithAdapterStatus(f.AdapterStatuses)}
	if o.console != nil {
		sopts = append(sopts, server.WithConsole(o.console))
	}
	if f.store != nil {
		sopts = append(sopts, server.WithEnrolments(f.store))
	}
	f.srv = server.New(f.lat, cfg, sopts...)
	return f, nil
}

// Lattice returns the live lattice for in-process queries.
func (f *Fendlemark) Lattice() *lattice.Lattice { return f.lat }

// Server returns the HTTP server, for mounting its Handler elsewhere.
func (f *Fendlemark) Server() *server.Server { return f.srv }

// Config returns the effective configuration.
func (f *Fendlemark) Config() *config.Config { return f.cfg }

// Adapters returns the names of constructed adapters, sorted.
func (f *Fendlemark) Adapters() []string {
	names := make([]string, 0, len(f.adapters))
	for n := range f.adapters {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (ra *runtimeAdapter) adapterConfig() adapter.Config {
	c := adapter.Config{}
	for k, v := range ra.cfg.Options {
		c[k] = v
	}
	c["timeout"] = ra.cfg.Timeout
	c["poll_interval"] = ra.cfg.PollInterval
	if len(ra.cfg.Labels) > 0 {
		m := map[string]any{}
		for k, v := range ra.cfg.Labels {
			m[k] = v
		}
		c["labels"] = m
	}
	return c
}

// Connect connects every adapter concurrently. An adapter that fails to
// connect is marked unreachable and does not prevent the others from
// starting; the error returned is the joined error of all failures.
func (f *Fendlemark) Connect(ctx context.Context) error {
	var wg sync.WaitGroup
	errs := make([]error, 0)
	var emu sync.Mutex
	for name, ra := range f.adapters {
		wg.Add(1)
		go func(name string, ra *runtimeAdapter) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, ra.cfg.Timeout+5*time.Second)
			defer cancel()
			start := time.Now()
			err := ra.a.Connect(cctx, ra.adapterConfig())
			ra.mu.Lock()
			defer ra.mu.Unlock()
			if err != nil {
				ra.status = adapter.Status{State: adapter.Unreachable, Detail: err.Error(), Latency: time.Since(start)}
				emu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
				emu.Unlock()
				f.log.Warn("adapter connect failed", "adapter", name, "err", err)
				return
			}
			ra.status = adapter.Status{State: adapter.Healthy, Detail: "connected", Latency: time.Since(start)}
			f.log.Info("adapter connected", "adapter", name, "took", time.Since(start).Round(time.Millisecond))
		}(name, ra)
	}
	wg.Wait()
	return errors.Join(errs...)
}

// Reconcile fetches facets from every connected adapter and applies them.
func (f *Fendlemark) Reconcile(ctx context.Context) (lattice.Delta, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var all []adapter.Facet
	var errs []error
	for name, ra := range f.adapters {
		ra.mu.RLock()
		st := ra.status.State
		ra.mu.RUnlock()
		if st == adapter.Unreachable || st == adapter.Disabled {
			continue
		}
		wg.Add(1)
		go func(name string, ra *runtimeAdapter) {
			defer wg.Done()
			start := time.Now()
			facets, err := ra.a.Facets(ctx, adapter.Query{})
			ra.mu.Lock()
			defer ra.mu.Unlock()
			if err != nil {
				ra.status = adapter.Status{State: adapter.Degraded, Detail: err.Error(), Latency: time.Since(start)}
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
				mu.Unlock()
				return
			}
			if ra.cfg.MaxEntities > 0 && len(facets) > ra.cfg.MaxEntities {
				sort.Slice(facets, func(i, j int) bool { return facets[i].StartedAt.After(facets[j].StartedAt) })
				facets = facets[:ra.cfg.MaxEntities]
			}
			ra.facets = len(facets)
			ra.lastOK = time.Now()
			ra.status = adapter.Status{State: adapter.Healthy, Detail: fmt.Sprintf("%d facets", len(facets)), Latency: time.Since(start)}
			mu.Lock()
			all = append(all, facets...)
			mu.Unlock()
		}(name, ra)
	}
	wg.Wait()
	start := time.Now()
	d := f.lat.Apply(all)
	st := f.lat.Stats()
	f.log.Info("lattice reconciled", "entities", st.Entities, "facets", st.Facets, "generation", d.Generation, "took", time.Since(start).Round(time.Millisecond))
	f.srv.SetReady(true)
	return d, errors.Join(errs...)
}

// Start begins the poll and watch loops. It returns immediately.
func (f *Fendlemark) Start(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.started {
		return
	}
	f.started = true
	ctx, f.cancel = context.WithCancel(ctx)
	for name, ra := range f.adapters {
		f.wg.Add(1)
		go f.loop(ctx, name, ra)
	}
}

func (f *Fendlemark) loop(ctx context.Context, name string, ra *runtimeAdapter) {
	defer f.wg.Done()
	poll := ra.cfg.PollInterval
	if poll <= 0 {
		poll = 30 * time.Second
	}
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		ra.mu.RLock()
		st := ra.status.State
		ra.mu.RUnlock()
		if st == adapter.Unreachable {
			// Retry connect with backoff.
			if err := ra.a.Connect(ctx, ra.adapterConfig()); err != nil {
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff = min(backoff*2, 2*time.Minute)
				continue
			}
			ra.mu.Lock()
			ra.status = adapter.Status{State: adapter.Healthy, Detail: "reconnected"}
			ra.mu.Unlock()
			backoff = time.Second
		}
		if ra.desc.Watchable {
			ch, err := ra.a.Watch(ctx)
			if err == nil {
				f.log.Info("adapter watching", "adapter", name)
				for ev := range ch {
					f.lat.ApplyEvent(ev)
					ra.mu.Lock()
					ra.lastOK = time.Now()
					ra.mu.Unlock()
				}
				if ctx.Err() != nil {
					return
				}
				f.log.Warn("adapter watch ended; reconnecting", "adapter", name)
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff = min(backoff*2, 2*time.Minute)
				continue
			}
			if !errors.Is(err, adapter.ErrWatchUnsupported) {
				f.log.Warn("adapter watch failed; polling", "adapter", name, "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(poll):
		}
		f.pollOne(ctx, name, ra)
	}
}

func (f *Fendlemark) pollOne(ctx context.Context, name string, ra *runtimeAdapter) {
	start := time.Now()
	facets, err := ra.a.Facets(ctx, adapter.Query{})
	ra.mu.Lock()
	if err != nil {
		ra.status = adapter.Status{State: adapter.Degraded, Detail: err.Error(), Latency: time.Since(start)}
		ra.mu.Unlock()
		f.log.Warn("adapter poll failed", "adapter", name, "err", err)
		return
	}
	ra.facets = len(facets)
	ra.lastOK = time.Now()
	ra.status = adapter.Status{State: adapter.Healthy, Detail: fmt.Sprintf("%d facets", len(facets)), Latency: time.Since(start)}
	ra.mu.Unlock()
	f.lat.Apply(facets)
}

// AdapterStatuses reports health for every constructed adapter.
func (f *Fendlemark) AdapterStatuses(ctx context.Context) []server.AdapterStatus {
	out := make([]server.AdapterStatus, 0, len(f.adapters))
	for _, name := range f.Adapters() {
		ra := f.adapters[name]
		ra.mu.RLock()
		st, n := ra.status, ra.facets
		ra.mu.RUnlock()
		if st.State == adapter.Healthy || st.State == adapter.Degraded {
			hctx, cancel := context.WithTimeout(ctx, ra.cfg.Timeout)
			live := ra.a.Health(hctx)
			cancel()
			if live.State != "" {
				st = live
				if st.State == adapter.Healthy && n > 0 {
					st.Detail = fmt.Sprintf("%s · %d facets", st.Detail, n)
				}
			}
		}
		out = append(out, server.AdapterStatus{Name: name, DisplayName: ra.desc.DisplayName, State: st.State, Detail: st.Detail,
			LatencyMS: st.Latency.Milliseconds(), Facets: n, Maturity: ra.desc.Maturity, Keys: ra.desc.Keys})
	}
	return out
}

// Run connects adapters, reconciles, starts the loops, and serves the console
// until ctx is done. It is what `fendlemark serve` calls.
func (f *Fendlemark) Run(ctx context.Context) error {
	f.log.Info("fendlemark starting", "version", Version, "adapters", strings.Join(f.Adapters(), ","))
	if err := f.Connect(ctx); err != nil {
		f.log.Warn("some adapters failed to connect", "err", err)
	}
	if _, err := f.Reconcile(ctx); err != nil {
		f.log.Warn("initial reconciliation had errors", "err", err)
	}
	f.Start(ctx)
	defer f.Close()
	f.log.Info("console listening", "addr", f.cfg.Server.Listen)
	return f.srv.ListenAndServe(ctx)
}

// Close stops the loops and releases adapter resources.
func (f *Fendlemark) Close() error {
	f.mu.Lock()
	cancel := f.cancel
	f.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	f.wg.Wait()
	var errs []error
	for _, ra := range f.adapters {
		if c, ok := ra.a.(adapter.Closer); ok {
			errs = append(errs, c.Close())
		}
	}
	return errors.Join(errs...)
}

// DiscoverConfig builds a configuration from the environment alone. An
// adapter is enabled when its primary variable is set:
//
//	MLFLOW_TRACKING_URI        -> mlflow
//	METAFLOW_SERVICE_URL       -> metaflow
//	ARGO_SERVER (+ARGO_NAMESPACE, comma-separated) -> argo
//	KUBEFLOW_PIPELINES_URL     -> kubeflow
//
// FENDLEMARK_* overrides apply on top, as they do to a file.
func DiscoverConfig() *config.Config {
	cfg := config.Default()
	enable := func(name string, opts map[string]any) {
		a := cfg.Adapters[name]
		a.Enabled = true
		for k, v := range opts {
			a.Options[k] = v
		}
		cfg.Adapters[name] = a
	}
	if v := os.Getenv("MLFLOW_TRACKING_URI"); v != "" {
		enable("mlflow", map[string]any{"tracking_uri": v})
	}
	if v := os.Getenv("METAFLOW_SERVICE_URL"); v != "" {
		enable("metaflow", map[string]any{"metadata_url": v, "ui_url": os.Getenv("METAFLOW_UI_URL")})
	}
	if v := os.Getenv("ARGO_SERVER"); v != "" {
		if !strings.Contains(v, "://") {
			v = "https://" + v
		}
		ns := os.Getenv("ARGO_NAMESPACE")
		if ns == "" {
			ns = "argo"
		}
		var list []any
		for _, n := range strings.Split(ns, ",") {
			if n = strings.TrimSpace(n); n != "" {
				list = append(list, n)
			}
		}
		enable("argo", map[string]any{"server": v, "namespaces": list})
	}
	if v := os.Getenv("KUBEFLOW_PIPELINES_URL"); v != "" {
		enable("kubeflow", map[string]any{"pipelines_url": v, "dashboard_url": os.Getenv("KUBEFLOW_DASHBOARD_URL")})
	}
	if v := os.Getenv("FENDLEMARK_SERVER__LISTEN"); v != "" {
		cfg.Server.Listen = v
	}
	return cfg
}

// IntegrateEasy discovers MLflow, Metaflow, Argo Workflows, and Kubeflow from
// the environment, enables every adapter whose variable is set, connects
// them concurrently, performs the initial reconciliation, starts the poll and
// watch loops, and returns a running instance. It does not serve HTTP; call
// Run, or IntegrateEasyAndServe, for that.
//
// The Keepers resisted adding it for two releases on the grounds that
// configuration files are a form of respect (FDR-035).
func IntegrateEasy(ctx context.Context, opts ...Option) (*Fendlemark, error) {
	cfg := DiscoverConfig()
	if len(cfg.Enabled()) == 0 {
		return nil, errors.New("fendlemark: nothing to integrate: set MLFLOW_TRACKING_URI, METAFLOW_SERVICE_URL, ARGO_SERVER, or KUBEFLOW_PIPELINES_URL")
	}
	f, err := New(cfg, opts...)
	if err != nil {
		return nil, err
	}
	if err := f.Connect(ctx); err != nil && allFailed(f) {
		return nil, fmt.Errorf("fendlemark: no adapter could connect: %w", err)
	}
	if _, err := f.Reconcile(ctx); err != nil {
		f.log.Warn("initial reconciliation had errors", "err", err)
	}
	f.Start(ctx)
	return f, nil
}

// IntegrateEasyAndServe is IntegrateEasy followed by serving the console on
// server.listen (default 127.0.0.1:7411) until ctx is done.
func IntegrateEasyAndServe(ctx context.Context, opts ...Option) error {
	f, err := IntegrateEasy(ctx, opts...)
	if err != nil {
		return err
	}
	defer f.Close()
	f.log.Info("console listening", "addr", f.cfg.Server.Listen)
	return f.srv.ListenAndServe(ctx)
}

func allFailed(f *Fendlemark) bool {
	for _, ra := range f.adapters {
		ra.mu.RLock()
		ok := ra.status.State != adapter.Unreachable
		ra.mu.RUnlock()
		if ok {
			return false
		}
	}
	return true
}
