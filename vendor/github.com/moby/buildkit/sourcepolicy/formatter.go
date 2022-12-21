package sourcepolicy

import (
	"regexp"

	spb "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/util/wildcard"
	"github.com/pkg/errors"
)

// Source wraps a a protobuf source in order to store cached state such as the compiled regexes.
type selectorCache struct {
	*spb.Selector

	re *regexp.Regexp
	w  *wildcardCache
}

// Format formats the provided ref according to the match/type of the source.
//
// For example, if the source is a wildcard, the ref will be formatted with the wildcard in the source replacing the parameters in the destination.
//
//	matcher: wildcard source: "docker.io/library/golang:*"  match: "docker.io/library/golang:1.19" format: "docker.io/library/golang:${1}-alpine" result: "docker.io/library/golang:1.19-alpine"
func (s *selectorCache) Format(match, format string) (string, error) {
	switch s.MatchType {
	case spb.MatchType_EXACT:
		return s.Identifier, nil
	case spb.MatchType_REGEX:
		re, err := s.regex()
		if err != nil {
			return "", err
		}
		return re.ReplaceAllString(match, format), nil
	case spb.MatchType_WILDCARD:
		w, err := s.wildcard()
		if err != nil {
			return "", err
		}
		m := w.Match(match)
		if m == nil {
			return match, nil
		}

		return m.Format(format)
	}
	return "", errors.Errorf("unknown match type: %s", s.MatchType)
}

// wildcardCache wraps a wildcard.Wildcard to cache returned matches by ref.
// This way a match only needs to be computed once per ref.
type wildcardCache struct {
	w *wildcard.Wildcard
	m map[string]*wildcard.Match
}

func (w *wildcardCache) Match(ref string) *wildcard.Match {
	if w.m == nil {
		w.m = make(map[string]*wildcard.Match)
	}

	if m, ok := w.m[ref]; ok {
		return m
	}

	m := w.w.Match(ref)
	w.m[ref] = m
	return m
}

func (s *selectorCache) wildcard() (*wildcardCache, error) {
	if s.w != nil {
		return s.w, nil
	}
	w, err := wildcard.New(s.Identifier)
	if err != nil {
		return nil, err
	}
	s.w = &wildcardCache{w: w}
	return s.w, nil
}

func (s *selectorCache) regex() (*regexp.Regexp, error) {
	if s.re != nil {
		return s.re, nil
	}
	re, err := regexp.Compile(s.Identifier)
	if err != nil {
		return nil, err
	}
	s.re = re
	return re, nil
}
