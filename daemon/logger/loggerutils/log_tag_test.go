package loggerutils

import (
	"testing"

	"github.com/docker/docker/daemon/logger"
)

func TestParseLogTagDefaultTag(t *testing.T) {
	ctx := buildContext(map[string]string{})
	tag, e := ParseLogTag(ctx, "{{.ID}}")
	assertTag(t, e, tag, ctx.ID())
}

func TestParseLogTag(t *testing.T) {
	ctx := buildContext(map[string]string{"tag": "{{.ImageName}}/{{.Name}}/{{.ID}}"})
	tag, e := ParseLogTag(ctx, "{{.ID}}")
	assertTag(t, e, tag, "test-image/test-container/container-ab")
}

func TestParseLogTagEmptyTag(t *testing.T) {
	ctx := buildContext(map[string]string{})
	tag, e := ParseLogTag(ctx, "{{.DaemonName}}/{{.ID}}")
	assertTag(t, e, tag, "test-dockerd/container-ab")
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

func assertTag(t *testing.T, e error, tag string, expected string) {
	if e != nil {
		t.Fatalf("Error generating tag: %q", e)
	}
	if tag != expected {
		t.Fatalf("Wrong tag: %q, should be %q", tag, expected)
	}
}
