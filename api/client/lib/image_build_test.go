package lib

import "testing"

func TestGetDockerOS(t *testing.T) {
	cases := map[string]string{
		"Docker/v1.22 (linux)":   "linux",
		"Docker/v1.22 (windows)": "windows",
		"Foo/v1.22 (bar)":        "",
	}
	for header, os := range cases {
		g := getDockerOS(header)
		if g != os {
			t.Fatalf("Expected %s, got %s", os, g)
		}
	}
}
