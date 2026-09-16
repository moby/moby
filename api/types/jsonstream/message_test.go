package jsonstream_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/moby/moby/api/types/jsonstream"
)

// TestMessageMarshal is a sanity-check to make sure the struct is
// marshaled as expected, including the Error formatted as JSON, not
// as the error-string.
func TestMessageMarshal(t *testing.T) {
	auxM := json.RawMessage(`{"aux":"aux"}`)
	b, err := json.Marshal(&jsonstream.Message{
		Stream: "stream",
		Status: "status",
		Progress: &jsonstream.Progress{
			Current:    1,
			Total:      2,
			Start:      94777200,
			HideCounts: true,
			Units:      "lightyear",
		},
		ID:    "id",
		Error: &jsonstream.Error{Code: http.StatusBadRequest, Message: "error message"},
		Aux:   &auxM,
	})
	if err != nil {
		t.Fatal(err)
	}

	const expected = `{"stream":"stream","status":"status","progressDetail":{"current":1,"total":2,"start":94777200,"hidecounts":true,"units":"lightyear"},"id":"id","errorDetail":{"code":400,"message":"error message"},"aux":{"aux":"aux"}}`
	if got := string(b); got != expected {
		t.Errorf("json.Marshal() = %q, want %q", got, expected)
	}
}
