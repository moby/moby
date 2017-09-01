package opts

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

func TestValidateEnv(t *testing.T) {
	testcase := []struct {
		value    string
		expected string
		err      error
	}{
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
			err:   fmt.Errorf(fmt.Sprintf("invalid environment variable: %s", "=a")),
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
			err:   fmt.Errorf(fmt.Sprintf("invalid environment variable: %s", "=")),
		},
	}

	// Environment variables are case in-sensitive on Windows
	if runtime.GOOS == "windows" {
		tmp := struct {
			value    string
			expected string
			err      error
		}{
			value:    "PaTh",
			expected: fmt.Sprintf("PaTh=%v", os.Getenv("PATH")),
		}
		testcase = append(testcase, tmp)

	}

	for _, r := range testcase {
		actual, err := ValidateEnv(r.value)

		if err != nil {
			if r.err == nil {
				t.Fatalf("Expected err is nil, got err[%v]", err)
			}
			if err.Error() != r.err.Error() {
				t.Fatalf("Expected err[%v], got err[%v]", r.err, err)
			}
		}

		if err == nil && r.err != nil {
			t.Fatalf("Expected err[%v], but err is nil", r.err)
		}

		if actual != r.expected {
			t.Fatalf("Expected [%v], got [%v]", r.expected, actual)
		}
	}
}
