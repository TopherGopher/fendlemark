package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

var jobs = json.RawMessage(`{"type":"object","required":["api_url","namespaces"],"properties":{
	"api_url":{"type":"string"},"namespaces":{"type":"array","items":{"type":"string"}},
	"mode":{"type":"string","enum":["poll","watch"]},"limit":{"type":"integer"},"verify":{"type":"boolean","default":true}}}`)

func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		opts map[string]any
		want string // substring of the error; empty for none
	}{
		{map[string]any{"api_url": "http://127.0.0.1:8001", "namespaces": []any{"ml-training"}}, ""},
		{map[string]any{"api_url": "http://x", "namespaces": []any{"a"}, "limit": 500, "verify": false, "mode": "poll", "extra": 1}, ""},
		{map[string]any{"api_url": 8001, "namespaces": []any{"a"}}, ""}, // numbers arrive from environment overrides
		{map[string]any{"namespaces": []any{"a"}}, "api_url is required"},
		{map[string]any{"api_url": "http://x", "namespaces": "a"}, "namespaces: want array, got a string"},
		{map[string]any{"api_url": "http://x", "namespaces": []any{"a", true}}, "namespaces[1]: want string"},
		{map[string]any{"api_url": "http://x", "namespaces": []any{"a"}, "limit": 2.5}, "limit: want integer"},
		{map[string]any{"api_url": "http://x", "namespaces": []any{"a"}, "mode": "stream"}, "mode: stream is not one of"},
		{map[string]any{"api_url": "http://x", "namespaces": []any{"a"}, "verify": "yes"}, "verify: want boolean"},
	} {
		err := Check(jobs, tc.opts)
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%v: unexpected error %v", tc.opts, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%v: got %v, want an error containing %q", tc.opts, err, tc.want)
		}
	}
}

func TestClosedSchema(t *testing.T) {
	closed := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"url":{"type":"string"}}}`)
	if err := Check(closed, map[string]any{"url": "http://x", "ulr": "http://y"}); err == nil || !strings.Contains(err.Error(), "ulr is not a known option") {
		t.Fatalf("got %v", err)
	}
}

func TestEmptyAndInvalidSchemas(t *testing.T) {
	if err := Check(nil, map[string]any{"anything": 1}); err != nil {
		t.Fatalf("empty schema: %v", err)
	}
	if err := Check(json.RawMessage(`{"type":`), nil); err == nil {
		t.Fatal("invalid schema should be reported")
	}
}
