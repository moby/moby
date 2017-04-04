package dockerfile

import (
	"strings"
	"testing"

	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestAddNodesForLabelOption(t *testing.T) {
	dockerfile := "FROM scratch"
	d := parser.Directive{}
	parser.SetEscapeToken(parser.DefaultEscapeToken, &d)
	nodes, err := parser.Parse(strings.NewReader(dockerfile), &d)
	assert.NilError(t, err)

	labels := map[string]string{
		"org.e": "cli-e",
		"org.d": "cli-d",
		"org.c": "cli-c",
		"org.b": "cli-b",
		"org.a": "cli-a",
	}
	addNodesForLabelOption(nodes, labels)

	expected := []string{
		"FROM scratch",
		`LABEL "org.a"='cli-a' "org.b"='cli-b' "org.c"='cli-c' "org.d"='cli-d' "org.e"='cli-e'`,
	}
	assert.Equal(t, len(nodes.Children), 2)
	for i, v := range nodes.Children {
		assert.Equal(t, v.Original, expected[i])
	}
}
