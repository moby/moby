// +build !windows

package dockerfile

import "github.com/go-check/check"

func (s *DockerSuite) TestNormaliseWorkdir(c *check.C) {
	testCases := []struct{ current, requested, expected, expectedError string }{
		{``, ``, ``, `cannot normalise nothing`},
		{``, `foo`, `/foo`, ``},
		{``, `/foo`, `/foo`, ``},
		{`/foo`, `bar`, `/foo/bar`, ``},
		{`/foo`, `/bar`, `/bar`, ``},
	}

	for _, test := range testCases {
		normalised, err := normaliseWorkdir(test.current, test.requested)

		if test.expectedError != "" && err == nil {
			c.Fatalf("NormaliseWorkdir should return an error %s, got nil", test.expectedError)
		}

		if test.expectedError != "" && err.Error() != test.expectedError {
			c.Fatalf("NormaliseWorkdir returned wrong error. Expected %s, got %s", test.expectedError, err.Error())
		}

		if normalised != test.expected {
			c.Fatalf("NormaliseWorkdir error. Expected %s for current %s and requested %s, got %s", test.expected, test.current, test.requested, normalised)
		}
	}
}
