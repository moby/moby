package parser // import "github.com/docker/docker/builder/dockerfile/parser"

import (
	"testing"

	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

func TestParseNameValOldFormat(t *testing.T) {
	directive := Directive{}
	node, err := parseNameVal("foo bar", "LABEL", &directive)
	assert.Check(t, err)

	expected := &Node{
		Value: "foo",
		Next:  &Node{Value: "bar"},
	}
	assert.Check(t, is.DeepEqual(expected, node))
}

func TestParseNameValNewFormat(t *testing.T) {
	directive := Directive{}
	node, err := parseNameVal("foo=bar thing=star", "LABEL", &directive)
	assert.Check(t, err)

	expected := &Node{
		Value: "foo",
		Next: &Node{
			Value: "bar",
			Next: &Node{
				Value: "thing",
				Next: &Node{
					Value: "star",
				},
			},
		},
	}
	assert.Check(t, is.DeepEqual(expected, node))
}

func TestNodeFromLabels(t *testing.T) {
	labels := map[string]string{
		"foo":   "bar",
		"weird": "first' second",
	}
	expected := &Node{
		Value:    "label",
		Original: `LABEL "foo"='bar' "weird"='first' second'`,
		Next: &Node{
			Value: "foo",
			Next: &Node{
				Value: "'bar'",
				Next: &Node{
					Value: "weird",
					Next: &Node{
						Value: "'first' second'",
					},
				},
			},
		},
	}

	node := NodeFromLabels(labels)
	assert.Check(t, is.DeepEqual(expected, node))

}

func TestParseNameValWithoutVal(t *testing.T) {
	directive := Directive{}
	// In Config.Env, a variable without `=` is removed from the environment. (#31634)
	// However, in Dockerfile, we don't allow "unsetting" an environment variable. (#11922)
	_, err := parseNameVal("foo", "ENV", &directive)
	assert.Check(t, is.ErrorContains(err, ""), "ENV must have two arguments")
}
