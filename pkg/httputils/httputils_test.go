package httputils

import "testing"

func TestDownload(t *testing.T) {
	_, err := Download("http://docker.com")

	if err != nil {
		t.Fatalf("Expected error to not exist when Download(http://docker.com)")
	}

	// Expected status code = 404
	if _, err = Download("http://docker.com/abc1234567"); err == nil {
		t.Fatalf("Expected error to exist when Download(http://docker.com/abc1234567)")
	}
}

func TestNewHTTPRequestError(t *testing.T) {
	errorMessage := "Some error message"
	httpResponse, _ := Download("http://docker.com")
	if err := NewHTTPRequestError(errorMessage, httpResponse); err.Error() != errorMessage {
		t.Fatalf("Expected err to equal error Message")
	}
}

func TestParseServerHeader(t *testing.T) {
	inputs := map[string][]string{
		"bad header":           {"error"},
		"(bad header)":         {"error"},
		"(without/spaces)":     {"error"},
		"(header/with spaces)": {"error"},
		"foo/bar (baz)":        {"foo", "bar", "baz"},
		"foo/bar":              {"error"},
		"foo":                  {"error"},
		"foo/bar (baz space)":           {"foo", "bar", "baz space"},
		"  f  f  /  b  b  (  b  s  )  ": {"f  f", "b  b", "b  s"},
		"foo/bar (baz) ignore":          {"foo", "bar", "baz"},
		"foo/bar ()":                    {"error"},
		"foo/bar()":                     {"error"},
		"foo/bar(baz)":                  {"foo", "bar", "baz"},
		"foo/bar/zzz(baz)":              {"foo/bar", "zzz", "baz"},
		"foo/bar(baz/abc)":              {"foo", "bar", "baz/abc"},
		"foo/bar(baz (abc))":            {"foo", "bar", "baz (abc)"},
	}

	for header, values := range inputs {
		serverHeader, err := ParseServerHeader(header)
		if err != nil {
			if err != errInvalidHeader {
				t.Fatalf("Failed to parse %q, and got some unexpected error: %q", header, err)
			}
			if values[0] == "error" {
				continue
			}
			t.Fatalf("Header %q failed to parse when it shouldn't have", header)
		}
		if values[0] == "error" {
			t.Fatalf("Header %q parsed ok when it should have failed(%q).", header, serverHeader)
		}

		if serverHeader.App != values[0] {
			t.Fatalf("Expected serverHeader.App for %q to equal %q, got %q", header, values[0], serverHeader.App)
		}

		if serverHeader.Ver != values[1] {
			t.Fatalf("Expected serverHeader.Ver for %q to equal %q, got %q", header, values[1], serverHeader.Ver)
		}

		if serverHeader.OS != values[2] {
			t.Fatalf("Expected serverHeader.OS for %q to equal %q, got %q", header, values[2], serverHeader.OS)
		}

	}

}
