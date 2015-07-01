package httputils

import (
	"testing"
)

func TestDetectContentType(t *testing.T) {
	input := []byte("That is just a plain text")

	if contentType, _, err := DetectContentType(input); err != nil || contentType != "text/plain" {
		t.Errorf("TestDetectContentType failed")
	}
}
