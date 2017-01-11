package logger

import (
	"reflect"
	"testing"
	"time"
)

func TestCopyMessage(t *testing.T) {
	msg := &Message{
		Line:      []byte("test line."),
		Source:    "stdout",
		Timestamp: time.Now(),
		Attrs: LogAttributes{
			"key1": "val1",
			"key2": "val2",
			"key3": "val3",
		},
		Partial: true,
	}

	m := CopyMessage(msg)
	if !reflect.DeepEqual(m, msg) {
		t.Fatalf("CopyMessage failed to copy message")
	}
}
