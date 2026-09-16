package srslog

import (
	"testing"
)

func TestDefaultFramer(t *testing.T) {
	out := DefaultFramer("input message")
	if out != "input message" {
		t.Errorf("should match the input message")
	}
}

func TestRFC5425MessageLengthFramer(t *testing.T) {
	out := RFC5425MessageLengthFramer("input message")
	if out != "13 input message" {
		t.Errorf("should prepend the input message length")
	}
}
