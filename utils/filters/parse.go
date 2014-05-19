package filters

import (
	"errors"
	"github.com/dotcloud/docker/pkg/beam/data"
	"strings"
)

type Args map[string][]string

/*
Parse the argument to the filter flag. Like

  `docker ps -f 'created=today' -f 'image.name=ubuntu*'`

If prev map is provided, then it is appended to, and returned. By default a new
map is created.
*/
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
	filters[f[0]] = append(filters[f[0]], f[1])

	return filters, nil
}

var ErrorBadFormat = errors.New("bad format of filter (expected name=value)")

/*
packs the Args into an string for easy transport from client to server
*/
func ToParam(a Args) string {
	return data.Encode(a)
}

/*
unpacks the filter Args
*/
func FromParam(p string) (Args, error) {
	return data.Decode(p)
}
