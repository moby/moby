package data

import (
	"testing"
)

func TestEmptyMessage(t *testing.T) {
	m := Empty()
	if m.String() != Encode(nil) {
		t.Fatalf("%v != %v", m.String(), Encode(nil))
	}
}

func TestSetMessage(t *testing.T) {
	m := Empty().Set("foo", "bar")
	output := m.String()
	expectedOutput := "000;3:foo,6:3:bar,,"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
	decodedOutput, err := Decode(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(decodedOutput) != 1 {
		t.Fatalf("wrong output data: %#v\n", decodedOutput)
	}
}

func TestSetMessageTwice(t *testing.T) {
	m := Empty().Set("foo", "bar").Set("ga", "bu")
	output := m.String()
	expectedOutput := "000;3:foo,6:3:bar,,2:ga,5:2:bu,,"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
	decodedOutput, err := Decode(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(decodedOutput) != 2 {
		t.Fatalf("wrong output data: %#v\n", decodedOutput)
	}
}

func TestSetDelMessage(t *testing.T) {
	m := Empty().Set("foo", "bar").Del("foo")
	output := m.String()
	expectedOutput := Encode(nil)
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
}
