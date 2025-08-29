// Package compat provides tools for backward-compatible API responses.
package compat

import "encoding/json"

// Wrapper augments a struct to add or omit fields for legacy JSON responses.
type Wrapper struct {
	Base any

	extraFields map[string]any
	omitFields  []string
}

// MarshalJSON merges the JSON with extra fields or omits fields.
func (w *Wrapper) MarshalJSON() ([]byte, error) {
	base, err := json.Marshal(w.Base)
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
	for key, val := range w.extraFields {
		merged[key] = val
	}

	return json.Marshal(merged)
}

type options struct {
	extraFields map[string]any
	omitFields  []string
}

// Option for Wrapper.
type Option func(*options)

// WithExtraFields adds fields to the marshaled output.
func WithExtraFields(fields map[string]any) Option {
	return func(c *options) {
		if c.extraFields == nil {
			c.extraFields = make(map[string]any)
		}
		for k, v := range fields {
			c.extraFields[k] = v
		}
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
