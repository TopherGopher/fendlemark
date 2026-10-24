package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fendlemark.app/fendlemark/adapter"
)

func TestLoadWithDeprecationAndEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fendlemark.yaml")
	os.WriteFile(p, []byte("adapters:\n  mlflow:\n    enabled: true\n    trackingUri: \"http://old:5000\"\nlattice:\n  correlation:\n    window: 120s\n"), 0o644)
	t.Setenv("FENDLEMARK_SERVER__LISTEN", ":9999")
	t.Setenv("FENDLEMARK_ADAPTERS__MLFLOW__TRACKING_URI", "http://env:5000")
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	findings := cfg.Findings()
	if cfg.Server.Listen != ":9999" {
		t.Fatalf("env override failed: %q", cfg.Server.Listen)
	}
	if cfg.Adapters["mlflow"].Options["tracking_uri"] != "http://env:5000" {
		t.Fatalf("env should win over deprecated key: %v", cfg.Adapters["mlflow"].Options)
	}
	if cfg.Lattice.Correlation.Window != 120*time.Second {
		t.Fatalf("window: %v", cfg.Lattice.Correlation.Window)
	}
	var sawDeprecation bool
	for _, f := range findings {
		if f.Code == "DOC-104" {
			sawDeprecation = true
		}
	}
	if !sawDeprecation {
		t.Fatalf("expected DOC-104, got %+v", findings)
	}
}

func TestUnknownKeyIsError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fendlemark.yaml")
	os.WriteFile(p, []byte("server:\n  lisen: \":1\"\n"), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("unknown key should be an error")
	}
}

func TestExposed(t *testing.T) {
	if Exposed(Default().Server.Listen) {
		t.Fatalf("default listen %q must stay on this machine", Default().Server.Listen)
	}
	for addr, want := range map[string]bool{
		"127.0.0.1:7411": false,
		"localhost:7411": false,
		"[::1]:7411":     false,
		":7411":          true,
		"0.0.0.0:7411":   true,
		"[::]:7411":      true,
		"10.0.0.5:7411":  true,
		"console:7411":   true,
		"7411":           false, // malformed; the listen probe reports it
	} {
		if got := Exposed(addr); got != want {
			t.Errorf("Exposed(%q) = %v, want %v", addr, got, want)
		}
	}
}

func TestSensitivityFloor(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fendlemark.yaml")
	os.WriteFile(p, []byte("auth:\n  mode: respiratory\n  respiratory:\n    sensitivity: 0.3\n"), 0o644)
	cfg, err := Load(p)
	if err == nil {
		t.Fatal("sensitivity below 0.4 must be refused")
	}
	findings := cfg.Findings()
	if findings[len(findings)-1].Code != "FRI-0007" && findings[0].Code != "FRI-0007" {
		t.Fatalf("expected FRI-0007, got %+v", findings)
	}
}

// jobs stands in for an adapter compiled in from outside this module.
type jobs struct{}

func (jobs) Name() string { return "jobs" }
func (jobs) Describe() adapter.Descriptor {
	return adapter.Descriptor{Kinds: []adapter.Kind{adapter.KindRun}, Maturity: adapter.Provisional,
		ConfigSchema: json.RawMessage(`{"type":"object","required":["api_url"],"properties":{"api_url":{"type":"string"},"namespaces":{"type":"array","items":{"type":"string"}}}}`)}
}
func (jobs) Connect(context.Context, adapter.Config) error { return nil }
func (jobs) Facets(context.Context, adapter.Query) ([]adapter.Facet, error) {
	return nil, nil
}
func (jobs) Watch(context.Context) (<-chan adapter.Event, error) {
	return nil, adapter.ErrWatchUnsupported
}
func (jobs) Health(context.Context) adapter.Status { return adapter.Status{State: adapter.Healthy} }

func init() { adapter.Register("jobs", func() adapter.Adapter { return jobs{} }) }

func TestRegisteredAdapterIsKnown(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fendlemark.yaml")
	os.WriteFile(p, []byte("adapters:\n  jobs:\n    enabled: true\n    api_url: \"http://127.0.0.1:8001\"\n    namespaces: [\"ml-training\"]\n"), 0o644)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("a registered adapter should need no KnownAdapters entry: %v", err)
	}
	if a := cfg.Adapters["jobs"]; a.PollInterval != 30*time.Second || a.Options["api_url"] != "http://127.0.0.1:8001" {
		t.Fatalf("jobs: %+v", a)
	}
	os.WriteFile(p, []byte("adapters:\n  unheard_of:\n    enabled: true\n"), 0o644)
	if _, err := Load(p); err == nil || !strings.HasPrefix(err.Error(), "DOC-110") {
		t.Fatalf("an adapter nobody registered is still DOC-110, got %v", err)
	}
}

func TestConfigSchemaCheckedOnLoad(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fendlemark.yaml")
	for body, want := range map[string]string{
		"adapters:\n  jobs:\n    enabled: true\n    namespaces: [\"ml-training\"]\n":                          "DOC-113: adapters.jobs: api_url is required",
		"adapters:\n  jobs:\n    enabled: true\n    api_url: \"http://x\"\n    namespaces: \"ml-training\"\n": "DOC-113: adapters.jobs: namespaces: want array, got a string",
		"adapters:\n  jobs:\n    enabled: false\n":                                                            "",
	} {
		os.WriteFile(p, []byte(body), 0o644)
		_, err := Load(p)
		if want == "" {
			if err != nil {
				t.Errorf("a disabled adapter is not checked: %v", err)
			}
			continue
		}
		if err == nil || err.Error() != want {
			t.Errorf("got %v, want %s", err, want)
		}
	}
}
