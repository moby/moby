package daemon

import (
	"path/filepath"
	"testing"
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
