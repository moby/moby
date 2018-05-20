package parser // import "github.com/docker/docker/builder/dockerfile/parser"

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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
	assert.DeepEqual(t, expected, node, cmpNodeOpt)
}

var cmpNodeOpt = cmp.AllowUnexported(Node{})

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
	assert.DeepEqual(t, expected, node, cmpNodeOpt)
}

func TestParseNameValWithoutVal(t *testing.T) {
	directive := Directive{}
	// In Config.Env, a variable without `=` is removed from the environment. (#31634)
	// However, in Dockerfile, we don't allow "unsetting" an environment variable. (#11922)
	_, err := parseNameVal("foo", "ENV", &directive)
	assert.Check(t, is.ErrorContains(err, ""), "ENV must have two arguments")
}
