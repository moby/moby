package strslice

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestStrSliceMarshalJSON(t *testing.T) {
	for _, testcase := range []struct {
		input    StrSlice
		expected string
	}{
		// MADNESS(stevvooe): No clue why nil would be "" but empty would be
		// "null". Had to make a change here that may affect compatibility.
		{input: nil, expected: "null"},
		{input: StrSlice{}, expected: "[]"},
		{input: StrSlice{"/bin/sh", "-c", "echo"}, expected: `["/bin/sh","-c","echo"]`},
	} {
		data, err := json.Marshal(testcase.input)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != testcase.expected {
			t.Fatalf("%#v: expected %v, got %v", testcase.input, testcase.expected, string(data))
		}
	}
}

func TestStrSliceUnmarshalJSON(t *testing.T) {
	parts := map[string][]string{
		"":                        {"default", "values"},
		"[]":                      {},
		`["/bin/sh","-c","echo"]`: {"/bin/sh", "-c", "echo"},
	}
	for input, expected := range parts {
		strs := StrSlice{"default", "values"}
		if err := strs.UnmarshalJSON([]byte(input)); err != nil {
			t.Fatal(err)
		}

		actual := []string(strs)
		if !slices.Equal(actual, expected) {
			t.Fatalf("%#v: expected %#v, got %#v", input, expected, actual)
		}
	}
}

func TestStrSliceUnmarshalString(t *testing.T) {
	var actual StrSlice
	echo, err := json.Marshal("echo")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(echo, &actual); err != nil {
		t.Fatal(err)
	}

	expected := []string{"echo"}
	if !slices.Equal(actual, expected) {
		t.Fatalf("expected %#v, got %#v", expected, actual)
	}
}

func TestStrSliceUnmarshalSlice(t *testing.T) {
	var actual StrSlice
	echo, err := json.Marshal([]string{"echo"})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(echo, &actual); err != nil {
		t.Fatal(err)
	}

	expected := []string{"echo"}
	if !slices.Equal(actual, expected) {
		t.Fatalf("expected %#v, got %#v", expected, actual)
	}
}
