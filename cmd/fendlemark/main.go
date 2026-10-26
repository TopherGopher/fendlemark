// Command fendlemark is the whole Fendlemark app: the server, with the
// Fendlemark console embedded, and the operator's tool.
//
//	fendlemark init [dir] [--adapters mlflow,argo] [--force]
//	fendlemark serve [--listen 127.0.0.1:7411] [--adapters ...] [--no-reconcile-on-start]
//	fendlemark adapters list|check|describe
//	fendlemark lattice inspect|explain|stats
//	fendlemark auth enrol|list|revoke|recovery
//	fendlemark doctor [--fix] [--json]
//	fendlemark version [--json]
//
// See https://fendlemark.app/docs/cli/ for every command, flag, and exit code.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"fendlemark.app/fendlemark"
	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/auth/fri"
	"fendlemark.app/fendlemark/config"
	"fendlemark.app/fendlemark/lattice"
)

const (
	exitOK     = 0
	exitUsage  = 1
	exitDegrad = 2
	exitUnreac = 3
	exitAuth   = 4
)

// exposedWarning follows the listen address in DOC-121.
const exposedWarning = "accepts connections from other machines; respiratory identity is not access control, so put an authenticating proxy in front"

var (
	configPath = "./fendlemark.yaml"
	logLevel   = ""
	logFormat  = "text"
	noColor    = false
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	global := globalFlags()
	// Global flags come before the command, and may follow it as well.
	if err := global.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	cmd, rest := global.Arg(0), global.Args()
	if len(rest) > 0 {
		rest = rest[1:]
	}
	rest, err := takeGlobalFlags(global, rest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fendlemark:", err)
		return exitUsage
	}
	switch cmd {
	case "init":
		return cmdInit(rest)
	case "serve":
		return cmdServe(rest)
	case "adapters":
		return cmdAdapters(rest)
	case "lattice":
		return cmdLattice(rest)
	case "auth":
		return cmdAuth(rest)
	case "doctor":
		return cmdDoctor(rest)
	case "version":
		return cmdVersion(rest)
	case "", "help":
		usage()
		return exitOK
	}
	fmt.Fprintf(os.Stderr, "fendlemark: unknown command %q\n\n", cmd)
	usage()
	return exitUsage
}

// globalFlags defines the global flags, resetting each to its default.
func globalFlags() *flag.FlagSet {
	global := flag.NewFlagSet("fendlemark", flag.ContinueOnError)
	global.SetOutput(os.Stderr)
	global.StringVar(&configPath, "config", envOr("FENDLEMARK_CONFIG", "./fendlemark.yaml"), "path to fendlemark.yaml")
	global.StringVar(&configPath, "c", envOr("FENDLEMARK_CONFIG", "./fendlemark.yaml"), "path to fendlemark.yaml (shorthand)")
	global.StringVar(&logLevel, "log-level", "", "debug, info, warn, error, serene")
	global.StringVar(&logFormat, "log-format", "text", "text or json")
	// Nothing the CLI prints is coloured, so there is nothing for this to
	// turn off; it is accepted so that scripts which pass it keep working.
	global.BoolVar(&noColor, "no-color", os.Getenv("NO_COLOR") != "", "disable ANSI colour (also NO_COLOR)")
	global.Usage = usage
	return global
}

// takeGlobalFlags applies the global flags found among a command's arguments
// and returns the arguments without them. No command defines a flag with a
// global flag's name, so there is nothing to confuse them with. Arguments
// after "--" are left alone.
func takeGlobalFlags(global *flag.FlagSet, args []string) ([]string, error) {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return append(out, args[i:]...), nil
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		f := global.Lookup(name)
		if !strings.HasPrefix(a, "-") || f == nil {
			out = append(out, a)
			continue
		}
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() && !hasValue {
			value = "true"
		} else if !hasValue {
			if i+1 == len(args) {
				return nil, fmt.Errorf("flag needs an argument: %s", a)
			}
			i++
			value = args[i]
		}
		if err := global.Set(name, value); err != nil {
			return nil, fmt.Errorf("invalid value %q for %s: %w", value, a, err)
		}
	}
	return out, nil
}

func usage() {
	fmt.Fprint(os.Stderr, `fendlemark `+fendlemark.Version+` · one state-of-the-art console for your entire ML platform

Usage:
  fendlemark <command> [flags]

Commands:
  init       write a minimal fendlemark.yaml
  serve      run the control plane and console
  adapters   list, check, or describe adapters
  lattice    inspect, explain, or summarise the lattice without serving
  auth       manage Respiratory Identity enrolments
  doctor     validate configuration and connectivity
  version    print version information

Global flags:
  -c, --config   path to fendlemark.yaml (default ./fendlemark.yaml)
  --log-level    debug, info, warn, error, serene
  --log-format   text or json
  --no-color     disable ANSI colour (also NO_COLOR)

Documentation: https://fendlemark.app/docs/cli/
`)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func logger(cfg *config.Config) *slog.Logger {
	lvl := logLevel
	if lvl == "" && cfg != nil {
		lvl = cfg.Server.LogLevel
	}
	var level slog.Level
	switch lvl {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error", "serene":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if logFormat == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	if lvl == "serene" {
		h = sereneHandler{h}
	}
	return slog.New(h)
}

// sereneHandler prefixes every surviving record with a reassurance.
type sereneHandler struct{ slog.Handler }

func (s sereneHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Message = "All is well. " + r.Message
	return s.Handler.Handle(ctx, r)
}
func (s sereneHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return sereneHandler{s.Handler.WithAttrs(a)}
}
func (s sereneHandler) WithGroup(n string) slog.Handler { return sereneHandler{s.Handler.WithGroup(n)} }

func loadConfig() (*config.Config, []config.Finding, error) {
	path := configPath
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if path != "./fendlemark.yaml" {
			return nil, nil, fmt.Errorf("config: %s does not exist", path)
		}
		path = ""
	}
	cfg, err := config.Load(path)
	return cfg, cfg.Findings(), err
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// ---- init ----

func cmdInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	adapters := fs.String("adapters", "", "comma-separated adapters to enable")
	force := fs.Bool("force", false, "overwrite an existing fendlemark.yaml")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return exitUsage
	}
	path := filepath.Join(dir, "fendlemark.yaml")
	if _, err := os.Stat(path); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "init: %s exists; pass --force to overwrite\n", path)
		return exitUsage
	}
	var enabled []string
	for _, a := range strings.Split(*adapters, ",") {
		if a = strings.TrimSpace(a); a != "" {
			enabled = append(enabled, a)
		}
	}
	if err := os.WriteFile(path, []byte(config.Minimal(enabled)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return exitUsage
	}
	readme := filepath.Join(dir, "README.md")
	if _, err := os.Stat(readme); errors.Is(err, os.ErrNotExist) {
		_ = os.WriteFile(readme, []byte("# Fendlemark\n\nEdit `fendlemark.yaml`, then:\n\n    fendlemark adapters check\n    fendlemark serve\n\nDocumentation: https://fendlemark.app/docs/getting-started/\n"), 0o644)
	}
	fmt.Printf("wrote %s\n", path)
	return exitOK
}

// ---- serve ----

func cmdServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := fs.String("listen", "", "override server.listen")
	adapters := fs.String("adapters", "", "comma-separated adapters to enable for this run")
	noReconcile := fs.Bool("no-reconcile-on-start", false, "serve before the initial reconciliation completes")
	consoleDev := fs.Bool("console-dev", false, "proxy the console to a Vite dev server on :5173")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	cfg, findings, err := loadConfig()
	log := logger(cfg)
	// The notice comes first, even when the configuration is wrong.
	if noticesOn(cfg) {
		showNotice(log, noticeURL)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "serve:", err)
		return exitUsage
	}
	for _, f := range findings {
		if f.Level == "warn" {
			log.Warn(f.Message, "code", f.Code)
		}
	}
	if *listen != "" {
		cfg.Server.Listen = *listen
	}
	if config.Exposed(cfg.Server.Listen) {
		log.Warn(cfg.Server.Listen+" "+exposedWarning, "code", "DOC-121")
	}
	if *consoleDev {
		log.Warn("--console-dev is accepted for compatibility; this build serves the embedded console")
	}
	var opts []fendlemark.Option
	opts = append(opts, fendlemark.WithLogger(log))
	if *adapters != "" {
		opts = append(opts, fendlemark.WithAdapters(strings.Split(*adapters, ",")...))
	}
	fm, err := fendlemark.New(cfg, opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "serve:", err)
		return exitUsage
	}
	ctx, cancel := signalContext()
	defer cancel()
	if *noReconcile {
		log.Info("serving before initial reconciliation, as requested, and against advice")
		go func() {
			if err := fm.Connect(ctx); err != nil {
				log.Warn("some adapters failed to connect", "err", err)
			}
			_, _ = fm.Reconcile(ctx)
			fm.Start(ctx)
		}()
		defer fm.Close()
		return exitOn(fm.Server().ListenAndServe(ctx))
	}
	return exitOn(fm.Run(ctx))
}

func exitOn(err error) int {
	if err == nil {
		return exitOK
	}
	fmt.Fprintln(os.Stderr, "fendlemark:", err)
	return exitUsage
}

// ---- adapters ----

func cmdAdapters(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: fendlemark adapters list|check|describe")
		return exitUsage
	}
	switch args[0] {
	case "list":
		cfg, _, _ := loadConfig()
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 3, ' ', 0)
		fmt.Fprintln(tw, "ADAPTER\tMATURITY\tENABLED\tKINDS")
		for _, name := range adapter.Registered() {
			a, _ := adapter.Lookup(name)
			d := a.Describe()
			enabled := false
			if cfg != nil {
				enabled = cfg.Adapters[name].Enabled
			}
			kinds := make([]string, 0, len(d.Kinds))
			for _, k := range d.Kinds {
				kinds = append(kinds, string(k))
			}
			fmt.Fprintf(tw, "%s\t%s\t%t\t%s\n", name, d.Maturity, enabled, strings.Join(kinds, ","))
		}
		tw.Flush()
		return exitOK
	case "describe":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: fendlemark adapters describe <name>")
			return exitUsage
		}
		a, ok := adapter.Lookup(args[1])
		if !ok {
			fmt.Fprintf(os.Stderr, "no adapter %q; compiled in: %s\n", args[1], strings.Join(adapter.Registered(), ", "))
			return exitUsage
		}
		b, _ := json.MarshalIndent(a.Describe(), "", "  ")
		fmt.Println(string(b))
		return exitOK
	case "check":
		fs := flag.NewFlagSet("check", flag.ContinueOnError)
		timeout := fs.Duration("timeout", 10*time.Second, "per-adapter timeout")
		asJSON := fs.Bool("json", false, "JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return exitUsage
		}
		cfg, _, err := loadConfig()
		if err != nil {
			fmt.Fprintln(os.Stderr, "check:", err)
			return exitUsage
		}
		var opts []fendlemark.Option
		opts = append(opts, fendlemark.WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))))
		if fs.NArg() > 0 {
			opts = append(opts, fendlemark.WithAdapters(fs.Args()...))
		}
		fm, err := fendlemark.New(cfg, opts...)
		if err != nil {
			fmt.Fprintln(os.Stderr, "check:", err)
			return exitUsage
		}
		ctx, cancel := context.WithTimeout(context.Background(), *timeout+5*time.Second)
		defer cancel()
		_ = fm.Connect(ctx)
		_, _ = fm.Reconcile(ctx)
		st := fm.AdapterStatuses(ctx)
		code := exitOK
		if *asJSON {
			b, _ := json.MarshalIndent(st, "", "  ")
			fmt.Println(string(b))
		} else {
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 3, ' ', 0)
			fmt.Fprintln(tw, "ADAPTER\tSTATUS\tLATENCY\tDETAIL")
			for _, s := range st {
				fmt.Fprintf(tw, "%s\t%s\t%dms\t%s\n", s.Name, s.State, s.LatencyMS, s.Detail)
			}
			for _, name := range config.KnownAdapters {
				if !cfg.Adapters[name].Enabled {
					fmt.Fprintf(tw, "%s\tdisabled\t\t\n", name)
				}
			}
			tw.Flush()
		}
		for _, s := range st {
			switch s.State {
			case adapter.Degraded:
				if code < exitDegrad {
					code = exitDegrad
				}
			case adapter.Unreachable:
				code = exitUnreac
			}
		}
		return code
	}
	fmt.Fprintln(os.Stderr, "usage: fendlemark adapters list|check|describe")
	return exitUsage
}

// ---- lattice ----

func cmdLattice(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: fendlemark lattice inspect|explain|stats")
		return exitUsage
	}
	fs := flag.NewFlagSet("lattice", flag.ContinueOnError)
	kind := fs.String("kind", "", "filter by kind")
	key := fs.String("key", "", "filter by key=value")
	limit := fs.Int("limit", 50, "max entities")
	asJSON := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args[1:]); err != nil {
		return exitUsage
	}
	cfg, _, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "lattice:", err)
		return exitUsage
	}
	fm, err := fendlemark.New(cfg, fendlemark.WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))))
	if err != nil {
		fmt.Fprintln(os.Stderr, "lattice:", err)
		return exitUsage
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = fm.Connect(ctx)
	if _, err := fm.Reconcile(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "lattice: some adapters failed:", err)
	}
	l := fm.Lattice()
	switch args[0] {
	case "stats":
		st := l.Stats()
		if *asJSON {
			b, _ := json.MarshalIndent(st, "", "  ")
			fmt.Println(string(b))
			return exitOK
		}
		fmt.Printf("generation  %d\nentities    %d\nfacets      %d\n", st.Generation, st.Entities, st.Facets)
		printMap("by kind", st.ByKind)
		printMap("by source", st.BySource)
		printMap("correlations", st.Correlations)
		return exitOK
	case "explain":
		if fs.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "usage: fendlemark lattice explain <entity-id>")
			return exitUsage
		}
		ex, ok := l.Explain(lattice.ID(fs.Arg(0)))
		if !ok {
			fmt.Fprintln(os.Stderr, "no such entity")
			return exitUsage
		}
		if *asJSON {
			b, _ := json.MarshalIndent(ex, "", "  ")
			fmt.Println(string(b))
			return exitOK
		}
		fmt.Printf("%s  %s  %s  %q\n", ex.Entity.ID, ex.Entity.Kind, ex.Entity.Status, ex.Entity.Title)
		for _, f := range ex.Entity.Facets {
			fmt.Printf("  facet  %-10s %s\n", f.Source, f.SourceID)
		}
		if len(ex.Merges) == 0 {
			fmt.Println("  no merges: a single facet describes this entity")
		}
		for _, m := range ex.Merges {
			fmt.Printf("  merge  %s = %q linked %s and %s (gap %s)\n", m.Key, m.Value, short(m.A), short(m.B), m.Gap.Round(time.Millisecond))
		}
		for _, r := range ex.Relations {
			fmt.Printf("  edge   %s -> %s via %s\n", r.Rel, r.To, r.Key)
		}
		return exitOK
	case "inspect":
		q := lattice.Query{Limit: *limit}
		if *kind != "" {
			q.Kinds = []adapter.Kind{adapter.Kind(*kind)}
		}
		if *key != "" {
			k, v, _ := strings.Cut(*key, "=")
			q.Keys = map[string]string{k: v}
		}
		ents := l.Query(q)
		if *asJSON {
			b, _ := json.MarshalIndent(ents, "", "  ")
			fmt.Println(string(b))
			return exitOK
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tKIND\tSTATUS\tFACETS\tSTARTED\tTITLE")
		for _, e := range ents {
			srcs := map[string]bool{}
			for _, f := range e.Facets {
				srcs[f.Source] = true
			}
			names := make([]string, 0, len(srcs))
			for s := range srcs {
				names = append(names, s)
			}
			sort.Strings(names)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", e.ID, e.Kind, e.Status, strings.Join(names, "+"), e.StartedAt.Format(time.RFC3339), e.Title)
		}
		tw.Flush()
		return exitOK
	}
	fmt.Fprintln(os.Stderr, "usage: fendlemark lattice inspect|explain|stats")
	return exitUsage
}

func short(ref string) string {
	src, id, _ := strings.Cut(ref, "\x00")
	return src + "/" + id
}

func printMap(title string, m map[string]int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println(title)
	for _, k := range keys {
		fmt.Printf("  %-28s %d\n", k, m[k])
	}
}

// ---- auth ----

func cmdAuth(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: fendlemark auth enrol|list|revoke|recovery")
		return exitUsage
	}
	fs := flag.NewFlagSet("auth", flag.ContinueOnError)
	user := fs.String("user", "", "user (email)")
	role := fs.String("role", "viewer", "viewer, operator, keeper")
	ttl := fs.Duration("ttl", 24*time.Hour, "recovery session lifetime")
	if err := fs.Parse(args[1:]); err != nil {
		return exitUsage
	}
	cfg, _, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "auth:", err)
		return exitAuth
	}
	if cfg.Auth.Mode != "respiratory" {
		fmt.Fprintln(os.Stderr, "auth: auth.mode is none; set it to respiratory (there are no other modes)")
		return exitAuth
	}
	store, err := fri.NewFileStore(cfg.Server.DataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "auth:", err)
		return exitAuth
	}
	base := cfg.Server.BaseURL
	if base == "" {
		host, port, _ := net.SplitHostPort(cfg.Server.Listen)
		if host == "" {
			host = "localhost"
		}
		base = "http://" + net.JoinHostPort(host, port)
	}
	switch args[0] {
	case "list":
		list, _ := store.List()
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 3, ' ', 0)
		fmt.Fprintln(tw, "USER\tROLE\tSAMPLES\tUPDATED")
		for _, e := range list {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", e.User, e.Role, e.Samples, e.UpdatedAt.Format(time.RFC3339))
		}
		tw.Flush()
		return exitOK
	case "revoke":
		if *user == "" {
			fmt.Fprintln(os.Stderr, "auth revoke: --user is required")
			return exitUsage
		}
		if err := store.Delete(*user); err != nil {
			fmt.Fprintln(os.Stderr, "auth:", err)
			return exitAuth
		}
		fmt.Printf("revoked %s\n", *user)
		return exitOK
	case "enrol", "enroll":
		if *user == "" {
			fmt.Fprintln(os.Stderr, "auth enrol: --user is required")
			return exitUsage
		}
		// Enrolment tokens live in the running server's memory. The CLI writes a
		// pending record the server picks up on its next request to the data dir.
		tok, err := writePending(cfg.Server.DataDir, *user, fri.Role(*role))
		if err != nil {
			fmt.Fprintln(os.Stderr, "auth:", err)
			return exitAuth
		}
		fmt.Printf("Enrolment link (valid 15 minutes):\n  %s/auth/enrol/%s\n\nAsk %s to open it and perform the Triad: three gentle exhalations toward the camera.\nBreathe normally. Do not perform.\n", base, tok, *user)
		return exitOK
	case "recovery":
		if *user == "" {
			fmt.Fprintln(os.Stderr, "auth recovery: --user is required")
			return exitUsage
		}
		if _, err := store.Get(*user); err != nil {
			fmt.Fprintln(os.Stderr, "auth:", err)
			return exitAuth
		}
		tok, err := writePending(cfg.Server.DataDir, "recovery:"+*user, fri.Role(*role))
		if err != nil {
			fmt.Fprintln(os.Stderr, "auth:", err)
			return exitAuth
		}
		fmt.Printf("Recovery link (valid %s, no breath required):\n  %s/auth/recover/%s\n", *ttl, base, tok)
		return exitOK
	}
	fmt.Fprintln(os.Stderr, "usage: fendlemark auth enrol|list|revoke|recovery")
	return exitUsage
}

func writePending(dataDir, user string, role fri.Role) (string, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "pending"), 0o700); err != nil {
		return "", err
	}
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (uint(i) % 8 * 8))
	}
	tok := fmt.Sprintf("%x%x", time.Now().UnixNano(), b[:4])
	rec, _ := json.Marshal(map[string]any{"user": user, "role": role, "expires": time.Now().Add(15 * time.Minute)})
	return tok, os.WriteFile(filepath.Join(dataDir, "pending", tok+".json"), rec, 0o600)
}

// ---- doctor ----

func cmdDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fix := fs.Bool("fix", false, "apply mechanical fixes and rewrite the file with a backup")
	asJSON := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	type row struct {
		Area, Level, Code, Message string
	}
	var rows []row
	add := func(area, level, code, msg string) { rows = append(rows, row{area, level, code, msg}) }
	fmt.Printf("fendlemark doctor %s\n\n", fendlemark.Version)
	cfg, findings, err := loadConfig()
	if err != nil {
		add("config", "error", "DOC-100", err.Error())
	}
	set := 0
	if n, err := config.SetCount(configPath); err == nil {
		set = n
	}
	if cfg != nil {
		add("config", "ok", "", fmt.Sprintf("%s (%d keys set)", configPath, set))
	}
	fixable := 0
	for _, f := range findings {
		add("config", f.Level, f.Code, f.Message+map[bool]string{true: " (fixable)", false: ""}[f.Fixable])
		if f.Fixable {
			fixable++
		}
	}
	code := exitOK
	if cfg != nil {
		if ln, err := net.Listen("tcp", cfg.Server.Listen); err != nil {
			add("listen", "warn", "DOC-120", fmt.Sprintf("%s is not available: %v", cfg.Server.Listen, err))
		} else {
			ln.Close()
			add("listen", "ok", "", cfg.Server.Listen+" is available")
		}
		if config.Exposed(cfg.Server.Listen) {
			add("listen", "warn", "DOC-121", cfg.Server.Listen+" "+exposedWarning)
		}
		if err := os.MkdirAll(cfg.Server.DataDir, 0o700); err != nil {
			add("data_dir", "error", "DOC-130", err.Error())
		} else if f, err := os.CreateTemp(cfg.Server.DataDir, ".probe-*"); err != nil {
			add("data_dir", "error", "DOC-130", "not writable: "+err.Error())
		} else {
			f.Close()
			os.Remove(f.Name())
			add("data_dir", "ok", "", cfg.Server.DataDir+" (writable)")
		}
		if err == nil {
			fm, nerr := fendlemark.New(cfg, fendlemark.WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))))
			if nerr != nil {
				add("adapters", "error", "DOC-110", nerr.Error())
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				_ = fm.Connect(ctx)
				_, _ = fm.Reconcile(ctx)
				for _, s := range fm.AdapterStatuses(ctx) {
					lvl := "ok"
					switch s.State {
					case adapter.Degraded:
						lvl = "warn"
						if code < exitDegrad {
							code = exitDegrad
						}
					case adapter.Unreachable:
						lvl = "error"
						code = exitUnreac
					}
					add(s.Name, lvl, "", fmt.Sprintf("%dms  %s", s.LatencyMS, s.Detail))
				}
				cancel()
				fm.Close()
			}
		}
	}
	if *asJSON {
		b, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(b))
	} else {
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 3, ' ', 0)
		for _, r := range rows {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Area, r.Level, strings.TrimSpace(r.Code+" "+r.Message))
		}
		tw.Flush()
	}
	warns, infos, errsN := 0, 0, 0
	for _, r := range rows {
		switch r.Level {
		case "warn":
			warns++
		case "info":
			infos++
		case "error":
			errsN++
		}
	}
	fmt.Printf("\n%d error(s), %d warning(s), %d info.", errsN, warns, infos)
	if fixable > 0 && !*fix {
		fmt.Printf(" Run with --fix to apply %d fix(es).", fixable)
	}
	fmt.Println()
	if *fix && fixable > 0 {
		if err := applyFixes(configPath); err != nil {
			fmt.Fprintln(os.Stderr, "doctor --fix:", err)
			return exitUsage
		}
	}
	if errsN > 0 && code == exitOK {
		code = exitUsage
	}
	return code
}

// applyFixes rewrites deprecated spellings in place, keeping a backup.
func applyFixes(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	backup := fmt.Sprintf("%s.bak-%s", path, time.Now().UTC().Format("20060102T150405Z"))
	if err := os.WriteFile(backup, b, 0o644); err != nil {
		return err
	}
	out := strings.ReplaceAll(string(b), "trackingUri:", "tracking_uri:")
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Printf("          fixed    renamed adapters.mlflow.trackingUri → adapters.mlflow.tracking_uri\n          backup   %s\n", backup)
	return nil
}

// ---- version ----

func cmdVersion(args []string) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	info := map[string]any{"version": fendlemark.Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
		"console": "embedded", "adapters": adapter.Registered()}
	if *asJSON {
		b, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(b))
		return exitOK
	}
	fmt.Printf("fendlemark %s (%s, %s/%s)\nconsole: embedded\nadapters: %s\n", fendlemark.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH, strings.Join(adapter.Registered(), " "))
	return exitOK
}
