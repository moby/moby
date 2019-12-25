package requirement // import "github.com/docker/docker/integration-cli/requirement"

import (
	"path"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Test represent a function that can be used as a requirement validation.
type Test func() bool

// Is checks if the environment satisfies the requirements
// for the test to run or skips the tests.
func Is(t *testing.T, requirements ...Test) {
	t.Helper()
	for _, r := range requirements {
		isValid := r()
		if !isValid {
			requirementFunc := runtime.FuncForPC(reflect.ValueOf(r).Pointer()).Name()
			t.Skipf("unmatched requirement %s", extractRequirement(requirementFunc))
		}
	}
}

func extractRequirement(requirementFunc string) string {
	requirement := path.Base(requirementFunc)
	return strings.SplitN(requirement, ".", 2)[1]
}
