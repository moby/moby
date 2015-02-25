// +build !windows

package archive

import (
	"testing"
)

func TestCanonicalTarNameForPath(t *testing.T) {
	cases := []struct{ in, expected string }{
		{"foo", "foo"},
		{"foo/bar", "foo/bar"},
		{"foo/dir/", "foo/dir/"},
	}
	for _, v := range cases {
		if out, err := CanonicalTarNameForPath(v.in); err != nil {
			t.Fatalf("cannot get canonical name for path: %s: %v", v.in, err)
		} else if out != v.expected {
			t.Fatalf("wrong canonical tar name. expected:%s got:%s", v.expected, out)
		}
	}
}

func TestCanonicalTarName(t *testing.T) {
	cases := []struct {
		in       string
		isDir    bool
		expected string
	}{
		{"foo", false, "foo"},
		{"foo", true, "foo/"},
		{"foo/bar", false, "foo/bar"},
		{"foo/bar", true, "foo/bar/"},
	}
	for _, v := range cases {
		if out, err := canonicalTarName(v.in, v.isDir); err != nil {
			t.Fatalf("cannot get canonical name for path: %s: %v", v.in, err)
		} else if out != v.expected {
			t.Fatalf("wrong canonical tar name. expected:%s got:%s", v.expected, out)
		}
	}
}
