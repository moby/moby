package filters

import (
	"encoding/json"
	"errors"
	"github.com/docker/docker/opts"
	"regexp"
	"strings"
)

type Args map[string][]string

// Parse the argument to the filter flag. Like
//
//   `docker ps -f 'created=today' -f 'image.name=ubuntu*'`
//
// If prev map is provided, then it is appended to, and returned. By default a new
// map is created.
func ParseFlag(arg string, prev Args) (Args, error) {
	var filters Args = prev
	if prev == nil {
		filters = Args{}
	}
	if len(arg) == 0 {
		return filters, nil
	}

	if !strings.Contains(arg, "=") {
		return filters, ErrorBadFormat
	}

	f := strings.SplitN(arg, "=", 2)
	name := strings.ToLower(strings.TrimSpace(f[0]))
	value := strings.TrimSpace(f[1])
	filters[name] = append(filters[name], value)

	return filters, nil
}

var ErrorBadFormat = errors.New("bad format of filter (expected name=value)")

// packs the Args into an string for easy transport from client to server
func ToParam(a Args) (string, error) {
	// this way we don't URL encode {}, just empty space
	if len(a) == 0 {
		return "", nil
	}

	buf, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// unpacks the filter Args
func FromParam(p string) (Args, error) {
	args := Args{}
	if len(p) == 0 {
		return args, nil
	}
	err := json.Unmarshal([]byte(p), &args)
	if err != nil {
		return nil, err
	}
	return args, nil
}

func (filters Args) Match(field, source string) bool {
	fieldValues := filters[field]

	//do not filter if there is no filter set or cannot determine filter
	if len(fieldValues) == 0 {
		return true
	}
	for _, name2match := range fieldValues {
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

// Consolidate all filter flags, and sanity check them early.
// They'll get process in the daemon/server.
func (args Args) GetAsJSON(opts opts.ListOpts) (string, error) {
	for _, f := range opts.GetAll() {
		var err error
		args, err = ParseFlag(f, args)
		if err != nil {
			return "", err
		}
	}
	if len(args) > 0 {
		return ToParam(args)
	}
	return "", nil
}
