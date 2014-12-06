package filters

import (
	"encoding/json"
	"errors"
)

// ParseArg returns an Arg from an opaque str parameter
func ParseArg(str string) (Arg, error) {
	items, err := SplitByOperators(str)
	if err != nil {
		return Arg{}, err
	}
	return Arg{Key: items[0], Operator: items[1], Value: items[2]}, nil
}

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

	a, err := ParseArg(arg)
	if err != nil {
		return filters, err
	}
	filters = append(filters, a)

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
