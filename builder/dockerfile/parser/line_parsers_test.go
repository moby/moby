package parser

import (
	"github.com/docker/docker/pkg/testutil/assert"
	"testing"
)

func TestParseNameValOldFormat(t *testing.T) {
	directive := Directive{}
	node, err := parseNameVal("foo bar", "LABEL", &directive)
	assert.NilError(t, err)

	expected := &Node{
		Value: "foo",
		Next:  &Node{Value: "bar"},
	}
	assert.DeepEqual(t, node, expected)
}

func TestParseNameValNewFormat(t *testing.T) {
	directive := Directive{}
	node, err := parseNameVal("foo=bar thing=star", "LABEL", &directive)
	assert.NilError(t, err)

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
	assert.DeepEqual(t, node, expected)
}

func TestNodeFromLabels(t *testing.T) {
	labels := map[string]string{
		"foo":   "bar",
		"weird": "'first second'",
	}
	expected := &Node{
		Value:    "label",
		Original: `LABEL "foo"='bar' "weird"=''first second''`,
		Next: &Node{
			Value: "foo",
			Next: &Node{
				Value: "bar",
				Next: &Node{
					Value: "weird",
					Next: &Node{
						Value: "'first second'",
					},
				},
			},
		},
	}

	node := NodeFromLabels(labels)
	assert.DeepEqual(t, node, expected)

}
