// +build !windows

package dockerfile

import (
	"runtime"
	"testing"
)

func TestNormalizeWorkdir(t *testing.T) {
	testCases := []struct{ current, requested, expected, expectedError string }{
		{``, ``, ``, `cannot normalize nothing`},
		{``, `foo`, `/foo`, ``},
		{``, `/foo`, `/foo`, ``},
		{`/foo`, `bar`, `/foo/bar`, ``},
		{`/foo`, `/bar`, `/bar`, ``},
	}

	for _, test := range testCases {
		normalized, err := normalizeWorkdir(runtime.GOOS, test.current, test.requested)

		if test.expectedError != "" && err == nil {
			t.Fatalf("NormalizeWorkdir should return an error %s, got nil", test.expectedError)
		}

		if test.expectedError != "" && err.Error() != test.expectedError {
			t.Fatalf("NormalizeWorkdir returned wrong error. Expected %s, got %s", test.expectedError, err.Error())
		}

		if normalized != test.expected {
			t.Fatalf("NormalizeWorkdir error. Expected %s for current %s and requested %s, got %s", test.expected, test.current, test.requested, normalized)
		}
	}
}
