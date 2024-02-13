package httputils // import "github.com/docker/docker/api/server/httputils"

import (
	"net/http"
	"strings"
	"testing"
)

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

func TestReadJSON(t *testing.T) {
	t.Run("nil body", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "https://example.com/some/path", nil)
		if err != nil {
			t.Error(err)
		}
		foo := struct{}{}
		err = ReadJSON(req, &foo)
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "https://example.com/some/path", strings.NewReader(""))
		if err != nil {
			t.Error(err)
		}
		foo := struct{ SomeField string }{}
		err = ReadJSON(req, &foo)
		if err != nil {
			t.Error(err)
		}
		if foo.SomeField != "" {
			t.Errorf("expected: '', got: %s", foo.SomeField)
		}
	})

	t.Run("with valid request", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "https://example.com/some/path", strings.NewReader(`{"SomeField":"some value"}`))
		if err != nil {
			t.Error(err)
		}
		req.Header.Set("Content-Type", "application/json")
		foo := struct{ SomeField string }{}
		err = ReadJSON(req, &foo)
		if err != nil {
			t.Error(err)
		}
		if foo.SomeField != "some value" {
			t.Errorf("expected: 'some value', got: %s", foo.SomeField)
		}
	})
	t.Run("with whitespace", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "https://example.com/some/path", strings.NewReader(`

	{"SomeField":"some value"}

`))
		if err != nil {
			t.Error(err)
		}
		req.Header.Set("Content-Type", "application/json")
		foo := struct{ SomeField string }{}
		err = ReadJSON(req, &foo)
		if err != nil {
			t.Error(err)
		}
		if foo.SomeField != "some value" {
			t.Errorf("expected: 'some value', got: %s", foo.SomeField)
		}
	})

	t.Run("with extra content", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "https://example.com/some/path", strings.NewReader(`{"SomeField":"some value"} and more content`))
		if err != nil {
			t.Error(err)
		}
		req.Header.Set("Content-Type", "application/json")
		foo := struct{ SomeField string }{}
		err = ReadJSON(req, &foo)
		if err == nil {
			t.Error("expected an error, got none")
		}
		expected := "unexpected content after JSON"
		if err.Error() != expected {
			t.Errorf("expected: '%s', got: %s", expected, err.Error())
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "https://example.com/some/path", strings.NewReader(`{invalid json`))
		if err != nil {
			t.Error(err)
		}
		req.Header.Set("Content-Type", "application/json")
		foo := struct{ SomeField string }{}
		err = ReadJSON(req, &foo)
		if err == nil {
			t.Error("expected an error, got none")
		}
		expected := "invalid JSON: invalid character 'i' looking for beginning of object key string"
		if err.Error() != expected {
			t.Errorf("expected: '%s', got: %s", expected, err.Error())
		}
	})
}
