// Package config loads fendlemark.yaml with defaults, environment overrides
// and ${VAR} expansion. Unknown keys are errors, not warnings: the Keepers
// believe a configuration file should be exactly right or not at all.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/internal/schema"
)

// Config is the whole file.
type Config struct {
	Server    Server             `yaml:"server"`
	Console   Console            `yaml:"console"`
	Adapters  map[string]Adapter `yaml:"adapters"`
	Lattice   Lattice            `yaml:"lattice"`
	Auth      Auth               `yaml:"auth"`
	Telemetry Telemetry          `yaml:"telemetry"`

	findings []Finding
}

type Server struct {
	Listen        string        `yaml:"listen"`
	BaseURL       string        `yaml:"base_url"`
	DataDir       string        `yaml:"data_dir"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	ShutdownGrace time.Duration `yaml:"shutdown_grace"`
	TLS           TLS           `yaml:"tls"`
	LogLevel      string        `yaml:"log_level"`
}

type TLS struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type Console struct {
	Title              string `yaml:"title"`
	DefaultView        string `yaml:"default_view"`
	Theme              string `yaml:"theme"`
	Motto              bool   `yaml:"motto"`
	PageSize           int    `yaml:"page_size"`
	DeepLinks          bool   `yaml:"deep_links"`
	HideEmptyPanels    bool   `yaml:"hide_empty_panels"`
	GenerationInFooter bool   `yaml:"generation_in_footer"`
}

// Adapter is the common block plus adapter-specific options, inlined.
type Adapter struct {
	Enabled      bool              `yaml:"enabled"`
	PollInterval time.Duration     `yaml:"poll_interval"`
	Timeout      time.Duration     `yaml:"timeout"`
	MaxEntities  int               `yaml:"max_entities"`
	Labels       map[string]string `yaml:"labels"`
	Options      map[string]any    `yaml:",inline"`
}

type Lattice struct {
	Correlation       Correlation   `yaml:"correlation"`
	ReconcileInterval time.Duration `yaml:"reconcile_interval"`
	RetainOrphans     time.Duration `yaml:"retain_orphans"`
	MaxEntities       int           `yaml:"max_entities"`
}

type Correlation struct {
	Window   time.Duration `yaml:"window"`
	WeakKeys bool          `yaml:"weak_keys"`
	Custom   []CustomKey   `yaml:"custom"`
}

type CustomKey struct {
	Name string            `yaml:"name"`
	Weak bool              `yaml:"weak"`
	From map[string]string `yaml:"from"`
}

type Auth struct {
	Mode        string      `yaml:"mode"`
	Respiratory Respiratory `yaml:"respiratory"`
}

type Respiratory struct {
	Sensitivity         float64       `yaml:"sensitivity"`
	AllowMouthBreathing bool          `yaml:"allow_mouth_breathing"`
	EnrolmentBreaths    int           `yaml:"enrolment_breaths"`
	SecondFactor        bool          `yaml:"second_factor"`
	SessionTTL          time.Duration `yaml:"session_ttl"`
	CaptureDevice       string        `yaml:"capture_device"`
}

type Telemetry struct {
	Metrics        Metrics `yaml:"metrics"`
	Tracing        Tracing `yaml:"tracing"`
	UsageReporting bool    `yaml:"usage_reporting"`
}

type Metrics struct {
	Enabled bool `yaml:"enabled"`
}

type Tracing struct {
	Endpoint string `yaml:"endpoint"`
}

// KnownAdapters are the adapters compiled into the release binary. Any other
// adapter registered with adapter.Register is accepted as well.
var KnownAdapters = []string{"mlflow", "metaflow", "argo", "kubeflow"}

// Default returns the built-in defaults.
func Default() *Config {
	c := &Config{
		Server:    Server{Listen: "127.0.0.1:7411", DataDir: "./.fendlemark", ReadTimeout: 15 * time.Second, ShutdownGrace: 10 * time.Second, LogLevel: "info"},
		Console:   Console{Title: "Fendlemark", DefaultView: "overview", Theme: "auto", Motto: true, PageSize: 50, DeepLinks: true, HideEmptyPanels: true, GenerationInFooter: true},
		Lattice:   Lattice{Correlation: Correlation{Window: 90 * time.Second, WeakKeys: true}, ReconcileInterval: 5 * time.Second, RetainOrphans: time.Hour, MaxEntities: 200000},
		Auth:      Auth{Mode: "none", Respiratory: Respiratory{Sensitivity: 0.62, EnrolmentBreaths: 3, SessionTTL: 8 * time.Hour}},
		Telemetry: Telemetry{Metrics: Metrics{Enabled: true}},
		Adapters:  map[string]Adapter{},
	}
	for _, a := range KnownAdapters {
		c.Adapters[a] = Adapter{PollInterval: 30 * time.Second, Timeout: 10 * time.Second, MaxEntities: 50000, Options: map[string]any{}}
	}
	return c
}

// Finding is one thing doctor has to say about a configuration.
type Finding struct {
	Code     string
	Level    string // error, warn, info
	Message  string
	Fixable  bool
	Fix      func(*Config)
	path     string
	newValue any
}

// Load reads path (or defaults only when path is "") and applies environment
// overrides. Deprecated spellings are accepted with findings; see Validate.
// The findings, warnings included, are on the returned Config's Findings.
// When a finding is an error, Load returns it together with the Config.
func Load(path string) (*Config, error) {
	raw := map[string]any{}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(b, &raw); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	findings := applyDeprecations(raw)
	applyEnv(raw, os.Environ())
	expand(raw)
	cfg, err := decode(raw)
	if err != nil {
		return nil, err
	}
	cfg.findings = append(findings, Validate(cfg)...)
	for _, f := range cfg.findings {
		if f.Level == "error" {
			return cfg, fmt.Errorf("%s: %s", f.Code, f.Message)
		}
	}
	return cfg, nil
}

// Findings returns what Load had to say about the file, in order: deprecated
// spellings first, then Validate's findings.
func (c *Config) Findings() []Finding {
	if c == nil {
		return nil
	}
	return c.findings
}

func decode(raw map[string]any) (*Config, error) {
	cfg := Default()
	b, err := yaml.Marshal(raw)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, errEOF) && err.Error() != "EOF" {
		return nil, fmt.Errorf("config: %w", err)
	}
	for name, a := range cfg.Adapters {
		if a.Options == nil {
			a.Options = map[string]any{}
		}
		if a.PollInterval == 0 {
			a.PollInterval = 30 * time.Second
		}
		if a.Timeout == 0 {
			a.Timeout = 10 * time.Second
		}
		if a.MaxEntities == 0 {
			a.MaxEntities = 50000
		}
		cfg.Adapters[name] = a
	}
	return cfg, nil
}

var errEOF = errors.New("EOF")

// deprecations maps old key paths to their current spelling.
var deprecations = map[string]string{
	"adapters.mlflow.trackingUri": "adapters.mlflow.tracking_uri",
}

func applyDeprecations(raw map[string]any) []Finding {
	var out []Finding
	for old, cur := range deprecations {
		if v, ok := getPath(raw, old); ok {
			setPath(raw, cur, v)
			delPath(raw, old)
			out = append(out, Finding{Code: "DOC-104", Level: "warn", Fixable: true,
				Message: fmt.Sprintf("%s is the pre-0.9 spelling; use %s", old, cur), path: cur, newValue: v})
		}
	}
	return out
}

// applyEnv overlays FENDLEMARK_SECTION__KEY=value onto the raw map.
func applyEnv(raw map[string]any, environ []string) {
	for _, kv := range environ {
		if !strings.HasPrefix(kv, "FENDLEMARK_") {
			continue
		}
		k, v, _ := strings.Cut(kv[len("FENDLEMARK_"):], "=")
		if k == "CONFIG" {
			continue
		}
		path := strings.ToLower(strings.ReplaceAll(k, "__", "."))
		setPath(raw, path, coerce(v))
	}
}

func coerce(s string) any {
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		var list []any
		for _, p := range strings.Split(strings.Trim(s, "[]"), ",") {
			if p = strings.TrimSpace(p); p != "" {
				list = append(list, p)
			}
		}
		return list
	}
	return s
}

var varRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func expand(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, e := range t {
			t[k] = expand(e)
		}
		return t
	case []any:
		for i, e := range t {
			t[i] = expand(e)
		}
		return t
	case string:
		return varRe.ReplaceAllStringFunc(t, func(m string) string { return os.Getenv(m[2 : len(m)-1]) })
	}
	return v
}

func getPath(m map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = m
	for _, p := range parts {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = mm[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func setPath(m map[string]any, path string, v any) {
	parts := strings.Split(path, ".")
	cur := m
	for _, p := range parts[:len(parts)-1] {
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = v
}

func delPath(m map[string]any, path string) {
	parts := strings.Split(path, ".")
	cur := m
	for _, p := range parts[:len(parts)-1] {
		next, ok := cur[p].(map[string]any)
		if !ok {
			return
		}
		cur = next
	}
	delete(cur, parts[len(parts)-1])
}

// Validate returns findings about a decoded configuration.
func Validate(c *Config) []Finding {
	var out []Finding
	switch c.Server.LogLevel {
	case "debug", "info", "warn", "error", "serene":
	default:
		out = append(out, Finding{Code: "DOC-101", Level: "error", Message: fmt.Sprintf("server.log_level %q is not one of debug, info, warn, error, serene", c.Server.LogLevel)})
	}
	switch c.Console.Theme {
	case "auto", "light", "dark":
	default:
		out = append(out, Finding{Code: "DOC-102", Level: "error", Message: fmt.Sprintf("console.theme %q is not one of auto, light, dark", c.Console.Theme)})
	}
	for name, a := range c.Adapters {
		registered, ok := adapter.Lookup(name)
		known := ok
		for _, k := range KnownAdapters {
			if k == name {
				known = true
			}
		}
		if !known {
			out = append(out, Finding{Code: "DOC-110", Level: "error", Message: fmt.Sprintf("adapters.%s is not compiled into this binary", name)})
		}
		if !a.Enabled {
			continue
		}
		before := len(out)
		req := map[string]string{"mlflow": "tracking_uri", "metaflow": "metadata_url", "kubeflow": "pipelines_url"}[name]
		if req != "" {
			if s, _ := a.Options[req].(string); s == "" {
				out = append(out, Finding{Code: "DOC-111", Level: "error", Message: fmt.Sprintf("adapters.%s.%s is required when the adapter is enabled", name, req)})
			}
		}
		if name == "argo" {
			srv, _ := a.Options["server"].(string)
			kw, _ := a.Options["kube_watch"].(bool)
			if srv == "" && !kw {
				out = append(out, Finding{Code: "DOC-111", Level: "error", Message: "adapters.argo.server is required unless kube_watch is true"})
			}
			if ns, _ := a.Options["namespaces"].([]any); len(ns) == 0 {
				out = append(out, Finding{Code: "DOC-112", Level: "error", Message: "adapters.argo.namespaces must list at least one namespace"})
			}
		}
		// The adapter's own schema, unless the checks above already found the problem.
		if registered != nil && len(out) == before {
			if err := schema.Check(registered.Describe().ConfigSchema, a.Options); err != nil {
				out = append(out, Finding{Code: "DOC-113", Level: "error", Message: fmt.Sprintf("adapters.%s: %v", name, err)})
			}
		}
	}
	switch c.Auth.Mode {
	case "none":
		out = append(out, Finding{Code: "DOC-201", Level: "info", Message: "auth.mode is none; anyone who can reach server.listen can use the console"})
	case "respiratory":
		if c.Auth.Respiratory.Sensitivity < 0.4 {
			out = append(out, Finding{Code: "FRI-0007", Level: "error", Message: fmt.Sprintf("auth.respiratory.sensitivity %.2f is below 0.4", c.Auth.Respiratory.Sensitivity)})
		}
		if c.Auth.Respiratory.Sensitivity > 0.95 {
			out = append(out, Finding{Code: "DOC-202", Level: "warn", Message: "auth.respiratory.sensitivity above 0.95 will reject most honest exhalations"})
		}
		if c.Auth.Respiratory.EnrolmentBreaths < 3 {
			out = append(out, Finding{Code: "DOC-203", Level: "warn", Message: "auth.respiratory.enrolment_breaths below 3 weakens the Triad"})
		}
	default:
		out = append(out, Finding{Code: "DOC-200", Level: "error", Message: fmt.Sprintf("auth.mode %q is not none or respiratory, and there are no other modes", c.Auth.Mode)})
	}
	if c.Lattice.Correlation.Window < 5*time.Second {
		out = append(out, Finding{Code: "DOC-301", Level: "warn", Message: "lattice.correlation.window below 5s will miss most real correlations"})
	}
	if c.Telemetry.UsageReporting {
		out = append(out, Finding{Code: "DOC-401", Level: "info", Message: "telemetry.usage_reporting is true; it does nothing, and never has"})
	}
	return out
}

// Exposed reports whether a listen address accepts connections from other
// machines. Malformed addresses are left to the listen probe.
func Exposed(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "localhost" {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}

// Enabled returns the names of enabled adapters, sorted.
func (c *Config) Enabled() []string {
	var out []string
	for n, a := range c.Adapters {
		if a.Enabled {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// Effective renders the configuration as YAML with secrets redacted.
func (c *Config) Effective() string {
	b, _ := yaml.Marshal(c)
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		k := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(k, "password") || strings.Contains(k, "token") || strings.Contains(k, "secret") {
			if i := strings.Index(line, ":"); i > 0 {
				line = line[:i+1] + " <redacted>"
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// SetCount reports how many keys the file set explicitly, for doctor's summary.
func SetCount(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	raw := map[string]any{}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return 0, err
	}
	return countLeaves(raw), nil
}

func countLeaves(v any) int {
	switch t := v.(type) {
	case map[string]any:
		n := 0
		for _, e := range t {
			n += countLeaves(e)
		}
		return n
	case []any:
		return 1
	default:
		return 1
	}
}

// Minimal is the file written by `fendlemark init`.
func Minimal(enabled []string) string {
	on := map[string]bool{}
	for _, e := range enabled {
		on[e] = true
	}
	var b strings.Builder
	b.WriteString("# fendlemark.yaml\n# Every key has a default. Most installations change five of them.\n# Reference: https://fendlemark.app/docs/configuration/\n\nserver:\n  listen: \"127.0.0.1:7411\"   # this machine only; widen it only behind an authenticating proxy\n\nadapters:\n")
	fmt.Fprintf(&b, "  mlflow:\n    enabled: %t\n    tracking_uri: \"http://mlflow.internal:5000\"\n", on["mlflow"])
	fmt.Fprintf(&b, "  metaflow:\n    enabled: %t\n    metadata_url: \"http://metaflow-metadata.internal:8080\"\n", on["metaflow"])
	fmt.Fprintf(&b, "  argo:\n    enabled: %t\n    server: \"https://argo.internal:2746\"\n    namespaces: [\"ml-training\"]\n", on["argo"])
	fmt.Fprintf(&b, "  kubeflow:\n    enabled: %t\n    pipelines_url: \"http://ml-pipeline.kubeflow.svc:8888\"\n", on["kubeflow"])
	b.WriteString("\nauth:\n  mode: none\n")
	return b.String()
}
