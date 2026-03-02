package sourcepolicy

import (
	"regexp"
	"sync"

	spb "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/util/wildcard"
	"github.com/pkg/errors"
)

// selectorCache wraps a protobuf selector in order to store cached state such as the compiled regexes.
type selectorCache struct {
	*spb.Selector
	regex    func() (*regexp.Regexp, error)
	wildcard func() (*wildcardCache, error)
}

func newSelectorCache(sel *spb.Selector) *selectorCache {
	s := &selectorCache{Selector: sel}
	s.regex = sync.OnceValues(func() (*regexp.Regexp, error) {
		return regexp.Compile(sel.Identifier)
	})
	s.wildcard = sync.OnceValues(func() (*wildcardCache, error) {
		w, err := wildcard.New(sel.Identifier)
		if err != nil {
			return nil, err
		}
		return &wildcardCache{w: w}, nil
	})
	return s
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
	mu sync.Mutex
	w  *wildcard.Wildcard
	m  map[string]*wildcard.Match
}

func (w *wildcardCache) Match(ref string) *wildcard.Match {
	w.mu.Lock()
	defer w.mu.Unlock()

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
