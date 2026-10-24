// Package httpx is the small HTTP client shared by the built-in adapters:
// timeouts, bearer and basic auth, JSON decoding, and upstream error shaping.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a base URL plus credentials.
type Client struct {
	Base     string
	Timeout  time.Duration
	Bearer   string
	User     string
	Password string
	Header   http.Header
	HTTP     *http.Client
}

// New constructs a client. base must be absolute.
func New(base string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q", base)
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{Base: strings.TrimRight(base, "/"), Timeout: timeout, Header: http.Header{}, HTTP: &http.Client{Timeout: timeout}}, nil
}

// Error is a non-2xx upstream response.
type Error struct {
	Status int
	Body   string
	URL    string
}

func (e *Error) Error() string {
	b := e.Body
	if len(b) > 160 {
		b = b[:160] + "…"
	}
	return fmt.Sprintf("%s: HTTP %d: %s", e.URL, e.Status, strings.TrimSpace(b))
}

// GetJSON performs GET and decodes JSON into out.
func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

// PostJSON performs POST with a JSON body and decodes JSON into out.
func (c *Client) PostJSON(ctx context.Context, path string, body any, out any) error {
	return c.do(ctx, http.MethodPost, path, nil, body, out)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	u := c.Base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fendlemark/0.9.6")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.Header {
		req.Header[k] = v
	}
	if c.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.Bearer)
	} else if c.User != "" {
		req.SetBasicAuth(c.User, c.Password)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &Error{Status: resp.StatusCode, Body: string(b), URL: u}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Ping performs a GET and reports latency, discarding the body.
func (c *Client) Ping(ctx context.Context, path string) (time.Duration, error) {
	start := time.Now()
	err := c.do(ctx, http.MethodGet, path, nil, nil, nil)
	return time.Since(start), err
}

// Millis converts a millisecond epoch to time.Time, zero for 0.
func Millis(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

// ParseTime parses RFC 3339 (with or without fractional seconds), zero on failure.
func ParseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
