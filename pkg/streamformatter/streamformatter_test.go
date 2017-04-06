package streamformatter

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/jsonmessage"
)

func TestFormatStream(t *testing.T) {
	sf := NewStreamFormatter()
	res := sf.FormatStream("stream")
	if string(res) != "stream"+"\r" {
		t.Fatalf("%q", res)
	}
}

func TestFormatJSONStatus(t *testing.T) {
	sf := NewStreamFormatter()
	res := sf.FormatStatus("ID", "%s%d", "a", 1)
	if string(res) != "a1\r\n" {
		t.Fatalf("%q", res)
	}
}

func TestFormatSimpleError(t *testing.T) {
	sf := NewStreamFormatter()
	res := sf.FormatError(errors.New("Error for formatter"))
	if string(res) != "Error: Error for formatter\r\n" {
		t.Fatalf("%q", res)
	}
}

func TestJSONFormatStream(t *testing.T) {
	sf := NewJSONStreamFormatter()
	res := sf.FormatStream("stream")
	if string(res) != `{"stream":"stream"}`+"\r\n" {
		t.Fatalf("%q", res)
	}
}

func TestJSONFormatStatus(t *testing.T) {
	sf := NewJSONStreamFormatter()
	res := sf.FormatStatus("ID", "%s%d", "a", 1)
	if string(res) != `{"status":"a1","id":"ID"}`+"\r\n" {
		t.Fatalf("%q", res)
	}
}

func TestJSONFormatSimpleError(t *testing.T) {
	sf := NewJSONStreamFormatter()
	res := sf.FormatError(errors.New("Error for formatter"))
	if string(res) != `{"errorDetail":{"message":"Error for formatter"},"error":"Error for formatter"}`+"\r\n" {
		t.Fatalf("%q", res)
	}
}

func TestJSONFormatJSONError(t *testing.T) {
	sf := NewJSONStreamFormatter()
	err := &jsonmessage.JSONError{Code: 50, Message: "Json error"}
	res := sf.FormatError(err)
	if string(res) != `{"errorDetail":{"code":50,"message":"Json error"},"error":"Json error"}`+"\r\n" {
		t.Fatalf("%q", res)
	}
}

func TestJSONFormatProgress(t *testing.T) {
	sf := NewJSONStreamFormatter()
	progress := &jsonmessage.JSONProgress{
		Current: 15,
		Total:   30,
		Start:   1,
	}
	res := sf.FormatProgress("id", "action", progress, nil)
	msg := &jsonmessage.JSONMessage{}
	if err := json.Unmarshal(res, msg); err != nil {
		t.Fatal(err)
	}
	if msg.ID != "id" {
		t.Fatalf("ID must be 'id', got: %s", msg.ID)
	}
	if msg.Status != "action" {
		t.Fatalf("Status must be 'action', got: %s", msg.Status)
	}

	// The progress will always be in the format of:
	// [=========================>                         ]      15B/30B 412910h51m30s
	// The last entry '404933h7m11s' is the timeLeftBox.
	// However, the timeLeftBox field may change as progress.String() depends on time.Now().
	// Therefore, we have to strip the timeLeftBox from the strings to do the comparison.

	// Compare the progress strings before the timeLeftBox
	expectedProgress := "[=========================>                         ]      15B/30B"
	// if terminal column is <= 110, expectedProgressShort is expected.
	expectedProgressShort := "      15B/30B"
	if !(strings.HasPrefix(msg.ProgressMessage, expectedProgress) ||
		strings.HasPrefix(msg.ProgressMessage, expectedProgressShort)) {
		t.Fatalf("ProgressMessage without the timeLeftBox must be %s or %s, got: %s",
			expectedProgress, expectedProgressShort, msg.ProgressMessage)
	}

	if !reflect.DeepEqual(msg.Progress, progress) {
		t.Fatal("Original progress not equals progress from FormatProgress")
	}
}
