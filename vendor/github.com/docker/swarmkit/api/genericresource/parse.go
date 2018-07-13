package genericresource

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/swarmkit/api"
)

func newParseError(format string, args ...interface{}) error {
	return fmt.Errorf("could not parse GenericResource: "+format, args...)
}

// discreteResourceVal returns an int64 if the string is a discreteResource
// and an error if it isn't
func discreteResourceVal(res string) (int64, error) {
	return strconv.ParseInt(res, 10, 64)
}

// allNamedResources returns true if the array of resources are all namedResources
// e.g: res = [red, orange, green]
func allNamedResources(res []string) bool {
	for _, v := range res {
		if _, err := discreteResourceVal(v); err == nil {
			return false
		}
	}

	return true
}

// ParseCmd parses the Generic Resource command line argument
// and returns a list of *api.GenericResource
func ParseCmd(cmd string) ([]*api.GenericResource, error) {
	if strings.Contains(cmd, "\n") {
		return nil, newParseError("unexpected '\\n' character")
	}

	r := csv.NewReader(strings.NewReader(cmd))
	records, err := r.ReadAll()

	if err != nil {
		return nil, newParseError("%v", err)
	}

	if len(records) != 1 {
		return nil, newParseError("found multiple records while parsing cmd %v", records)
	}

	return Parse(records[0])
}

// Parse parses a table of GenericResource resources
func Parse(cmds []string) ([]*api.GenericResource, error) {
	tokens := make(map[string][]string)

	for _, term := range cmds {
		kva := strings.Split(term, "=")
		if len(kva) != 2 {
			return nil, newParseError("incorrect term %s, missing"+
				" '=' or malformed expression", term)
		}

		key := strings.TrimSpace(kva[0])
		val := strings.TrimSpace(kva[1])

		tokens[key] = append(tokens[key], val)
	}

	var rs []*api.GenericResource
	for k, v := range tokens {
		if u, ok := isDiscreteResource(v); ok {
			if u < 0 {
				return nil, newParseError("cannot ask for"+
					" negative resource %s", k)
			}

			rs = append(rs, NewDiscrete(k, u))
			continue
		}

		if allNamedResources(v) {
			rs = append(rs, NewSet(k, v...)...)
			continue
		}

		return nil, newParseError("mixed discrete and named resources"+
			" in expression '%s=%s'", k, v)
	}

	return rs, nil
}

// isDiscreteResource returns true if the array of resources is a
// Discrete Resource.
// e.g: res = [1]
func isDiscreteResource(values []string) (int64, bool) {
	if len(values) != 1 {
		return int64(0), false
	}

	u, err := discreteResourceVal(values[0])
	if err != nil {
		return int64(0), false
	}

	return u, true

}
