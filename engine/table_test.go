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

func TestTableSortStringValue(t *testing.T) {
	table := NewTable("Key", 0)

	e := &Env{}
	e.Set("Key", "A")
	table.Add(e)

	e = &Env{}
	e.Set("Key", "D")
	table.Add(e)

	e = &Env{}
	e.Set("Key", "B")
	table.Add(e)

	e = &Env{}
	e.Set("Key", "C")
	table.Add(e)

	table.Sort()

	if len := table.Len(); len != 4 {
		t.Fatalf("Expected 4, got %d", len)
	}

	if value := table.Data[0].Get("Key"); value != "A" {
		t.Fatalf("Expected A, got %s", value)
	}

	if value := table.Data[1].Get("Key"); value != "B" {
		t.Fatalf("Expected B, got %s", value)
	}

	if value := table.Data[2].Get("Key"); value != "C" {
		t.Fatalf("Expected C, got %s", value)
	}

	if value := table.Data[3].Get("Key"); value != "D" {
		t.Fatalf("Expected D, got %s", value)
	}
}

func TestTableReverseSortStringValue(t *testing.T) {
	table := NewTable("Key", 0)

	e := &Env{}
	e.Set("Key", "A")
	table.Add(e)

	e = &Env{}
	e.Set("Key", "D")
	table.Add(e)

	e = &Env{}
	e.Set("Key", "B")
	table.Add(e)

	e = &Env{}
	e.Set("Key", "C")
	table.Add(e)

	table.ReverseSort()

	if len := table.Len(); len != 4 {
		t.Fatalf("Expected 4, got %d", len)
	}

	if value := table.Data[0].Get("Key"); value != "D" {
		t.Fatalf("Expected D, got %s", value)
	}

	if value := table.Data[1].Get("Key"); value != "C" {
		t.Fatalf("Expected B, got %s", value)
	}

	if value := table.Data[2].Get("Key"); value != "B" {
		t.Fatalf("Expected C, got %s", value)
	}

	if value := table.Data[3].Get("Key"); value != "A" {
		t.Fatalf("Expected A, got %s", value)
	}
}
