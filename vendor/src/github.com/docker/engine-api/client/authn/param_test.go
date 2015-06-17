package authn

import (
	"testing"
)

func TestStrspn(t *testing.T) {
	type strspntest struct {
		input, chars string
		length       int
	}
	tests := []strspntest{
		{"abcdefg", "bdca", 4},
		{"abcdefg", "efg", 0},
		{"abcdefg", "hijkl", 0},
		{"abcdefg", "0123", 0},
		{"abcdefg", "aaaa", 1},
		{"abcdefg", "fedcba", 6},
		{"abcdefg", "fedghijklmcba", 7},
	}
	for _, test := range tests {
		if strspn(test.input, test.chars) != test.length {
			t.Fatalf(`strspn("%s","%s") != %d`, test.input, test.chars, test.length)
		}
	}
}

func TestTokenize(t *testing.T) {
	type tokenizeTest struct {
		input  string
		output []string
		err    string
	}
	tests := []tokenizeTest{
		{"Bearer", []string{"Bearer"}, ""},
		{`Basic realm="foo"`, []string{"Basic", "realm=foo"}, ""},
		{`Basic realm="foo", chars=blah`, []string{"Basic", "realm=foo", "chars=blah"}, ""},
	}
	for _, test := range tests {
		_, err := tokenize(test.input)
		if err != nil {
			if err.Error() != test.err {
				t.Fatalf("Unexpected error tokenizing %v: %v", test.input, err)
			}
		} else {
			if test.err != "" {
				t.Fatalf("Unexpected non-error tokenizing %v: expected %v", test.input, test.err)
			}
		}
	}
}

func TestGetParameter(t *testing.T) {
	type getParameterTest struct {
		input     string
		parameter string
		value     string
	}
	tests := []getParameterTest{
		{`Basic realm="foo"`, "realm", "foo"},
		{`Basic realm="foo", chars=blah`, "realm", "foo"},
		{`Basic realm="foo", chars=blah`, "chars", "blah"},
	}
	for _, test := range tests {
		value, err := getParameter(test.input, test.parameter)
		if err != nil {
			t.Fatalf(`Got error %v instead of "%s" from "%s"["%s"]`, err, test.value, test.input, test.parameter)
		}
		if value != test.value {
			t.Fatalf(`Got "%s" instead of "%s" from "%s"["%s"]`, value, test.value, test.input, test.parameter)
		}
	}
}
