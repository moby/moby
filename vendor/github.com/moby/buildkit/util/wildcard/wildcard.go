package wildcard

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// New returns a wildcard object for a string that contains "*" symbols.
func New(s string) (*Wildcard, error) {
	reStr, err := Wildcard2Regexp(s)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to translate wildcard %q to regexp", s)
	}
	re, err := regexp.Compile(reStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compile regexp %q (translated from wildcard %q)", reStr, s)
	}
	w := &Wildcard{
		orig: s,
		re:   re,
	}
	return w, nil
}

// Wildcard2Regexp translates a wildcard string to a regexp string.
func Wildcard2Regexp(wildcard string) (string, error) {
	s := regexp.QuoteMeta(wildcard)
	if strings.Contains(s, "\\*\\*") {
		return "", errors.New("invalid wildcard: \"**\"")
	}
	s = strings.ReplaceAll(s, "\\*", "(.*)")
	s = "^" + s + "$"
	return s, nil
}

// Wildcard is a wildcard matcher object.
type Wildcard struct {
	orig string
	re   *regexp.Regexp
}

// String implements fmt.Stringer.
func (w *Wildcard) String() string {
	return w.orig
}

// Match returns a non-nil Match on match.
func (w *Wildcard) Match(q string) *Match {
	submatches := w.re.FindStringSubmatch(q)
	if len(submatches) == 0 {
		return nil
	}
	m := &Match{
		w:          w,
		Submatches: submatches,
		// FIXME: avoid executing regexp twice
		idx: w.re.FindStringSubmatchIndex(q),
	}
	return m
}

// Match is a matched result.
type Match struct {
	w          *Wildcard
	Submatches []string // 0: the entire query, 1: the first submatch, 2: the second submatch, ...
	idx        []int
}

// String implements fmt.Stringer.
func (m *Match) String() string {
	if len(m.Submatches) == 0 {
		return ""
	}
	return m.Submatches[0]
}

// Format formats submatch strings like "$1", "$2".
func (m *Match) Format(f string) (string, error) {
	if m.w == nil || len(m.Submatches) == 0 || len(m.idx) == 0 {
		return "", errors.New("invalid state")
	}
	var b []byte
	b = m.w.re.ExpandString(b, f, m.Submatches[0], m.idx)
	return string(b), nil
}
