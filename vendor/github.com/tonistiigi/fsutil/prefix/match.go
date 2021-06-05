package prefix

import (
	"path"
	"path/filepath"
	"strings"
)

// Match matches a path against a pattern. It returns m = true if the path
// matches the pattern, and partial = true if the pattern has more separators
// than the path and the common components match (for example, name = foo and
// pattern = foo/bar/*). slashSeparator determines whether the path and pattern
// are '/' delimited (true) or use the native path separator (false).
func Match(pattern, name string, slashSeparator bool) (m bool, partial bool) {
	separator := filepath.Separator
	if slashSeparator {
		separator = '/'
	}
	count := strings.Count(name, string(separator))
	if strings.Count(pattern, string(separator)) > count {
		pattern = trimUntilIndex(pattern, string(separator), count)
		partial = true
	}
	if slashSeparator {
		m, _ = path.Match(pattern, name)
	} else {
		m, _ = filepath.Match(pattern, name)
	}
	return m, partial
}

func trimUntilIndex(str, sep string, count int) string {
	s := str
	i := 0
	c := 0
	for {
		idx := strings.Index(s, sep)
		s = s[idx+len(sep):]
		i += idx + len(sep)
		c++
		if c > count {
			return str[:i-len(sep)]
		}
	}
}
