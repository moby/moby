package daemon

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// regression test for https://github.com/moby/moby/issues/52300
func TestSanitizeTestName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "shouldn't do something",
			want: "TestSanitizeTestName/shouldn_t_do_something",
		},
		{
			name: `double "quotes"`,
			want: "TestSanitizeTestName/double__quotes_",
		},
		{
			name: "contains spaces",
			want: "TestSanitizeTestName/contains_spaces",
		},
		{
			name: "contains ..dots",
			want: "TestSanitizeTestName/contains___dots",
		},
		{
			name: "..starts-with-dots",
			want: "TestSanitizeTestName/__starts-with-dots",
		},
		{
			name: ".starts-with-dot",
			want: "TestSanitizeTestName/_starts-with-dot",
		},
		{
			name: "ends-with-dot.",
			want: "TestSanitizeTestName/ends-with-dot_",
		},
		{
			name: "--starts-with-dash",
			want: "TestSanitizeTestName/starts-with-dash",
		},
		{
			name: "_starts-with-underscore",
			want: "TestSanitizeTestName/_starts-with-underscore",
		},
		{
			name: "ends-with-dash-",
			want: "TestSanitizeTestName/ends-with-dash-",
		},
		{
			name: "foo/bar",
			want: "TestSanitizeTestName/foo/bar",
		},
		{
			name: `foo/"bar"`,
			want: "TestSanitizeTestName/foo/_bar_",
		},
		{
			name: "foo/..bar",
			want: "TestSanitizeTestName/foo/__bar",
		},
		{
			name: "../foo",
			want: "TestSanitizeTestName/__/foo",
		},
		{
			name: "foo/..",
			want: "TestSanitizeTestName/foo/__",
		},
		{
			name: "'",
			want: "TestSanitizeTestName/_",
		},
		{
			name: "...",
			want: "TestSanitizeTestName/___",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := sanitizedTestName(t)
			want := filepath.FromSlash(tc.want) // let's be nice to Windows.
			if out != want {
				t.Errorf("got %q, want %q", out, want)
			}
		})
	}
}

func TestSetEnvVar(t *testing.T) {
	var d Daemon
	for _, step := range []struct {
		name, val string
		want      []string
	}{
		{name: "FOO", val: "1", want: []string{"FOO=1"}},
		{name: "BAR", val: "2", want: []string{"FOO=1", "BAR=2"}},
		{name: "FOO", val: "3", want: []string{"FOO=3", "BAR=2"}},
		{name: "BAR", val: "4", want: []string{"FOO=3", "BAR=4"}},
		{name: "FOO", val: "", want: []string{"FOO=", "BAR=4"}},
	} {
		d.SetEnvVar(step.name, step.val)
		assert.Check(t, is.DeepEqual(d.extraEnv, step.want), "after SetEnvVar(%q, %q)", step.name, step.val)
	}
}
