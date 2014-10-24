package log

import (
	"bytes"
	"regexp"

	"testing"
)

func TestLogFatalf(t *testing.T) {
	var output *bytes.Buffer

	tests := []struct {
		Level           priority
		Format          string
		Values          []interface{}
		ExpectedPattern string
	}{
		{fatal, "%d + %d = %d", []interface{}{1, 1, 2}, "\\[fatal\\] testing.go:\\d+ 1 \\+ 1 = 2"},
		{error, "%d + %d = %d", []interface{}{1, 1, 2}, "\\[error\\] testing.go:\\d+ 1 \\+ 1 = 2"},
		{info, "%d + %d = %d", []interface{}{1, 1, 2}, "\\[info\\] 1 \\+ 1 = 2"},
		{debug, "%d + %d = %d", []interface{}{1, 1, 2}, "\\[debug\\] testing.go:\\d+ 1 \\+ 1 = 2"},
	}

	for i, test := range tests {
		output = &bytes.Buffer{}
		logf(output, test.Level, test.Format, test.Values...)

		expected := regexp.MustCompile(test.ExpectedPattern)
		if !expected.MatchString(output.String()) {
			t.Errorf("[%d] Log output does not match expected pattern:\n\tExpected: %s\n\tOutput: %s",
				i,
				expected.String(),
				output.String())
		}
	}
}
