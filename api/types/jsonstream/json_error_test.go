package jsonstream_test

import (
	"testing"

	"github.com/moby/moby/api/types/jsonstream"
)

func TestError(t *testing.T) {
	je := jsonstream.Error{Code: 404, Message: "Not found"}
	if got := je.Error(); got != "Not found" {
		t.Errorf("Error() = %q, want %q", got, "Not found")
	}
}

func TestNilError(t *testing.T) {
	var je *jsonstream.Error
	if got := je.Error(); got != "<nil>" {
		t.Errorf("Error() = %q, want %q", got, "<nil>")
	}
}
