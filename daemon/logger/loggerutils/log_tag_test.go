package loggerutils

import (
	"testing"

	"github.com/docker/docker/daemon/logger"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestParseLogTagDefaultTag(c *check.C) {
	ctx := buildContext(map[string]string{})
	tag, e := ParseLogTag(ctx, "{{.ID}}")
	assertTag(c, e, tag, ctx.ID())
}

func (s *DockerSuite) TestParseLogTag(c *check.C) {
	ctx := buildContext(map[string]string{"tag": "{{.ImageName}}/{{.Name}}/{{.ID}}"})
	tag, e := ParseLogTag(ctx, "{{.ID}}")
	assertTag(c, e, tag, "test-image/test-container/container-ab")
}

func (s *DockerSuite) TestParseLogTagEmptyTag(c *check.C) {
	ctx := buildContext(map[string]string{})
	tag, e := ParseLogTag(ctx, "{{.DaemonName}}/{{.ID}}")
	assertTag(c, e, tag, "test-dockerd/container-ab")
}

// Helpers

func buildContext(cfg map[string]string) logger.Context {
	return logger.Context{
		ContainerID:        "container-abcdefghijklmnopqrstuvwxyz01234567890",
		ContainerName:      "/test-container",
		ContainerImageID:   "image-abcdefghijklmnopqrstuvwxyz01234567890",
		ContainerImageName: "test-image",
		Config:             cfg,
		DaemonName:         "test-dockerd",
	}
}

func assertTag(c *check.C, e error, tag string, expected string) {
	if e != nil {
		c.Fatalf("Error generating tag: %q", e)
	}
	if tag != expected {
		c.Fatalf("Wrong tag: %q, should be %q", tag, expected)
	}
}
