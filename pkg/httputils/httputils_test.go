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
	if _, err := ParseServerHeader("bad header"); err != errInvalidHeader {
		t.Fatalf("Should fail when header can not be parsed")
	}

	if _, err := ParseServerHeader("(bad header)"); err != errInvalidHeader {
		t.Fatalf("Should fail when header can not be parsed")
	}

	if _, err := ParseServerHeader("(without/spaces)"); err != errInvalidHeader {
		t.Fatalf("Should fail when header can not be parsed")
	}

	if _, err := ParseServerHeader("(header/with space)"); err != errInvalidHeader {
		t.Fatalf("Expected err to not exist when ParseServerHeader(\"(header/with space)\")")
	}

	serverHeader, err := ParseServerHeader("foo/bar (baz)")
	if err != nil {
		t.Fatal(err)
	}

	if serverHeader.App != "foo" {
		t.Fatalf("Expected serverHeader.App to equal \"foo\", got %s", serverHeader.App)
	}

	if serverHeader.Ver != "bar" {
		t.Fatalf("Expected serverHeader.Ver to equal \"bar\", got %s", serverHeader.Ver)
	}

	if serverHeader.OS != "baz" {
		t.Fatalf("Expected serverHeader.OS to equal \"baz\", got %s", serverHeader.OS)
	}
}
