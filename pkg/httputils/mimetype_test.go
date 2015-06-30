package httputils

import (
	"testing"
)

func TestDetectContentType(t *testing.T) {
	input := []byte("That is just a plain text")

	contentType, _, err := DetectContentType(input)
	if err != nil || contentType != "text/plain" {
		t.Errorf("TestDetectContentType failed")
	}
}
