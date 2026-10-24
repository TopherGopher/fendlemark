package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// quiet sends stdout and stderr to files for the rest of the test and returns
// a function that reads back what went to stderr.
func quiet(t *testing.T) (stderr func() string) {
	t.Helper()
	dir := t.TempDir()
	out, _ := os.Create(filepath.Join(dir, "stdout"))
	errf, _ := os.Create(filepath.Join(dir, "stderr"))
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, errf
	t.Cleanup(func() { os.Stdout, os.Stderr = oldOut, oldErr; out.Close(); errf.Close() })
	return func() string {
		b, _ := os.ReadFile(errf.Name())
		return string(b)
	}
}

func TestGlobalFlags(t *testing.T) {
	quiet(t)
	for _, tc := range []struct {
		args              []string
		config, logFormat string
	}{
		{[]string{"-c", "/etc/fm.yaml", "version"}, "/etc/fm.yaml", "text"},
		{[]string{"--config=/etc/fm.yaml", "--log-format", "json", "version"}, "/etc/fm.yaml", "json"},
		{[]string{"version", "--log-format=json", "-c", "/etc/fm.yaml"}, "/etc/fm.yaml", "json"},
		{[]string{"--no-color", "version", "--no-color"}, "./fendlemark.yaml", "text"},
	} {
		if code := run(tc.args); code != exitOK {
			t.Errorf("%v: exit %d", tc.args, code)
		}
		if configPath != tc.config || logFormat != tc.logFormat {
			t.Errorf("%v: config %q, log format %q", tc.args, configPath, logFormat)
		}
	}
	if code := run([]string{"version", "--log-format"}); code != exitUsage {
		t.Errorf("a global flag with no value: exit %d, want %d", code, exitUsage)
	}
	if code := run([]string{"--help"}); code != exitOK {
		t.Errorf("--help: exit %d", code)
	}
}

func TestTakeGlobalFlagsLeavesTheRest(t *testing.T) {
	got, err := takeGlobalFlags(globalFlags(), []string{"--listen", ":7411", "-c", "x.yaml", "--", "-c", "kept"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "--listen :7411 -- -c kept"; strings.Join(got, " ") != want {
		t.Fatalf("got %q, want %q", strings.Join(got, " "), want)
	}
}

// TestServeShowsNoticeFirst checks that serve shows the notice even when the
// configuration is wrong, and not at all when notices are off.
func TestServeShowsNoticeFirst(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+noticeToken(time.Now()) {
			http.NotFound(w, r)
			return
		}
		hits.Add(1)
		w.Write([]byte(`{"message":"a notice from the test server"}`))
	}))
	defer srv.Close()
	old := noticeURL
	noticeURL = srv.URL
	defer func() { noticeURL = old }()

	bad := filepath.Join(t.TempDir(), "fendlemark.yaml")
	os.WriteFile(bad, []byte("server:\n  log_level: loud\n"), 0o644)

	stderr := quiet(t)
	if code := run([]string{"-c", bad, "serve"}); code != exitUsage {
		t.Fatalf("serve with a bad config: exit %d, want %d", code, exitUsage)
	}
	out := stderr()
	notice, failure := strings.Index(out, "a notice from the test server"), strings.Index(out, "DOC-101")
	if hits.Load() != 1 || notice < 0 || failure < notice {
		t.Fatalf("want the notice, then the configuration error; got %d requests and:\n%s", hits.Load(), out)
	}

	t.Setenv("FENDLEMARK_SERVER__NOTICES", "false")
	run([]string{"-c", bad, "serve"})
	if hits.Load() != 1 {
		t.Fatal("server.notices: false should skip the notice")
	}
}
