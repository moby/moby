package log

import (
	"bytes"
	"regexp"

	"testing"
)

var reRFC3339NanoFixed = "[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}.[0-9]{9}.([0-9]{2}:[0-9]{2})?"

func TestLogFatalf(t *testing.T) {
	var output *bytes.Buffer

	tests := []struct {
		Level           priority
		Format          string
		Values          []interface{}
		ExpectedPattern string
	}{
		{fatalPriority, "%d + %d = %d", []interface{}{1, 1, 2}, "\\[" + reRFC3339NanoFixed + "\\] \\[fatal\\] testing.go:\\d+ 1 \\+ 1 = 2"},
		{errorPriority, "%d + %d = %d", []interface{}{1, 1, 2}, "\\[" + reRFC3339NanoFixed + "\\] \\[error\\] testing.go:\\d+ 1 \\+ 1 = 2"},
		{infoPriority, "%d + %d = %d", []interface{}{1, 1, 2}, "\\[" + reRFC3339NanoFixed + "\\] \\[info\\] 1 \\+ 1 = 2"},
		{debugPriority, "%d + %d = %d", []interface{}{1, 1, 2}, "\\[" + reRFC3339NanoFixed + "\\] \\[debug\\] testing.go:\\d+ 1 \\+ 1 = 2"},
	}

	for i, test := range tests {
		output = &bytes.Buffer{}
		DefaultLogger.logf(output, test.Level, test.Format, test.Values...)

		expected := regexp.MustCompile(test.ExpectedPattern)
		if !expected.MatchString(output.String()) {
			t.Errorf("[%d] Log output does not match expected pattern:\n\tExpected: %s\n\tOutput: %s",
				i,
				expected.String(),
				output.String())
		}
	}
}
