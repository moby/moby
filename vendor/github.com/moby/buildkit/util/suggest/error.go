package suggest

import (
	"strings"

	"github.com/agext/levenshtein"
)

// WrapError wraps error with a suggestion for fixing it
func WrapError(err error, val string, options []string, caseSensitive bool) error {
	if err == nil {
		return nil
	}
	orig := val
	if !caseSensitive {
		val = strings.ToLower(val)
	}
	var match string
	mindist := 3 // same as hcl
	for _, opt := range options {
		if !caseSensitive {
			opt = strings.ToLower(opt)
		}
		if val == opt {
			// exact match means error was unrelated to the value
			return err
		}
		dist := levenshtein.Distance(val, opt, nil)
		if dist < mindist {
			if !caseSensitive {
				match = matchCase(opt, orig)
			} else {
				match = opt
			}
			mindist = dist
		}
	}

	if match == "" {
		return err
	}

	return &suggestError{
		err:   err,
		match: match,
	}
}

type suggestError struct {
	err   error
	match string
}

func (e *suggestError) Error() string {
	return e.err.Error() + " (did you mean " + e.match + "?)"
}

// Unwrap returns the underlying error.
func (e *suggestError) Unwrap() error {
	return e.err
}

func matchCase(val, orig string) string {
	if orig == strings.ToLower(orig) {
		return strings.ToLower(val)
	}
	if orig == strings.ToUpper(orig) {
		return strings.ToUpper(val)
	}
	return val
}
