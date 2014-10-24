package utils

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestFormatStream(t *testing.T) {
	sf := NewStreamFormatter(true)
	res := sf.FormatStream("stream")
	if string(res) != `{"stream":"stream"}`+"\r\n" {
		t.Fatalf("%q", res)
	}
}

func TestFormatStatus(t *testing.T) {
	sf := NewStreamFormatter(true)
	res := sf.FormatStatus("ID", "%s%d", "a", 1)
	if string(res) != `{"status":"a1","id":"ID"}`+"\r\n" {
		t.Fatalf("%q", res)
	}
}

func TestFormatSimpleError(t *testing.T) {
	sf := NewStreamFormatter(true)
	res := sf.FormatError(errors.New("Error for formatter"))
	if string(res) != `{"errorDetail":{"message":"Error for formatter"},"error":"Error for formatter"}`+"\r\n" {
		t.Fatalf("%q", res)
	}
}

func TestFormatJSONError(t *testing.T) {
	sf := NewStreamFormatter(true)
	err := &JSONError{Code: 50, Message: "Json error"}
	res := sf.FormatError(err)
	if string(res) != `{"errorDetail":{"code":50,"message":"Json error"},"error":"Json error"}`+"\r\n" {
		t.Fatalf("%q", res)
	}
}

func TestFormatProgress(t *testing.T) {
	sf := NewStreamFormatter(true)
	progress := &JSONProgress{
		Current: 15,
		Total:   30,
		Start:   1,
	}
	res := sf.FormatProgress("id", "action", progress)
	msg := &JSONMessage{}
	if err := json.Unmarshal(res, msg); err != nil {
		t.Fatal(err)
	}
	if msg.ID != "id" {
		t.Fatalf("ID must be 'id', got: %s", msg.ID)
	}
	if msg.Status != "action" {
		t.Fatalf("Status must be 'action', got: %s", msg.Status)
	}
	if msg.ProgressMessage != progress.String() {
		t.Fatalf("ProgressMessage must be %s, got: %s", progress.String(), msg.ProgressMessage)
	}
	if !reflect.DeepEqual(msg.Progress, progress) {
		t.Fatal("Original progress not equals progress from FormatProgress")
	}
}
