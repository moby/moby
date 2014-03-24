package data

import (
	"testing"
)

func TestEncodeHelloWorld(t *testing.T) {
	input := "hello world!"
	output := encodeString(input)
	expectedOutput := "12:hello world!,"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
}

func TestEncodeEmptyString(t *testing.T) {
	input := ""
	output := encodeString(input)
	expectedOutput := "0:,"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
}

func TestEncodeEmptyList(t *testing.T) {
	input := []string{}
	output := encodeList(input)
	expectedOutput := "0:,"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
}

func TestEncodeEmptyMap(t *testing.T) {
	input := make(map[string][]string)
	output := Encode(input)
	expectedOutput := "000;"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
}

func TestEncode1Key1Value(t *testing.T) {
	input := make(map[string][]string)
	input["hello"] = []string{"world"}
	output := Encode(input)
	expectedOutput := "000;5:hello,8:5:world,,"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
}

func TestEncode1Key2Value(t *testing.T) {
	input := make(map[string][]string)
	input["hello"] = []string{"beautiful", "world"}
	output := Encode(input)
	expectedOutput := "000;5:hello,20:9:beautiful,5:world,,"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
}

func TestEncodeEmptyValue(t *testing.T) {
	input := make(map[string][]string)
	input["foo"] = []string{}
	output := Encode(input)
	expectedOutput := "000;3:foo,0:,"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
}

func TestEncodeBinaryKey(t *testing.T) {
	input := make(map[string][]string)
	input["foo\x00bar\x7f"] = []string{}
	output := Encode(input)
	expectedOutput := "000;8:foo\x00bar\x7f,0:,"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
}

func TestEncodeBinaryValue(t *testing.T) {
	input := make(map[string][]string)
	input["foo\x00bar\x7f"] = []string{"\x01\x02\x03\x04"}
	output := Encode(input)
	expectedOutput := "000;8:foo\x00bar\x7f,7:4:\x01\x02\x03\x04,,"
	if output != expectedOutput {
		t.Fatalf("'%v' != '%v'", output, expectedOutput)
	}
}
