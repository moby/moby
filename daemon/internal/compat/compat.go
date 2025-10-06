// Package compat provides tools for backward-compatible API responses.
package compat

import (
	"bytes"
	"encoding/json"
)

// Wrapper augments a struct to add or omit fields for legacy JSON responses.
type Wrapper struct {
	Base any

	extraFields map[string]any
	omitFields  []string
}

// MarshalJSON merges the JSON with extra fields or omits fields.
func (w *Wrapper) MarshalJSON() ([]byte, error) {
	base, err := toJSON(w.Base)
	if err != nil {
		return nil, err
	}
	if len(w.omitFields) == 0 && len(w.extraFields) == 0 {
		return base, nil
	}

	var merged map[string]any
	if err := json.Unmarshal(base, &merged); err != nil {
		return nil, err
	}

	for _, key := range w.omitFields {
		delete(merged, key)
	}

	appendFields(w.extraFields, merged)

	return toJSON(merged)
}

func appendFields(src, dst map[string]any) {
	for k, v := range src {
		if vmap, ok := v.(map[string]any); ok {
			if dmap, ok := dst[k].(map[string]any); ok {
				appendFields(vmap, dmap)
				continue
			}
		}
		if _, ok := dst[k]; !ok {
			dst[k] = v
		}
	}
}

func toJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	out, _ := bytes.CutSuffix(buf.Bytes(), []byte{'\n'})
	return out, nil
}

type options struct {
	extraFields map[string]any
	omitFields  []string
}

// Option for Wrapper.
type Option func(*options)

// WithExtraFields adds extra JSON object fields to the marshaled output.
// The merge is recursive and additive-only: if a key already exists in
// the output, its existing value is preserved and the incoming value is
// ignored. For nested objects, missing keys are created as needed
// (depth-first traversal).
//
// If a conflict occurs between an object and a non-object at the same key,
// the existing (output) value is kept.
//
// Repeated calls accumulate; on conflicts, earlier calls win.
// This affects only the marshaled JSON, and does not mutate the source struct.
func WithExtraFields(fields map[string]any) Option {
	return func(c *options) {
		if c.extraFields == nil {
			c.extraFields = make(map[string]any)
		}
		appendFields(fields, c.extraFields)
	}
}

// WithOmittedFields removes fields from the marshaled output.
func WithOmittedFields(fields ...string) Option {
	return func(c *options) {
		c.omitFields = append(c.omitFields, fields...)
	}
}

// Wrap constructs a Wrapper from the given type.
func Wrap(base any, opts ...Option) *Wrapper {
	cfg := options{extraFields: make(map[string]any)}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Wrapper{
		Base:        base,
		extraFields: cfg.extraFields,
		omitFields:  cfg.omitFields,
	}
}
