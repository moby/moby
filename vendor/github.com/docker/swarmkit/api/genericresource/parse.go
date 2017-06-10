package genericresource

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/swarmkit/api"
)

func newParseError(format string, args ...interface{}) error {
	return fmt.Errorf("could not parse GenericResource: "+format, args...)
}

// Parse parses the GenericResource resources given by the arguments
func Parse(cmd string) ([]*api.GenericResource, error) {
	var rs []*api.GenericResource

	for _, term := range strings.Split(cmd, ";") {
		kva := strings.Split(term, "=")
		if len(kva) != 2 {
			return nil, newParseError("incorrect term %s, missing '=' or malformed expr", term)
		}

		key := strings.TrimSpace(kva[0])
		val := strings.TrimSpace(kva[1])

		u, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			if u < 0 {
				return nil, newParseError("cannot ask for negative resource %s", key)
			}
			rs = append(rs, NewDiscrete(key, u))
			continue
		}

		if len(val) > 2 && val[0] == '{' && val[len(val)-1] == '}' {
			val = val[1 : len(val)-1]
			rs = append(rs, NewSet(key, strings.Split(val, ",")...)...)
			continue
		}

		return nil, newParseError("could not parse expression '%s'", term)
	}

	return rs, nil
}
