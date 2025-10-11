package client

import (
	"encoding/json"
	"net/url"
)

// Filters describes a predicate for an API request.
//
// Each entry in the map is a filter term.
// Each term is evaluated against the set of values.
// A filter term is satisfied if any one of the values in the set is a match.
// An item matches the filters when all terms are satisfied.
//
// Like all other map types in Go, the zero value is empty and read-only.
type Filters map[string]map[string]bool

// Add appends values to the value-set of term.
//
// The receiver f is returned for chaining.
//
//	f := make(Filters).Add("name", "foo", "bar").Add("status", "exited")
func (f Filters) Add(term string, values ...string) Filters {
	if _, ok := f[term]; !ok {
		f[term] = make(map[string]bool)
	}
	for _, v := range values {
		f[term][v] = true
	}
	return f
}

// Clone returns a deep copy of f.
func (f Filters) Clone() Filters {
	out := make(Filters, len(f))
	for term, values := range f {
		inner := make(map[string]bool, len(values))
		for v, ok := range values {
			inner[v] = ok
		}
		out[term] = inner
	}
	return out
}

// updateURLValues sets the "filters" key in values to the marshalled value of
// f, replacing any existing values. When f is empty, any existing "filters" key
// is removed.
func (f Filters) updateURLValues(values url.Values) {
	if len(f) > 0 {
		b, err := json.Marshal(f)
		if err != nil {
			panic(err) // Marshaling builtin types should never fail
		}
		values.Set("filters", string(b))
	} else {
		values.Del("filters")
	}
}
