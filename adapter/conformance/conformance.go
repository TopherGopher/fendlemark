// Package conformance is the test harness every adapter must pass before it
// is compiled into the binary. It encodes the rules on the Writing an adapter
// page plus a few that are only obvious after someone has broken them.
package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"fendlemark.app/fendlemark/adapter"
)

// Options configures a conformance run.
type Options struct {
	// Fixture is a recorded upstream response, such as testdata/jobs.json.
	// When set, the suite serves it at http://fixture: every request gets the
	// file, and any string in Config beginning http://fixture is pointed at
	// it. Once the other checks pass, the fixture stops and Health must still
	// return within HealthWithin. Leave it empty to run your own fixture
	// server and put its URL in Config.
	Fixture string
	// Config is passed to Connect.
	Config adapter.Config
	// MinFacets is the minimum number of facets Facets must return.
	MinFacets int
	// ConnectWithin bounds Connect; default two seconds.
	ConnectWithin time.Duration
	// HealthWithin bounds Health; default the adapter's timeout or ten seconds.
	HealthWithin time.Duration
}

var hex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Run executes the suite against a fresh adapter from ctor.
func Run(t testing.TB, ctor func() adapter.Adapter, opts Options) {
	t.Helper()
	if opts.ConnectWithin == 0 {
		opts.ConnectWithin = 2 * time.Second
	}
	if opts.HealthWithin == 0 {
		opts.HealthWithin = 10 * time.Second
	}
	var stopFixture func()
	if opts.Fixture != "" {
		srv := serveFixture(t, opts.Fixture)
		opts.Config = pointAt(opts.Config, srv.URL)
		stopFixture = srv.Close
	}
	a := ctor()
	if a.Name() == "" {
		t.Fatal("Name() is empty")
	}

	d1, d2 := a.Describe(), a.Describe()
	if !reflect.DeepEqual(d1, d2) {
		t.Fatal("Describe() is not pure: two calls differ")
	}
	if len(d1.Kinds) == 0 {
		t.Fatal("Describe().Kinds is empty")
	}
	if d1.Maturity == "" {
		t.Fatal("Describe().Maturity is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.ConnectWithin)
	start := time.Now()
	err := a.Connect(ctx, opts.Config)
	cancel()
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if took := time.Since(start); took > opts.ConnectWithin {
		t.Fatalf("Connect took %s; it must not fetch facets", took)
	}

	facets, err := a.Facets(context.Background(), adapter.Query{})
	if err != nil {
		t.Fatalf("Facets: %v", err)
	}
	if len(facets) < opts.MinFacets {
		t.Fatalf("Facets returned %d facets, want at least %d", len(facets), opts.MinFacets)
	}
	seen := map[string]bool{}
	declared := map[string]bool{}
	for _, k := range d1.Keys {
		declared[k] = true
	}
	for _, f := range facets {
		if err := f.Validate(); err != nil {
			t.Fatal(err)
		}
		if f.Source != a.Name() {
			t.Fatalf("facet %s/%s: Source must equal Name() %q", f.Source, f.SourceID, a.Name())
		}
		if seen[f.Ref()] {
			t.Fatalf("duplicate facet %s/%s in one Facets call", f.Source, f.SourceID)
		}
		seen[f.Ref()] = true
		for k, v := range f.Keys {
			if !declared[k] {
				t.Fatalf("facet %s/%s emits undeclared key %q", f.Source, f.SourceID, k)
			}
			if hex32.MatchString(v) && k != adapter.KeyMLflowRunID && k != adapter.KeyKFPRunID {
				t.Fatalf("facet %s/%s: 32-hex value under key %q looks like an MLflow run id", f.Source, f.SourceID, k)
			}
		}
	}

	ch, err := a.Watch(context.Background())
	switch {
	case errors.Is(err, adapter.ErrWatchUnsupported):
		if d1.Watchable {
			t.Fatal("Describe().Watchable is true but Watch returned ErrWatchUnsupported")
		}
	case err != nil:
		t.Fatalf("Watch: %v", err)
	default:
		if !d1.Watchable {
			t.Fatal("Watch succeeded but Describe().Watchable is false")
		}
		wctx, wcancel := context.WithCancel(context.Background())
		ch, err = a.Watch(wctx)
		if err != nil {
			t.Fatalf("Watch (second): %v", err)
		}
		wcancel()
		select {
		case _, open := <-ch:
			for open {
				_, open = <-ch
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Watch channel did not close within 2s of context cancellation")
		}
	}

	hctx, hcancel := context.WithTimeout(context.Background(), opts.HealthWithin)
	start = time.Now()
	st := a.Health(hctx)
	hcancel()
	if st.State == "" {
		t.Fatal("Health returned an empty State")
	}
	if took := time.Since(start); took > opts.HealthWithin {
		t.Fatalf("Health took %s, over the bound of %s", took, opts.HealthWithin)
	}

	if stopFixture == nil {
		return
	}
	stopFixture()
	hctx, hcancel = context.WithTimeout(context.Background(), opts.HealthWithin)
	start = time.Now()
	st = a.Health(hctx)
	hcancel()
	if st.State == "" {
		t.Fatal("Health returned an empty State with the upstream unreachable")
	}
	if took := time.Since(start); took > opts.HealthWithin {
		t.Fatalf("Health took %s with the upstream unreachable, over the bound of %s", took, opts.HealthWithin)
	}
}

// fixtureURL is the placeholder upstream that Options.Fixture replaces.
const fixtureURL = "http://fixture"

func serveFixture(t testing.TB, path string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Fixture: %v", err)
	}
	isJSON := json.Valid(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isJSON {
			w.Header().Set("Content-Type", "application/json")
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// pointAt returns a copy of cfg with http://fixture replaced by url.
func pointAt(cfg adapter.Config, url string) adapter.Config {
	out := adapter.Config{}
	for k, v := range cfg {
		if s, ok := v.(string); ok && strings.HasPrefix(s, fixtureURL) {
			if rest := s[len(fixtureURL):]; rest == "" || strings.ContainsRune("/?#", rune(rest[0])) {
				v = url + rest
			}
		}
		out[k] = v
	}
	return out
}
