package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
	"unicode"

	"fendlemark.app/fendlemark"
	"fendlemark.app/fendlemark/config"
)

// At startup, serve asks the project whether there is anything to say about
// the running version, and prints the answer before its first log line. It
// waits at most noticeTimeout and starts either way. server.notices: false
// skips it.
const (
	noticeTimeout = 2 * time.Second
	noticeMaxSize = 64 << 10
)

// noticeURL is where serve looks. The tests point it somewhere else.
var noticeURL = "https://info.fendlemark.app/about-this-app"

// noticeKey signs the day's token, which the notice service requires. It is
// not a secret; it keeps the service quiet for anything that isn't fendlemark.
const noticeKey = "fendlemark notices"

// noticeToken returns the token for the UTC day containing t: the first 32
// hex digits of HMAC-SHA256(noticeKey, "notice:<days since 1970>"). The
// service also accepts the previous and next day's, for clocks a little off.
func noticeToken(t time.Time) string {
	mac := hmac.New(sha256.New, []byte(noticeKey))
	fmt.Fprintf(mac, "notice:%d", t.Unix()/86400)
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// fetchNotice returns the message at url, made safe to print.
func fetchNotice(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, noticeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+noticeToken(time.Now()))
	req.Header.Set("User-Agent", fmt.Sprintf("fendlemark/%s (%s/%s)", fendlemark.Version, runtime.GOOS, runtime.GOARCH))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("notice: %s", resp.Status)
	}
	var n struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, noticeMaxSize)).Decode(&n); err != nil {
		return "", fmt.Errorf("notice: %w", err)
	}
	return printable(n.Message), nil
}

// printable drops control and bidirectional formatting characters other than
// newlines and tabs, so a notice can't move the cursor or recolour the terminal.
func printable(s string) string {
	s = strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && (unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r)) {
			return -1
		}
		return r
	}, s)
	return strings.TrimRight(s, " \t\n")
}

// noticesOn reports whether server.notices is set. A file that could not be
// read leaves only the environment to go by.
func noticesOn(cfg *config.Config) bool {
	if cfg != nil {
		return cfg.Server.Notices
	}
	return !strings.EqualFold(os.Getenv("FENDLEMARK_SERVER__NOTICES"), "false")
}

// showNotice prints the notice at url, if there is one. With --log-format
// json it is a log record instead, so the output stays parseable; it is
// written whatever the log level, as the text form is.
func showNotice(log *slog.Logger, url string) {
	msg, err := fetchNotice(context.Background(), url)
	if err != nil {
		log.Debug("no startup notice", "err", err)
		return
	}
	if msg == "" {
		return
	}
	if logFormat == "json" {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Warn("notice", "text", msg)
		return
	}
	fmt.Fprintf(os.Stderr, "\n%s\n\n", msg)
}
