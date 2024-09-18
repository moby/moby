package suggest

import (
	"strings"

	"github.com/agext/levenshtein"
)

func Search(val string, options []string, caseSensitive bool) (string, bool) {
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
			return "", false
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
	return match, match != ""
}

// WrapError wraps error with a suggestion for fixing it
func WrapError(err error, val string, options []string, caseSensitive bool) error {
	_, err = WrapErrorMaybe(err, val, options, caseSensitive)
	return err
}

func WrapErrorMaybe(err error, val string, options []string, caseSensitive bool) (bool, error) {
	if err == nil {
		return false, nil
	}
	match, ok := Search(val, options, caseSensitive)
	if match == "" || !ok {
		return false, err
	}

	return true, &suggestError{
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
