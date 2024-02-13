/*
Package filters provides tools for encoding a mapping of keys to a set of
multiple values.
*/
package filters // import "github.com/docker/docker/api/types/filters"

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types/versions"
)

// Args stores a mapping of keys to a set of multiple values.
type Args struct {
	fields map[string]map[string]bool
}

// KeyValuePair are used to initialize a new Args
type KeyValuePair struct {
	Key   string
	Value string
}

// Arg creates a new KeyValuePair for initializing Args
func Arg(key, value string) KeyValuePair {
	return KeyValuePair{Key: key, Value: value}
}

// NewArgs returns a new Args populated with the initial args
func NewArgs(initialArgs ...KeyValuePair) Args {
	args := Args{fields: map[string]map[string]bool{}}
	for _, arg := range initialArgs {
		args.Add(arg.Key, arg.Value)
	}
	return args
}

// Keys returns all the keys in list of Args
func (args Args) Keys() []string {
	keys := make([]string, 0, len(args.fields))
	for k := range args.fields {
		keys = append(keys, k)
	}
	return keys
}

// MarshalJSON returns a JSON byte representation of the Args
func (args Args) MarshalJSON() ([]byte, error) {
	if len(args.fields) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(args.fields)
}

// ToJSON returns the Args as a JSON encoded string
func ToJSON(a Args) (string, error) {
	if a.Len() == 0 {
		return "", nil
	}
	buf, err := json.Marshal(a)
	return string(buf), err
}

// ToParamWithVersion encodes Args as a JSON string. If version is less than 1.22
// then the encoded format will use an older legacy format where the values are a
// list of strings, instead of a set.
//
// Deprecated: do not use in any new code; use ToJSON instead
func ToParamWithVersion(version string, a Args) (string, error) {
	if a.Len() == 0 {
		return "", nil
	}

	if version != "" && versions.LessThan(version, "1.22") {
		buf, err := json.Marshal(convertArgsToSlice(a.fields))
		return string(buf), err
	}

	return ToJSON(a)
}

// FromJSON decodes a JSON encoded string into Args
func FromJSON(p string) (Args, error) {
	args := NewArgs()

	if p == "" {
		return args, nil
	}

	raw := []byte(p)
	err := json.Unmarshal(raw, &args)
	if err == nil {
		return args, nil
	}

	// Fallback to parsing arguments in the legacy slice format
	deprecated := map[string][]string{}
	if legacyErr := json.Unmarshal(raw, &deprecated); legacyErr != nil {
		return args, &invalidFilter{}
	}

	args.fields = deprecatedArgs(deprecated)
	return args, nil
}

// UnmarshalJSON populates the Args from JSON encode bytes
func (args Args) UnmarshalJSON(raw []byte) error {
	return json.Unmarshal(raw, &args.fields)
}

// Get returns the list of values associated with the key
func (args Args) Get(key string) []string {
	values := args.fields[key]
	if values == nil {
		return make([]string, 0)
	}
	slice := make([]string, 0, len(values))
	for key := range values {
		slice = append(slice, key)
	}
	return slice
}

// Add a new value to the set of values
func (args Args) Add(key, value string) {
	if _, ok := args.fields[key]; ok {
		args.fields[key][value] = true
	} else {
		args.fields[key] = map[string]bool{value: true}
	}
}

// Del removes a value from the set
func (args Args) Del(key, value string) {
	if _, ok := args.fields[key]; ok {
		delete(args.fields[key], value)
		if len(args.fields[key]) == 0 {
			delete(args.fields, key)
		}
	}
}

// Len returns the number of keys in the mapping
func (args Args) Len() int {
	return len(args.fields)
}

// MatchKVList returns true if all the pairs in sources exist as key=value
// pairs in the mapping at key, or if there are no values at key.
func (args Args) MatchKVList(key string, sources map[string]string) bool {
	fieldValues := args.fields[key]

	// do not filter if there is no filter set or cannot determine filter
	if len(fieldValues) == 0 {
		return true
	}

	if len(sources) == 0 {
		return false
	}

	for value := range fieldValues {
		testK, testV, hasValue := strings.Cut(value, "=")

		v, ok := sources[testK]
		if !ok {
			return false
		}
		if hasValue && testV != v {
			return false
		}
	}

	return true
}

// Match returns true if any of the values at key match the source string
func (args Args) Match(field, source string) bool {
	if args.ExactMatch(field, source) {
		return true
	}

	fieldValues := args.fields[field]
	for name2match := range fieldValues {
		match, err := regexp.MatchString(name2match, source)
		if err != nil {
			continue
		}
		if match {
			return true
		}
	}
	return false
}

// GetBoolOrDefault returns a boolean value of the key if the key is present
// and is intepretable as a boolean value. Otherwise the default value is returned.
// Error is not nil only if the filter values are not valid boolean or are conflicting.
func (args Args) GetBoolOrDefault(key string, defaultValue bool) (bool, error) {
	fieldValues, ok := args.fields[key]

	if !ok {
		return defaultValue, nil
	}

	if len(fieldValues) == 0 {
		return defaultValue, &invalidFilter{key, nil}
	}

	isFalse := fieldValues["0"] || fieldValues["false"]
	isTrue := fieldValues["1"] || fieldValues["true"]

	conflicting := isFalse && isTrue
	invalid := !isFalse && !isTrue

	if conflicting || invalid {
		return defaultValue, &invalidFilter{key, args.Get(key)}
	} else if isFalse {
		return false, nil
	} else if isTrue {
		return true, nil
	}

	// This code shouldn't be reached.
	return defaultValue, &unreachableCode{Filter: key, Value: args.Get(key)}
}

// ExactMatch returns true if the source matches exactly one of the values.
func (args Args) ExactMatch(key, source string) bool {
	fieldValues, ok := args.fields[key]
	// do not filter if there is no filter set or cannot determine filter
	if !ok || len(fieldValues) == 0 {
		return true
	}

	// try to match full name value to avoid O(N) regular expression matching
	return fieldValues[source]
}

// UniqueExactMatch returns true if there is only one value and the source
// matches exactly the value.
func (args Args) UniqueExactMatch(key, source string) bool {
	fieldValues := args.fields[key]
	// do not filter if there is no filter set or cannot determine filter
	if len(fieldValues) == 0 {
		return true
	}
	if len(args.fields[key]) != 1 {
		return false
	}

	// try to match full name value to avoid O(N) regular expression matching
	return fieldValues[source]
}

// FuzzyMatch returns true if the source matches exactly one value,  or the
// source has one of the values as a prefix.
func (args Args) FuzzyMatch(key, source string) bool {
	if args.ExactMatch(key, source) {
		return true
	}

	fieldValues := args.fields[key]
	for prefix := range fieldValues {
		if strings.HasPrefix(source, prefix) {
			return true
		}
	}
	return false
}

// Contains returns true if the key exists in the mapping
func (args Args) Contains(field string) bool {
	_, ok := args.fields[field]
	return ok
}

// Validate compared the set of accepted keys against the keys in the mapping.
// An error is returned if any mapping keys are not in the accepted set.
func (args Args) Validate(accepted map[string]bool) error {
	for name := range args.fields {
		if !accepted[name] {
			return &invalidFilter{name, nil}
		}
	}
	return nil
}

// WalkValues iterates over the list of values for a key in the mapping and calls
// op() for each value. If op returns an error the iteration stops and the
// error is returned.
func (args Args) WalkValues(field string, op func(value string) error) error {
	if _, ok := args.fields[field]; !ok {
		return nil
	}
	for v := range args.fields[field] {
		if err := op(v); err != nil {
			return err
		}
	}
	return nil
}

// Clone returns a copy of args.
func (args Args) Clone() (newArgs Args) {
	newArgs.fields = make(map[string]map[string]bool, len(args.fields))
	for k, m := range args.fields {
		var mm map[string]bool
		if m != nil {
			mm = make(map[string]bool, len(m))
			for kk, v := range m {
				mm[kk] = v
			}
		}
		newArgs.fields[k] = mm
	}
	return newArgs
}

func deprecatedArgs(d map[string][]string) map[string]map[string]bool {
	m := map[string]map[string]bool{}
	for k, v := range d {
		values := map[string]bool{}
		for _, vv := range v {
			values[vv] = true
		}
		m[k] = values
	}
	return m
}

func convertArgsToSlice(f map[string]map[string]bool) map[string][]string {
	m := map[string][]string{}
	for k, v := range f {
		values := []string{}
		for kk := range v {
			if v[kk] {
				values = append(values, kk)
			}
		}
		m[k] = values
	}
	return m
}
