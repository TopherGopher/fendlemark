// Package schema checks an adapter's options against the JSON Schema in its
// Descriptor.ConfigSchema. It understands the part of JSON Schema that
// configuration needs: type, required, properties, additionalProperties,
// items, and enum. Other keywords, such as default, are ignored.
package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type node struct {
	Type                 string           `json:"type"`
	Required             []string         `json:"required"`
	Properties           map[string]*node `json:"properties"`
	AdditionalProperties *bool            `json:"additionalProperties"`
	Items                *node            `json:"items"`
	Enum                 []any            `json:"enum"`
}

// Check reports the first way opts fails schema, or nil. An empty schema
// accepts anything.
func Check(schema json.RawMessage, opts map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	var n node
	if err := json.Unmarshal(schema, &n); err != nil {
		return fmt.Errorf("ConfigSchema is not valid JSON Schema: %w", err)
	}
	if opts == nil {
		opts = map[string]any{}
	}
	return n.check("", opts)
}

func (n *node) check(path string, v any) error {
	if n.Type != "" && !is(n.Type, v) {
		return fmt.Errorf("%s: want %s, got %s", name(path), n.Type, kind(v))
	}
	if len(n.Enum) > 0 && !inEnum(n.Enum, v) {
		return fmt.Errorf("%s: %v is not one of %v", name(path), v, n.Enum)
	}
	switch t := v.(type) {
	case map[string]any:
		for _, r := range n.Required {
			if _, ok := t[r]; !ok {
				return fmt.Errorf("%s is required", join(path, r))
			}
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p, ok := n.Properties[k]
			if !ok {
				if n.AdditionalProperties != nil && !*n.AdditionalProperties {
					return fmt.Errorf("%s is not a known option", join(path, k))
				}
				continue
			}
			if err := p.check(join(path, k), t[k]); err != nil {
				return err
			}
		}
	case []any:
		if n.Items != nil {
			for i, e := range t {
				if err := n.Items.check(fmt.Sprintf("%s[%d]", path, i), e); err != nil {
					return err
				}
			}
		}
	case []string:
		if n.Items != nil {
			for i, e := range t {
				if err := n.Items.check(fmt.Sprintf("%s[%d]", path, i), e); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// is reports whether v, as decoded from YAML or set by an environment
// override, has the JSON Schema type typ. Numbers are accepted as strings
// because an override such as FENDLEMARK_ADAPTERS__X__TOKEN=1234 arrives as
// one, and adapter.Config.String renders them.
func is(typ string, v any) bool {
	switch typ {
	case "string":
		switch v.(type) {
		case string, int, int64, float64:
			return true
		}
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "integer":
		switch t := v.(type) {
		case int, int64:
			return true
		case float64:
			return t == float64(int64(t))
		}
	case "number":
		switch v.(type) {
		case int, int64, float64:
			return true
		}
	case "array":
		switch v.(type) {
		case []any, []string:
			return true
		}
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "null":
		return v == nil
	default:
		return true
	}
	return false
}

func kind(v any) string {
	switch v.(type) {
	case nil:
		return "nothing"
	case string:
		return "a string"
	case bool:
		return "a boolean"
	case int, int64, float64:
		return "a number"
	case []any, []string:
		return "a list"
	case map[string]any:
		return "a mapping"
	}
	return reflect.TypeOf(v).String()
}

func inEnum(enum []any, v any) bool {
	for _, e := range enum {
		if fmt.Sprint(e) == fmt.Sprint(v) {
			return true
		}
	}
	return false
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func name(path string) string {
	if path == "" {
		return "options"
	}
	return strings.TrimPrefix(path, ".")
}
