package opts // import "github.com/docker/docker/opts"

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateEnv(t *testing.T) {
	type testCase struct {
		value    string
		expected string
		err      error
	}
	tests := []testCase{
		{
			value:    "a",
			expected: "a",
		},
		{
			value:    "something",
			expected: "something",
		},
		{
			value:    "_=a",
			expected: "_=a",
		},
		{
			value:    "env1=value1",
			expected: "env1=value1",
		},
		{
			value:    "_env1=value1",
			expected: "_env1=value1",
		},
		{
			value:    "env2=value2=value3",
			expected: "env2=value2=value3",
		},
		{
			value:    "env3=abc!qwe",
			expected: "env3=abc!qwe",
		},
		{
			value:    "env_4=value 4",
			expected: "env_4=value 4",
		},
		{
			value:    "PATH",
			expected: fmt.Sprintf("PATH=%v", os.Getenv("PATH")),
		},
		{
			value: "=a",
			err:   fmt.Errorf("invalid environment variable: =a"),
		},
		{
			value:    "PATH=",
			expected: "PATH=",
		},
		{
			value:    "PATH=something",
			expected: "PATH=something",
		},
		{
			value:    "asd!qwe",
			expected: "asd!qwe",
		},
		{
			value:    "1asd",
			expected: "1asd",
		},
		{
			value:    "123",
			expected: "123",
		},
		{
			value:    "some space",
			expected: "some space",
		},
		{
			value:    "  some space before",
			expected: "  some space before",
		},
		{
			value:    "some space after  ",
			expected: "some space after  ",
		},
		{
			value: "=",
			err:   fmt.Errorf("invalid environment variable: ="),
		},
	}

	if runtime.GOOS == "windows" {
		// Environment variables are case in-sensitive on Windows
		tests = append(tests, testCase{
			value:    "PaTh",
			expected: fmt.Sprintf("PaTh=%v", os.Getenv("PATH")),
			err:      nil,
		})
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			actual, err := ValidateEnv(tc.value)

			if tc.err == nil {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.err.Error())
			}
			assert.Equal(t, actual, tc.expected)
		})
	}
}
