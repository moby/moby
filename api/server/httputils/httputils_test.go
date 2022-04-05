package httputils // import "github.com/docker/docker/api/server/httputils"

import "testing"

// matchesContentType
func TestJsonContentType(t *testing.T) {
	err := matchesContentType("application/json", "application/json")
	if err != nil {
		t.Error(err)
	}

	err = matchesContentType("application/json; charset=utf-8", "application/json")
	if err != nil {
		t.Error(err)
	}

	expected := "unsupported Content-Type header (dockerapplication/json): must be 'application/json'"
	err = matchesContentType("dockerapplication/json", "application/json")
	if err == nil || err.Error() != expected {
		t.Errorf(`expected "%s", got "%v"`, expected, err)
	}

	expected = "malformed Content-Type header (foo;;;bar): mime: invalid media parameter"
	err = matchesContentType("foo;;;bar", "application/json")
	if err == nil || err.Error() != expected {
		t.Errorf(`expected "%s", got "%v"`, expected, err)
	}
}
