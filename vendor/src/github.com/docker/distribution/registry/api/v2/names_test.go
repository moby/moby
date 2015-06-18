package v2

import (
	"strings"
	"testing"
)

func TestRepositoryNameRegexp(t *testing.T) {
	for _, testcase := range []struct {
		input string
		err   error
	}{
		{
			input: "short",
		},
		{
			input: "simple/name",
		},
		{
			input: "library/ubuntu",
		},
		{
			input: "docker/stevvooe/app",
		},
		{
			input: "aa/aa/aa/aa/aa/aa/aa/aa/aa/bb/bb/bb/bb/bb/bb",
		},
		{
			input: "aa/aa/bb/bb/bb",
		},
		{
			input: "a/a/a/b/b",
			err:   ErrRepositoryNameComponentShort,
		},
		{
			input: "a/a/a/a/",
			err:   ErrRepositoryNameComponentShort,
		},
		{
			input: "foo.com/bar/baz",
		},
		{
			input: "blog.foo.com/bar/baz",
		},
		{
			input: "asdf",
		},
		{
			input: "asdf$$^/aa",
			err:   ErrRepositoryNameComponentInvalid,
		},
		{
			input: "aa-a/aa",
		},
		{
			input: "aa/aa",
		},
		{
			input: "a-a/a-a",
		},
		{
			input: "a",
			err:   ErrRepositoryNameComponentShort,
		},
		{
			input: "a-/a/a/a",
			err:   ErrRepositoryNameComponentInvalid,
		},
		{
			input: strings.Repeat("a", 255),
		},
		{
			input: strings.Repeat("a", 256),
			err:   ErrRepositoryNameLong,
		},
	} {

		failf := func(format string, v ...interface{}) {
			t.Logf(testcase.input+": "+format, v...)
			t.Fail()
		}

		if err := ValidateRepositoryName(testcase.input); err != testcase.err {
			if testcase.err != nil {
				if err != nil {
					failf("unexpected error for invalid repository: got %v, expected %v", err, testcase.err)
				} else {
					failf("expected invalid repository: %v", testcase.err)
				}
			} else {
				if err != nil {
					// Wrong error returned.
					failf("unexpected error validating repository name: %v, expected %v", err, testcase.err)
				} else {
					failf("unexpected error validating repository name: %v", err)
				}
			}
		}
	}
}
