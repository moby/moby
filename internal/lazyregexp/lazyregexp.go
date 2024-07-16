// Package lazyregexp provides utilities to define lazily compiled regular
// expressions.
package lazyregexp

import (
	"regexp"
	"sync"
	"testing"
)

// CompileOnce creates a function to compile the given regexp once, delaying
// the compiling work until it is first needed. If the code is being run as
// part of tests, the regexp compiling happens immediately so that regular
// expressions are verified in tests and do not result in a panic at runtime.
func CompileOnce(str string) func() *regexp.Regexp {
	if testing.Testing() {
		re := regexp.MustCompile(str)
		return func() *regexp.Regexp { return re }
	}
	return sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(str)
	})
}
