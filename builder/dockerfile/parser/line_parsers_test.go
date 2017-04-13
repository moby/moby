package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNameValOldFormat(t *testing.T) {
	directive := Directive{}
	node, err := parseNameVal("foo bar", "LABEL", &directive)
	assert.NoError(t, err)

	expected := &Node{
		Value: "foo",
		Next:  &Node{Value: "bar"},
	}
	assert.Equal(t, expected, node)
}

func TestParseNameValNewFormat(t *testing.T) {
	directive := Directive{}
	node, err := parseNameVal("foo=bar thing=star", "LABEL", &directive)
	assert.NoError(t, err)

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
	assert.Equal(t, expected, node)
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
	assert.Equal(t, expected, node)

}
