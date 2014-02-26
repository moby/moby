package filters

import (
	"errors"
	"strings"
)

/*
Parse the argument to the filter flag. Like

  `docker ps -f 'created=today;image.name=ubuntu*'`

Filters delimited by ';', and expected to be 'name=value'

If prev map is provided, then it is appended to, and returned. By default a new
map is created.
*/
func ParseFlag(arg string, prev map[string]string) (map[string]string, error) {
	var filters map[string]string
	if prev != nil {
		filters = prev
	} else {
		filters = map[string]string{}
	}
  if len(arg) == 0 {
    return filters, nil
  }

	for _, chunk := range strings.Split(arg, ";") {
		if !strings.Contains(chunk, "=") {
			return filters, ErrorBadFormat
		}
		f := strings.SplitN(chunk, "=", 2)
		filters[f[0]] = f[1]
	}
	return filters, nil
}

var ErrorBadFormat = errors.New("bad format of filter (expected name=value)")

func ToParam(f map[string]string) string {
	fs := []string{}
	for k, v := range f {
		fs = append(fs, k+"="+v)
	}
	return strings.Join(fs, ";")
}
