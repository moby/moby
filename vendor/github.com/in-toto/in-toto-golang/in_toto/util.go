package in_toto

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var ErrUnknownMetadataType = errors.New("unknown metadata type encountered: not link or layout")

/*
Set represents a data structure for set operations. See `NewSet` for how to
create a Set, and available Set receivers for useful set operations.

Under the hood Set aliases map[string]struct{}, where the map keys are the set
elements and the map values are a memory-efficient way of storing the keys.
*/
type Set map[string]struct{}

/*
NewSet creates a new Set, assigns it the optionally passed variadic string
elements, and returns it.
*/
func NewSet(elems ...string) Set {
	var s Set = make(map[string]struct{})
	for _, elem := range elems {
		s.Add(elem)
	}
	return s
}

/*
Has returns True if the passed string is member of the set on which it was
called and False otherwise.
*/
func (s Set) Has(elem string) bool {
	_, ok := s[elem]
	return ok
}

/*
Add adds the passed string to the set on which it was called, if the string is
not a member of the set.
*/
func (s Set) Add(elem string) {
	s[elem] = struct{}{}
}

/*
Remove removes the passed string from the set on which was is called, if the
string is a member of the set.
*/
func (s Set) Remove(elem string) {
	delete(s, elem)
}

/*
Intersection creates and returns a new Set with the elements of the set on
which it was called that are also in the passed set.
*/
func (s Set) Intersection(s2 Set) Set {
	res := NewSet()
	for elem := range s {
		if !s2.Has(elem) {
			continue
		}
		res.Add(elem)
	}
	return res
}

/*
Difference creates and returns a new Set with the elements of the set on
which it was called that are not in the passed set.
*/
func (s Set) Difference(s2 Set) Set {
	res := NewSet()
	for elem := range s {
		if s2.Has(elem) {
			continue
		}
		res.Add(elem)
	}
	return res
}

/*
Filter creates and returns a new Set with the elements of the set on which it
was called that match the passed pattern. A matching error is treated like a
non-match plus a warning is printed.
*/
func (s Set) Filter(pattern string) Set {
	res := NewSet()
	for elem := range s {
		matched, err := match(pattern, elem)
		if err != nil {
			fmt.Printf("WARNING: %s, pattern was '%s'\n", err, pattern)
			continue
		}
		if !matched {
			continue
		}
		res.Add(elem)
	}
	return res
}

/*
Slice creates and returns an unordered string slice with the elements of the
set on which it was called.
*/
func (s Set) Slice() []string {
	var res []string
	res = make([]string, 0, len(s))
	for elem := range s {
		res = append(res, elem)
	}
	return res
}

/*
InterfaceKeyStrings returns string keys of passed interface{} map in an
unordered string slice.
*/
func InterfaceKeyStrings(m map[string]interface{}) []string {
	res := make([]string, len(m))
	i := 0
	for k := range m {
		res[i] = k
		i++
	}
	return res
}

/*
IsSubSet checks if the parameter subset is a
subset of the superset s.
*/
func (s Set) IsSubSet(subset Set) bool {
	if len(subset) > len(s) {
		return false
	}
	for key := range subset {
		if s.Has(key) {
			continue
		} else {
			return false
		}
	}
	return true
}

func loadPayload(payloadBytes []byte) (any, error) {
	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	if payload["_type"] == "link" {
		var link Link
		if err := checkRequiredJSONFields(payload, reflect.TypeOf(link)); err != nil {
			return nil, fmt.Errorf("error decoding payload: %w", err)
		}

		decoder := json.NewDecoder(strings.NewReader(string(payloadBytes)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&link); err != nil {
			return nil, fmt.Errorf("error decoding payload: %w", err)
		}

		return link, nil
	} else if payload["_type"] == "layout" {
		var layout Layout
		if err := checkRequiredJSONFields(payload, reflect.TypeOf(layout)); err != nil {
			return nil, fmt.Errorf("error decoding payload: %w", err)
		}

		decoder := json.NewDecoder(strings.NewReader(string(payloadBytes)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&layout); err != nil {
			return nil, fmt.Errorf("error decoding payload: %w", err)
		}

		return layout, nil
	}

	return nil, ErrUnknownMetadataType
}
