package data

import (
	"strings"
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

func TestDecodeString(t *testing.T) {
	validEncodedStrings := []struct {
		input  string
		output string
		skip   int
	}{
		{"3:foo,", "foo", 6},
		{"5:hello,", "hello", 8},
		{"5:hello,5:world,", "hello", 8},
	}
	for _, sample := range validEncodedStrings {
		output, skip, err := decodeString(sample.input)
		if err != nil {
			t.Fatalf("error decoding '%v': %v", sample.input, err)
		}
		if skip != sample.skip {
			t.Fatalf("invalid skip: %v!=%v", skip, sample.skip)
		}
		if output != sample.output {
			t.Fatalf("invalid output: %v!=%v", output, sample.output)
		}
	}
}

func TestDecode1Key1Value(t *testing.T) {
	input := "000;3:foo,6:3:bar,,"
	output, err := Decode(input)
	if err != nil {
		t.Fatal(err)
	}
	if v, exists := output["foo"]; !exists {
		t.Fatalf("wrong output: %v\n", output)
	} else if len(v) != 1 || strings.Join(v, "") != "bar" {
		t.Fatalf("wrong output: %v\n", output)
	}
}
