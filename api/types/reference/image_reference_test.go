package reference

import (
	"testing"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		ref           string
		expectedName  string
		expectedTag   string
		expectedError bool
	}{
		{
			ref:           "",
			expectedName:  "",
			expectedTag:   "",
			expectedError: true,
		},
		{
			ref:           "repository",
			expectedName:  "repository",
			expectedTag:   "latest",
			expectedError: false,
		},
		{
			ref:           "repository:tag",
			expectedName:  "repository",
			expectedTag:   "tag",
			expectedError: false,
		},
		{
			ref:           "test.com/repository",
			expectedName:  "test.com/repository",
			expectedTag:   "latest",
			expectedError: false,
		},
		{
			ref:           "test.com:5000/test/repository",
			expectedName:  "test.com:5000/test/repository",
			expectedTag:   "latest",
			expectedError: false,
		},
		{
			ref:           "test.com:5000/repo@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			expectedName:  "test.com:5000/repo",
			expectedTag:   "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			expectedError: false,
		},
		{
			ref:           "test.com:5000/repo:tag@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			expectedName:  "test.com:5000/repo",
			expectedTag:   "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			expectedError: false,
		},
	}

	for _, c := range testCases {
		name, tag, err := Parse(c.ref)
		if err != nil && c.expectedError {
			continue
		} else if err != nil {
			t.Fatalf("error with %s: %s", c.ref, err.Error())
		}
		if name != c.expectedName {
			t.Fatalf("expected name %s, got %s", c.expectedName, name)
		}
		if tag != c.expectedTag {
			t.Fatalf("expected tag %s, got %s", c.expectedTag, tag)
		}
	}
}
