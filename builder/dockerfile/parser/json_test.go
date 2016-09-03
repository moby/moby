package parser

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

var invalidJSONArraysOfStrings = []string{
	`["a",42,"b"]`,
	`["a",123.456,"b"]`,
	`["a",{},"b"]`,
	`["a",{"c": "d"},"b"]`,
	`["a",["c"],"b"]`,
	`["a",true,"b"]`,
	`["a",false,"b"]`,
	`["a",null,"b"]`,
}

var validJSONArraysOfStrings = map[string][]string{
	`[]`:           {},
	`[""]`:         {""},
	`["a"]`:        {"a"},
	`["a","b"]`:    {"a", "b"},
	`[ "a", "b" ]`: {"a", "b"},
	`[	"a",	"b"	]`: {"a", "b"},
	`	[	"a",	"b"	]	`: {"a", "b"},
	`["abc 123", "♥", "☃", "\" \\ \/ \b \f \n \r \t \u0000"]`: {"abc 123", "♥", "☃", "\" \\ / \b \f \n \r \t \u0000"},
}

func (s *DockerSuite) TestJSONArraysOfStrings(c *check.C) {
	for json, expected := range validJSONArraysOfStrings {
		d := Directive{}
		SetEscapeToken(DefaultEscapeToken, &d)

		if node, _, err := parseJSON(json, &d); err != nil {
			c.Fatalf("%q should be a valid JSON array of strings, but wasn't! (err: %q)", json, err)
		} else {
			i := 0
			for node != nil {
				if i >= len(expected) {
					c.Fatalf("expected result is shorter than parsed result (%d vs %d+) in %q", len(expected), i+1, json)
				}
				if node.Value != expected[i] {
					c.Fatalf("expected %q (not %q) in %q at pos %d", expected[i], node.Value, json, i)
				}
				node = node.Next
				i++
			}
			if i != len(expected) {
				c.Fatalf("expected result is longer than parsed result (%d vs %d) in %q", len(expected), i+1, json)
			}
		}
	}
	for _, json := range invalidJSONArraysOfStrings {
		d := Directive{}
		SetEscapeToken(DefaultEscapeToken, &d)

		if _, _, err := parseJSON(json, &d); err != errDockerfileNotStringArray {
			c.Fatalf("%q should be an invalid JSON array of strings, but wasn't!", json)
		}
	}
}
