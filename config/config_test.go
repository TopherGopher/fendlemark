package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadWithDeprecationAndEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fendlemark.yaml")
	os.WriteFile(p, []byte("adapters:\n  mlflow:\n    enabled: true\n    trackingUri: \"http://old:5000\"\nlattice:\n  correlation:\n    window: 120s\n"), 0o644)
	t.Setenv("FENDLEMARK_SERVER__LISTEN", ":9999")
	t.Setenv("FENDLEMARK_ADAPTERS__MLFLOW__TRACKING_URI", "http://env:5000")
	cfg, findings, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
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
	if _, _, err := Load(p); err == nil {
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
	_, findings, err := Load(p)
	if err == nil {
		t.Fatal("sensitivity below 0.4 must be refused")
	}
	if findings[len(findings)-1].Code != "FRI-0007" && findings[0].Code != "FRI-0007" {
		t.Fatalf("expected FRI-0007, got %+v", findings)
	}
}
