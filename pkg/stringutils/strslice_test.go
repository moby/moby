package stringutils

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStrSliceMarshalJSON(t *testing.T) {
	strss := map[*StrSlice]string{
		nil:         "",
		&StrSlice{}: "null",
		&StrSlice{[]string{"/bin/sh", "-c", "echo"}}: `["/bin/sh","-c","echo"]`,
	}

	for strs, expected := range strss {
		data, err := strs.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != expected {
			t.Fatalf("Expected %v, got %v", expected, string(data))
		}
	}
}

func TestStrSliceUnmarshalJSON(t *testing.T) {
	parts := map[string][]string{
		"":   {"default", "values"},
		"[]": {},
		`["/bin/sh","-c","echo"]`: {"/bin/sh", "-c", "echo"},
	}
	for json, expectedParts := range parts {
		strs := &StrSlice{
			[]string{"default", "values"},
		}
		if err := strs.UnmarshalJSON([]byte(json)); err != nil {
			t.Fatal(err)
		}

		actualParts := strs.Slice()
		if len(actualParts) != len(expectedParts) {
			t.Fatalf("Expected %v parts, got %v (%v)", len(expectedParts), len(actualParts), expectedParts)
		}
		for index, part := range actualParts {
			if part != expectedParts[index] {
				t.Fatalf("Expected %v, got %v", expectedParts, actualParts)
				break
			}
		}
	}
}

func TestStrSliceUnmarshalString(t *testing.T) {
	var e *StrSlice
	echo, err := json.Marshal("echo")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(echo, &e); err != nil {
		t.Fatal(err)
	}

	slice := e.Slice()
	if len(slice) != 1 {
		t.Fatalf("expected 1 element after unmarshal: %q", slice)
	}

	if slice[0] != "echo" {
		t.Fatalf("expected `echo`, got: %q", slice[0])
	}
}

func TestStrSliceUnmarshalSlice(t *testing.T) {
	var e *StrSlice
	echo, err := json.Marshal([]string{"echo"})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(echo, &e); err != nil {
		t.Fatal(err)
	}

	slice := e.Slice()
	if len(slice) != 1 {
		t.Fatalf("expected 1 element after unmarshal: %q", slice)
	}

	if slice[0] != "echo" {
		t.Fatalf("expected `echo`, got: %q", slice[0])
	}
}

func TestStrSliceToString(t *testing.T) {
	slices := map[*StrSlice]string{
		NewStrSlice(""):           "",
		NewStrSlice("one"):        "one",
		NewStrSlice("one", "two"): "one two",
	}
	for s, expected := range slices {
		toString := s.ToString()
		if toString != expected {
			t.Fatalf("Expected %v, got %v", expected, toString)
		}
	}
}

func TestStrSliceLen(t *testing.T) {
	var emptyStrSlice *StrSlice
	slices := map[*StrSlice]int{
		NewStrSlice(""):           1,
		NewStrSlice("one"):        1,
		NewStrSlice("one", "two"): 2,
		emptyStrSlice:             0,
	}
	for s, expected := range slices {
		if s.Len() != expected {
			t.Fatalf("Expected %d, got %d", s.Len(), expected)
		}
	}
}

func TestStrSliceSlice(t *testing.T) {
	var emptyStrSlice *StrSlice
	slices := map[*StrSlice][]string{
		NewStrSlice("one"):        {"one"},
		NewStrSlice("one", "two"): {"one", "two"},
		emptyStrSlice:             nil,
	}
	for s, expected := range slices {
		if !reflect.DeepEqual(s.Slice(), expected) {
			t.Fatalf("Expected %v, got %v", s.Slice(), expected)
		}
	}
}
