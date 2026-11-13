package pflag

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
)

// -- stringToString Value
type stringToStringValue struct {
	value   *map[string]string
	changed bool
}

func newStringToStringValue(val map[string]string, p *map[string]string) *stringToStringValue {
	ssv := new(stringToStringValue)
	ssv.value = p
	*ssv.value = val
	return ssv
}

// Set updates the flag value from the given string, adding additional mappings or updating existing ones.
func (s *stringToStringValue) Set(val string) error {
	var ss []string
	n := strings.Count(val, "=")
	switch n {
	case 0:
		return fmt.Errorf("%s must be formatted as key=value", val)
	case 1:
		ss = append(ss, strings.Trim(val, `"`))
	default:
		r := csv.NewReader(strings.NewReader(val))
		var err error
		ss, err = r.Read()
		if err != nil {
			return err
		}
		if _, err := r.Read(); err != io.EOF {
			return fmt.Errorf("must be a single CSV record; quote fields containing newlines")
		}
	}

	out := make(map[string]string, len(ss))
	for _, pair := range ss {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("%s must be formatted as key=value", pair)
		}
		out[kv[0]] = kv[1]
	}

	// clear out any default flag values
	if !s.changed {
		for k := range *s.value {
			delete(*s.value, k)
		}
	}

	if *s.value == nil {
		*s.value = make(map[string]string, len(out))
	}
	for k, v := range out {
		(*s.value)[k] = v
	}
	s.changed = true
	return nil
}

func (s *stringToStringValue) Type() string {
	return "stringToString"
}

func (s *stringToStringValue) String() string {
	keys := make([]string, 0, len(*s.value))
	for k := range *s.value {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	records := make([]string, 0, len(*s.value)>>1)
	for _, k := range keys {
		v := (*s.value)[k]
		records = append(records, k+"="+v)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(records); err != nil {
		panic(err)
	}
	w.Flush()
	return "[" + strings.TrimSuffix(buf.String(), "\n") + "]"
}

// GetStringToString return the map value of a flag with the given name from f. The returned map shares memory with the
// internal flag value [Flag.Value].
func (f *FlagSet) GetStringToString(name string) (map[string]string, error) {
	val, err := f.getFlagType(name, "stringToString", nil)
	if err != nil {
		return map[string]string{}, err
	}

	fv, ok := val.(*stringToStringValue)
	if !ok {
		panic(fmt.Errorf("illegal state: unspected internal type for stringToString flag '%s'", name))
	}
	if fv.value == nil {
		return nil, nil
	}

	return *fv.value, nil
}

// StringToString defines a map flag with specified name, default value, and usage string.
//
// StringToString flags are used to pass key=value pairs to applications. The same flag can be provided more than once
// with all key=value pairs being merged into a final map. Multiple key=value pairs may be provided in a single arg,
// separated by commas. A few simple examples include:
//
//	--arg a=1           -> map[string]string{ "a": "1" }
//	--arg a=1 --arg b=2 -> map[string]string{ "a": "1", "b": "2" }
//	--arg a=1,b=2       -> map[string]string{ "a": "1", "b": "2" }
//	--arg=a=1           -> map[string]string{ "a": "1" }
//
// As a special case, a single key=value pair whose value contains a comma will be interpreted as shown below:
//
//	--arg a=1,2         -> map[string]string{ "a": "1,2" }
//
// Arguments containing multiple '=' characters are parsed as one CSV record.
// Quote the entire key=value field to preserve commas, quotes, or newlines in
// its value. When invoking a command from a shell, preserve the CSV quotes,
// for example by enclosing the argument in single quotes:
//
//	--arg '"a=first,second=value"' -> map[string]string{ "a": "first,second=value" }
//
// Multiple unquoted CSV records in one argument are rejected rather than
// silently discarding mappings after the first record.
//
// Returns a pointer to the map which will be updated upon invocation of [FlagSet.Parse], [Flag.Value.Set], and others.
func (f *FlagSet) StringToString(name string, value map[string]string, usage string) *map[string]string {
	p := map[string]string{}
	f.StringToStringVarP(&p, name, "", value, usage)
	return &p
}

// StringToStringP is like [FlagSet.StringToString], but also accepts a shorthand letter that can be used after a single
// dash.
//
// See [FlagSet.StringToString].
func (f *FlagSet) StringToStringP(name, shorthand string, value map[string]string, usage string) *map[string]string {
	p := map[string]string{}
	f.StringToStringVarP(&p, name, shorthand, value, usage)
	return &p
}

// StringToStringVar is like [FlagSet.StringToString], but also accepts a map pointer argument p which is updated with
// the parsed key-value pairs.
//
// See [FlagSet.StringToString].
func (f *FlagSet) StringToStringVar(p *map[string]string, name string, value map[string]string, usage string) {
	f.VarP(newStringToStringValue(value, p), name, "", usage)
}

// StringToStringVarP is like [FlagSet.StringToString], but also accepts a map pointer argument p which is updated with
// the parsed key-value pairs, and a shorthand letter that can be used after a single dash.
//
// See [FlagSet.StringToString].
func (f *FlagSet) StringToStringVarP(p *map[string]string, name, shorthand string, value map[string]string, usage string) {
	f.VarP(newStringToStringValue(value, p), name, shorthand, usage)
}

// StringToString defines a string flag with specified name, default value, and usage string.
//
// See [FlagSet.StringToString].
func StringToString(name string, value map[string]string, usage string) *map[string]string {
	return CommandLine.StringToStringP(name, "", value, usage)
}

// StringToStringP is like [FlagSet.StringToString], but also accepts a shorthand letter that can be used after a single
// dash.
//
// See [FlagSet.StringToString].
func StringToStringP(name, shorthand string, value map[string]string, usage string) *map[string]string {
	return CommandLine.StringToStringP(name, shorthand, value, usage)
}

// StringToStringVar is like [FlagSet.StringToString], but also accepts a map pointer argument p which is updated with
// the parsed key-value pairs.
//
// See [FlagSet.StringToString].
func StringToStringVar(p *map[string]string, name string, value map[string]string, usage string) {
	CommandLine.VarP(newStringToStringValue(value, p), name, "", usage)
}

// StringToStringVarP is like [FlagSet.StringToString], but also accepts a map pointer argument p which is updated with
// the parsed key-value pairs, and a shorthand letter that can be used after a single dash.
//
// See [FlagSet.StringToString].
func StringToStringVarP(p *map[string]string, name, shorthand string, value map[string]string, usage string) {
	CommandLine.VarP(newStringToStringValue(value, p), name, shorthand, usage)
}
