package requirement

import (
	"fmt"
	"reflect"
	"runtime"
)

type skipT interface {
	Skip(reason string)
}

// Test represent a function that can be used as a requirement validation.
type Test func() bool

// Is checks if the environment satisfies the requirements
// for the test to run or skips the tests.
func Is(s skipT, requirements ...Test) {
	for _, r := range requirements {
		isValid := r()
		if !isValid {
			requirementFunc := runtime.FuncForPC(reflect.ValueOf(r).Pointer()).Name()
			s.Skip(fmt.Sprintf("unmatched requirement %s", requirementFunc))
		}
	}
}
