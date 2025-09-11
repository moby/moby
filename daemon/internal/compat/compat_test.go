package compat_test

import (
	"encoding/json"
	"testing"

	"github.com/moby/moby/v2/daemon/internal/compat"
)

type Info struct {
	Name     string        `json:"name"`
	Version  string        `json:"version"`
	NewField string        `json:"newfield"`
	Nested   *NestedStruct `json:"legacy,omitempty"`
}

type NestedStruct struct {
	Field1 string `json:"field1"`
	Field2 int    `json:"field2"`
}

func TestWrap(t *testing.T) {
	info := &Info{
		Name:     "daemon",
		Version:  "v2.0",
		NewField: "new field",
	}

	tests := []struct {
		name     string
		options  []compat.Option
		expected string
	}{
		{
			name:     "none",
			expected: `{"name":"daemon","version":"v2.0","newfield":"new field"}`,
		},
		{
			name:     "extra fields",
			options:  []compat.Option{compat.WithExtraFields(map[string]any{"legacy_field": "hello"})},
			expected: `{"legacy_field":"hello","name":"daemon","newfield":"new field","version":"v2.0"}`,
		},
		{
			name:     "omit fields",
			options:  []compat.Option{compat.WithOmittedFields("newfield", "version")},
			expected: `{"name":"daemon"}`,
		},
		{
			name: "omit and extra fields",
			options: []compat.Option{
				compat.WithExtraFields(map[string]any{"legacy_field": "hello"}),
				compat.WithOmittedFields("newfield", "version"),
			},
			expected: `{"legacy_field":"hello","name":"daemon"}`,
		},
		{
			name: "replace field",
			options: []compat.Option{compat.WithExtraFields(map[string]any{"version": struct {
				Major, Minor int
			}{Major: 1, Minor: 0}})},
			expected: `{"name":"daemon","newfield":"new field","version":{"Major":1,"Minor":0}}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := compat.Wrap(info, tc.options...)
			data, err := json.Marshal(resp)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tc.expected {
				t.Errorf("\nExpected: %s\nGot:      %s", tc.expected, string(data))
			}
		})
	}
}

func TestNestedCompat(t *testing.T) {
	info := &Info{
		Name:     "daemon",
		Version:  "v2.0",
		NewField: "new field",
	}

	detail := &NestedStruct{
		Field1: "ok",
		Field2: 42,
	}
	nested := compat.Wrap(detail, compat.WithExtraFields(map[string]any{
		"legacy_field": "hello",
	}))
	resp := compat.Wrap(info, compat.WithExtraFields(map[string]any{
		"nested": nested,
	}))

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	const expected = `{"name":"daemon","nested":{"field1":"ok","field2":42,"legacy_field":"hello"},"newfield":"new field","version":"v2.0"}`
	if string(data) != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, string(data))
	}
}
