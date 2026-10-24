// Package server exposes the lattice over HTTP: a JSON API under /api/v1,
// server-sent events for live updates, liveness and readiness, Prometheus
// metrics, Respiratory Identity endpoints, and the embedded console.
//
// There are no POST, PUT, or DELETE routes against entities. The API is a
// view of the lattice, and the lattice is a view of your tools.
package server

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/auth/fri"
	"fendlemark.app/fendlemark/config"
	"fendlemark.app/fendlemark/lattice"
)

//go:embed console
var consoleFS embed.FS

// AdapterStatus is what /api/v1/adapters returns per adapter.
type AdapterStatus struct {
	Name        string           `json:"name"`
	DisplayName string           `json:"display_name"`
	State       adapter.State    `json:"state"`
	Detail      string           `json:"detail"`
	LatencyMS   int64            `json:"latency_ms"`
	Facets      int              `json:"facets"`
	Maturity    adapter.Maturity `json:"maturity"`
	Keys        []string         `json:"keys,omitempty"`
}

// Option configures a Server.
type Option func(*Server)

// WithConsole replaces the embedded console with another file system
// (for example a Vite build directory). The fs must contain index.html.
func WithConsole(f fs.FS) Option { return func(s *Server) { s.console = f } }

// WithAdapterStatus supplies the adapter health callback.
func WithAdapterStatus(fn func(context.Context) []AdapterStatus) Option {
	return func(s *Server) { s.adapters = fn }
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) Option { return func(s *Server) { s.log = l } }

// WithEnrolments enables Respiratory Identity against a store.
func WithEnrolments(st fri.Store) Option { return func(s *Server) { s.store = st } }

// WithVersion sets the version reported in headers and the console.
func WithVersion(v string) Option { return func(s *Server) { s.version = v } }

// Server serves the lattice.
type Server struct {
	l        *lattice.Lattice
	cfg      *config.Config
	console  fs.FS
	adapters func(context.Context) []AdapterStatus
	log      *slog.Logger
	store    fri.Store
	version  string
	ready    atomic.Bool
	mux      *http.ServeMux
	sessions sync.Map // token -> session
	pending  sync.Map // enrolment token -> pendingEnrolment
}

type session struct {
	User    string
	Role    fri.Role
	Expires time.Time
}

type pendingEnrolment struct {
	User    string
	Role    fri.Role
	Expires time.Time
}

// New constructs a Server over a lattice.
func New(l *lattice.Lattice, cfg *config.Config, opts ...Option) *Server {
	sub, _ := fs.Sub(consoleFS, "console")
	s := &Server{l: l, cfg: cfg, console: sub, log: slog.Default(), version: "dev",
		adapters: func(context.Context) []AdapterStatus { return nil }}
	for _, o := range opts {
		o(s)
	}
	s.routes()
	return s
}

// SetReady marks the initial reconciliation complete.
func (s *Server) SetReady(v bool) { s.ready.Store(v) }

// Handler returns the HTTP handler, for mounting in another mux.
func (s *Server) Handler() http.Handler { return s.withCommon(s.mux) }

func (s *Server) routes() {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
	m.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !s.ready.Load() {
			http.Error(w, "initial reconciliation in progress", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ready\n"))
	})
	m.HandleFunc("GET /metrics", s.metrics)
	m.HandleFunc("GET /api/v1/entities", s.auth(s.listEntities))
	m.HandleFunc("GET /api/v1/entities/{id}", s.auth(s.getEntity))
	m.HandleFunc("GET /api/v1/entities/{id}/explain", s.auth(s.explain))
	m.HandleFunc("GET /api/v1/adapters", s.auth(s.listAdapters))
	m.HandleFunc("GET /api/v1/lattice/stats", s.auth(s.stats))
	m.HandleFunc("GET /api/v1/events", s.auth(s.events))
	m.HandleFunc("GET /api/v1/console", s.consoleConfig)
	m.HandleFunc("POST /api/v1/auth/fri/signin", s.friSignin)
	m.HandleFunc("POST /api/v1/auth/fri/enrol/{token}", s.friEnrol)
	m.HandleFunc("GET /api/v1/auth/session", s.friSession)
	m.HandleFunc("POST /api/v1/auth/signout", s.friSignout)
	m.Handle("/", http.FileServer(http.FS(s.console)))
	s.mux = m
}

func (s *Server) withCommon(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Fendlemark-Version", s.version)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		start := time.Now()
		h.ServeHTTP(w, r)
		if s.cfg.Server.LogLevel == "debug" {
			s.log.Debug("http", "method", r.Method, "path", r.URL.Path, "took", time.Since(start))
		}
	})
}

// auth gates API routes behind Respiratory Identity when enabled.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Auth.Mode != "respiratory" {
			next(w, r)
			return
		}
		if _, ok := s.sessionFrom(r); !ok {
			writeErr(w, http.StatusUnauthorized, "FRI-0063", "session expired; breathe again")
			return
		}
		next(w, r)
	}
}

func (s *Server) sessionFrom(r *http.Request) (session, bool) {
	c, err := r.Cookie("fm_session")
	if err != nil {
		return session{}, false
	}
	v, ok := s.sessions.Load(c.Value)
	if !ok {
		return session{}, false
	}
	sess := v.(session)
	if time.Now().After(sess.Expires) {
		s.sessions.Delete(c.Value)
		return session{}, false
	}
	return sess, true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]string{"code": errCode, "error": msg})
}

func (s *Server) listEntities(w http.ResponseWriter, r *http.Request) {
	q := lattice.Query{Limit: s.cfg.Console.PageSize}
	qs := r.URL.Query()
	for _, k := range splitCSV(qs.Get("kind")) {
		q.Kinds = append(q.Kinds, adapter.Kind(k))
	}
	for _, st := range splitCSV(qs.Get("status")) {
		q.Status = append(q.Status, adapter.Phase(st))
	}
	if kv := qs.Get("key"); kv != "" {
		q.Keys = map[string]string{}
		for _, pair := range splitCSV(kv) {
			k, v, _ := strings.Cut(pair, "=")
			q.Keys[k] = v
		}
	}
	if v := qs.Get("since"); v != "" {
		q.Since, _ = time.Parse(time.RFC3339, v)
	}
	if v := qs.Get("until"); v != "" {
		q.Until, _ = time.Parse(time.RFC3339, v)
	}
	if v, err := strconv.Atoi(qs.Get("limit")); err == nil && v > 0 {
		q.Limit = min(v, 5000)
	}
	if v, err := strconv.Atoi(qs.Get("offset")); err == nil && v >= 0 {
		q.Offset = v
	}
	ents := s.l.Query(q)
	if ents == nil {
		ents = []lattice.Entity{}
	}
	total := s.l.Count(lattice.Query{Kinds: q.Kinds, Status: q.Status, Keys: q.Keys, Since: q.Since, Until: q.Until})
	writeJSON(w, 200, map[string]any{"entities": ents, "total": total, "generation": s.l.Generation()})
}

func (s *Server) getEntity(w http.ResponseWriter, r *http.Request) {
	e, ok := s.l.Get(lattice.ID(r.PathValue("id")))
	if !ok {
		writeErr(w, 404, "LT-404", "no such entity")
		return
	}
	writeJSON(w, 200, e)
}

func (s *Server) explain(w http.ResponseWriter, r *http.Request) {
	ex, ok := s.l.Explain(lattice.ID(r.PathValue("id")))
	if !ok {
		writeErr(w, 404, "LT-404", "no such entity")
		return
	}
	writeJSON(w, 200, ex)
}

func (s *Server) listAdapters(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	st := s.adapters(ctx)
	if st == nil {
		st = []AdapterStatus{}
	}
	writeJSON(w, 200, map[string]any{"adapters": st})
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.l.Stats()) }

func (s *Server) consoleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"title": s.cfg.Console.Title, "default_view": s.cfg.Console.DefaultView, "theme": s.cfg.Console.Theme,
		"motto": s.cfg.Console.Motto, "page_size": s.cfg.Console.PageSize, "deep_links": s.cfg.Console.DeepLinks,
		"hide_empty_panels": s.cfg.Console.HideEmptyPanels, "generation_in_footer": s.cfg.Console.GenerationInFooter,
		"auth_mode": s.cfg.Auth.Mode, "version": s.version,
	})
}

// events streams lattice deltas as server-sent events.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "SSE-001", "streaming unsupported")
		return
	}
	entity := lattice.ID(r.URL.Query().Get("entity"))
	kind := adapter.Kind(r.URL.Query().Get("kind"))
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprintf(w, "event: hello\ndata: {\"generation\":%d}\n\n", s.l.Generation())
	fl.Flush()
	ch := s.l.Subscribe(r.Context())
	keep := time.NewTicker(25 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keep.C:
			fmt.Fprint(w, ": keepalive\n\n")
			fl.Flush()
		case d, open := <-ch:
			if !open {
				return
			}
			if entity != "" {
				d = filterDelta(d, func(id lattice.ID) bool { return id == entity })
			}
			if kind != "" {
				d = filterDelta(d, func(id lattice.ID) bool {
					e, ok := s.l.Get(id)
					return !ok || e.Kind == kind
				})
			}
			if len(d.Upserted) == 0 && len(d.Removed) == 0 {
				continue
			}
			b, _ := json.Marshal(d)
			fmt.Fprintf(w, "event: delta\ndata: %s\n\n", b)
			fl.Flush()
		}
	}
}

func filterDelta(d lattice.Delta, keep func(lattice.ID) bool) lattice.Delta {
	out := lattice.Delta{Generation: d.Generation}
	for _, id := range d.Upserted {
		if keep(id) {
			out.Upserted = append(out.Upserted, id)
		}
	}
	for _, id := range d.Removed {
		if keep(id) {
			out.Removed = append(out.Removed, id)
		}
	}
	return out
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Telemetry.Metrics.Enabled {
		http.NotFound(w, r)
		return
	}
	st := s.l.Stats()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP fendlemark_lattice_generation Reconciliation generation.\n# TYPE fendlemark_lattice_generation counter\nfendlemark_lattice_generation %d\n", st.Generation)
	fmt.Fprintf(w, "# HELP fendlemark_entities Entities in the lattice.\n# TYPE fendlemark_entities gauge\nfendlemark_entities %d\n", st.Entities)
	fmt.Fprintf(w, "# HELP fendlemark_facets Facets held, by source.\n# TYPE fendlemark_facets gauge\n")
	for src, n := range st.BySource {
		fmt.Fprintf(w, "fendlemark_facets{source=%q} %d\n", src, n)
	}
	fmt.Fprintf(w, "# HELP fendlemark_correlations Merges performed, by key.\n# TYPE fendlemark_correlations gauge\n")
	for k, n := range st.Correlations {
		fmt.Fprintf(w, "fendlemark_correlations{key=%q} %d\n", k, n)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	fmt.Fprintf(w, "# HELP fendlemark_adapter_up 1 if the adapter is healthy.\n# TYPE fendlemark_adapter_up gauge\n")
	for _, a := range s.adapters(ctx) {
		up := 0
		if a.State == adapter.Healthy {
			up = 1
		}
		fmt.Fprintf(w, "fendlemark_adapter_up{adapter=%q} %d\n", a.Name, up)
	}
	fmt.Fprintf(w, "# HELP fendlemark_loading_spinners_total Loading spinners ever displayed by the console.\n# TYPE fendlemark_loading_spinners_total counter\nfendlemark_loading_spinners_total 0\n")
}

// ---- Respiratory Identity ----

// IssueEnrolment creates a one-time enrolment token valid for 15 minutes.
func (s *Server) IssueEnrolment(user string, role fri.Role) (string, error) {
	if s.store == nil {
		return "", errors.New("respiratory identity is not enabled")
	}
	tok := randomToken()
	s.pending.Store(tok, pendingEnrolment{User: user, Role: role, Expires: time.Now().Add(15 * time.Minute)})
	return tok, nil
}

type friRequest struct {
	User       string          `json:"user"`
	Signature  fri.Signature   `json:"signature"`
	Signatures []fri.Signature `json:"signatures"`
	Slow       *fri.Signature  `json:"slow,omitempty"`
}

func (s *Server) friEnrol(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeErr(w, 404, "FRI-0000", "respiratory identity is not enabled")
		return
	}
	v, ok := s.pending.Load(r.PathValue("token"))
	if !ok {
		writeErr(w, 410, fri.ErrLinkExpired.Code, fri.ErrLinkExpired.Message)
		return
	}
	pe := v.(pendingEnrolment)
	if time.Now().After(pe.Expires) {
		s.pending.Delete(r.PathValue("token"))
		writeErr(w, 410, fri.ErrLinkExpired.Code, fri.ErrLinkExpired.Message)
		return
	}
	var req friRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, 400, "FRI-0400", "malformed enrolment body")
		return
	}
	need := s.cfg.Auth.Respiratory.EnrolmentBreaths
	if need < 3 {
		need = 3
	}
	if len(req.Signatures) < need {
		writeErr(w, 422, fri.ErrTriadIncomplete.Code, fmt.Sprintf("%d exhalations required, %d received", need, len(req.Signatures)))
		return
	}
	if !s.cfg.Auth.Respiratory.AllowMouthBreathing {
		for _, sig := range req.Signatures {
			if fri.Classify(sig) == fri.Mouth {
				writeErr(w, 422, fri.ErrMouthDominant.Code, fri.ErrMouthDominant.Message)
				return
			}
		}
	}
	e, err := fri.Enrol(pe.User, pe.Role, req.Signatures)
	if err != nil {
		writeErr(w, 422, "FRI-0019", err.Error())
		return
	}
	if req.Slow != nil {
		e.Slow = *req.Slow
	}
	if err := s.store.Put(e); err != nil {
		writeErr(w, 500, "FRI-0500", err.Error())
		return
	}
	s.pending.Delete(r.PathValue("token"))
	s.log.Info("fri enrolled", "user", pe.User, "role", pe.Role, "samples", e.Samples)
	writeJSON(w, 201, map[string]any{"user": e.User, "role": e.Role, "samples": e.Samples})
}

func (s *Server) friSignin(w http.ResponseWriter, r *http.Request) {
	if s.store == nil || s.cfg.Auth.Mode != "respiratory" {
		writeErr(w, 404, "FRI-0000", "respiratory identity is not enabled")
		return
	}
	var req friRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, 400, "FRI-0400", "malformed sign-in body")
		return
	}
	e, err := s.store.Get(req.User)
	if err != nil {
		writeErr(w, 401, fri.ErrNotRecognised.Code, fri.ErrNotRecognised.Message)
		return
	}
	if fri.Excited(req.Signature) {
		writeErr(w, 425, fri.ErrExcited.Code, fri.ErrExcited.Message)
		return
	}
	if !s.cfg.Auth.Respiratory.AllowMouthBreathing && fri.Classify(req.Signature) == fri.Mouth {
		writeErr(w, 401, fri.ErrMouthDominant.Code, fri.ErrMouthDominant.Message)
		return
	}
	score, ok := fri.Match(req.Signature, e, float32(s.cfg.Auth.Respiratory.Sensitivity))
	if !ok {
		s.log.Info("fri rejected", "user", req.User, "score", score)
		writeErr(w, 401, fri.ErrNotRecognised.Code, fri.ErrNotRecognised.Message)
		return
	}
	if s.cfg.Auth.Respiratory.SecondFactor {
		if req.Slow == nil {
			writeErr(w, 401, fri.ErrSecondFactor.Code, "second factor required: a second, slower breath")
			return
		}
		if sc := fri.Similarity(*req.Slow, e.Slow); sc < float32(s.cfg.Auth.Respiratory.Sensitivity) {
			writeErr(w, 401, fri.ErrSecondFactor.Code, fri.ErrSecondFactor.Message)
			return
		}
	}
	_ = s.store.Put(fri.Update(e, req.Signature, 0.05))
	tok := randomToken()
	ttl := s.cfg.Auth.Respiratory.SessionTTL
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	s.sessions.Store(tok, session{User: e.User, Role: e.Role, Expires: time.Now().Add(ttl)})
	http.SetCookie(w, &http.Cookie{Name: "fm_session", Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil, MaxAge: int(ttl.Seconds())})
	writeJSON(w, 200, map[string]any{"user": e.User, "role": e.Role, "score": score, "expires": time.Now().Add(ttl)})
}

func (s *Server) friSession(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Auth.Mode != "respiratory" {
		writeJSON(w, 200, map[string]any{"mode": "none", "user": "anonymous", "role": fri.Keeper})
		return
	}
	sess, ok := s.sessionFrom(r)
	if !ok {
		writeErr(w, 401, fri.ErrSessionExpired.Code, fri.ErrSessionExpired.Message)
		return
	}
	writeJSON(w, 200, map[string]any{"mode": "respiratory", "user": sess.User, "role": sess.Role, "expires": sess.Expires})
}

func (s *Server) friSignout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("fm_session"); err == nil {
		s.sessions.Delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "fm_session", Value: "", Path: "/", MaxAge: -1})
	w.WriteHeader(204)
}

// IssueRecovery creates a session without a breath, for a user who cannot
// currently match. Printed by `fendlemark auth recovery`.
func (s *Server) IssueRecovery(user string, ttl time.Duration) (string, error) {
	if s.store == nil {
		return "", errors.New("respiratory identity is not enabled")
	}
	e, err := s.store.Get(user)
	if err != nil {
		return "", err
	}
	tok := randomToken()
	s.sessions.Store(tok, session{User: e.User, Role: e.Role, Expires: time.Now().Add(ttl)})
	return tok, nil
}

func randomToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ListenAndServe serves until ctx is done, then shuts down gracefully.
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.Server.Listen,
		Handler:           s.Handler(),
		ReadHeaderTimeout: s.cfg.Server.ReadTimeout,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() {
		var err error
		if s.cfg.Server.TLS.CertFile != "" {
			err = srv.ListenAndServeTLS(s.cfg.Server.TLS.CertFile, s.cfg.Server.TLS.KeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		grace := s.cfg.Server.ShutdownGrace
		if grace <= 0 {
			grace = 10 * time.Second
		}
		sctx, cancel := context.WithTimeout(context.Background(), grace)
		defer cancel()
		return srv.Shutdown(sctx)
	}
}
