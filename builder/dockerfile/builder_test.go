package dockerfile

import (
	"strings"

	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestBuildProcessLabels(t *testing.T) {
	dockerfile := "FROM scratch"
	d := parser.Directive{}
	parser.SetEscapeToken(parser.DefaultEscapeToken, &d)
	n, err := parser.Parse(strings.NewReader(dockerfile), &d)
	assert.NilError(t, err)

	options := &types.ImageBuildOptions{
		Labels: map[string]string{
			"org.e": "cli-e",
			"org.d": "cli-d",
			"org.c": "cli-c",
			"org.b": "cli-b",
			"org.a": "cli-a",
		},
	}
	b := &Builder{
		runConfig:  &container.Config{},
		options:    options,
		directive:  d,
		dockerfile: n,
	}
	b.processLabels()

	expected := []string{
		"FROM scratch",
		`LABEL "org.a"='cli-a' "org.b"='cli-b' "org.c"='cli-c' "org.d"='cli-d' "org.e"='cli-e'`,
	}
	assert.Equal(t, len(b.dockerfile.Children), 2)
	for i, v := range b.dockerfile.Children {
		assert.Equal(t, v.Original, expected[i])
	}
}
