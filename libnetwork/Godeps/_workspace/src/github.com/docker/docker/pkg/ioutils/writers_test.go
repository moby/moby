package ioutils

import (
	"bytes"
	"strings"
	"testing"
)

func TestNopWriter(t *testing.T) {
	nw := &NopWriter{}
	l, err := nw.Write([]byte{'c'})
	if err != nil {
		t.Fatal(err)
	}
	if l != 1 {
		t.Fatalf("Expected 1 got %d", l)
	}
}

func TestWriteCounter(t *testing.T) {
	dummy1 := "This is a dummy string."
	dummy2 := "This is another dummy string."
	totalLength := int64(len(dummy1) + len(dummy2))

	reader1 := strings.NewReader(dummy1)
	reader2 := strings.NewReader(dummy2)

	var buffer bytes.Buffer
	wc := NewWriteCounter(&buffer)

	reader1.WriteTo(wc)
	reader2.WriteTo(wc)

	if wc.Count != totalLength {
		t.Errorf("Wrong count: %d vs. %d", wc.Count, totalLength)
	}

	if buffer.String() != dummy1+dummy2 {
		t.Error("Wrong message written")
	}
}
