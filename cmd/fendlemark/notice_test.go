package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// The tests never fetch the real notice. TestMain points noticeURL at a port
// nothing listens on, and the one test that reaches the service uses its test
// notice, which has the same shape and says only what the documentation says.
const testNoticeURL = "https://info.fendlemark.app/test-post"

func TestMain(m *testing.M) {
	noticeURL = "http://127.0.0.1:1/"
	os.Exit(m.Run())
}

func TestNoticeToken(t *testing.T) {
	// The same vector is checked against the service's implementation.
	day := time.Unix(20000*86400, 0)
	const want = "38623998cee0adc88b00d5993ea8d756"
	for _, at := range []time.Time{day, day.Add(23*time.Hour + 59*time.Minute)} {
		if got := noticeToken(at); got != want {
			t.Errorf("noticeToken(%v) = %s, want %s", at, got, want)
		}
	}
	if noticeToken(day.Add(24*time.Hour)) == want {
		t.Error("the token should change at midnight UTC")
	}
}

func TestFetchNotice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+noticeToken(time.Now()) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"\u001b[31mred\u001b[0m\n\tline two‮\n\n"}`))
	}))
	defer srv.Close()
	got, err := fetchNotice(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[31mred[0m\n\tline two"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	srv.Config.Handler = http.NotFoundHandler()
	if _, err := fetchNotice(context.Background(), srv.URL); err == nil {
		t.Fatal("a 404 should be an error")
	}
}

func TestFetchNoticeGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	start := time.Now()
	if _, err := fetchNotice(context.Background(), srv.URL); err == nil {
		t.Fatal("a server that never answers should be an error")
	}
	if waited := time.Since(start); waited > noticeTimeout+time.Second {
		t.Fatalf("waited %v; the limit is %v", waited, noticeTimeout)
	}
}

// TestNoticeService fetches the test notice from the real service, to check
// that it still accepts today's token. It is skipped with -short or when the
// network is unreachable, but fails if the service answers wrongly.
func TestNoticeService(t *testing.T) {
	if testing.Short() {
		t.Skip("needs the network")
	}
	msg, err := fetchNotice(context.Background(), testNoticeURL)
	var netErr net.Error
	if errors.As(err, &netErr) {
		t.Skipf("network unreachable: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "Fendlemark") {
		t.Fatalf("unexpected test notice:\n%s", msg)
	}
}
