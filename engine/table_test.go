package engine

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestTableWriteTo(t *testing.T) {
	table := NewTable("", 0)
	e := &Env{}
	e.Set("foo", "bar")
	table.Add(e)
	var buf bytes.Buffer
	if _, err := table.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	output := make(map[string]string)
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 {
		t.Fatalf("Incorrect output: %v", output)
	}
	if val, exists := output["foo"]; !exists || val != "bar" {
		t.Fatalf("Inccorect output: %v", output)
	}
}
