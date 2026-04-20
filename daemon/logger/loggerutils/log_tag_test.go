package loggerutils

import (
	"os"
	"testing"

	"github.com/moby/moby/v2/daemon/logger"
	"gotest.tools/v3/assert"
)

func TestParseLogTagZeroValues(t *testing.T) {
	out, err := ParseLogTag(logger.Info{}, "")
	assert.NilError(t, err)
	assert.Assert(t, out == "")

	out, err = ParseLogTag(logger.Info{}, DefaultTemplate)
	assert.NilError(t, err)
	assert.Assert(t, out == "")
}

func TestParseLogTag(t *testing.T) {
	tests := []struct {
		doc             string
		customTag       string
		defaultTemplate string
		expected        string
	}{
		{
			doc: "empty tag without default",
		},
		{
			doc:             "non-template tag",
			customTag:       "my-custom-tag",
			defaultTemplate: "{{.ID}}",
			expected:        "my-custom-tag",
		},
		{
			doc:             "empty tag with default template",
			defaultTemplate: "{{.Name}}/{{.ID}}",
			expected:        "test-container/abcdef012345",
		},
		{
			doc:             "custom tag overrides default",
			customTag:       "{{.Name}}/{{.ID}}",
			defaultTemplate: "{{.ID}}",
			expected:        "test-container/abcdef012345",
		},
		{
			doc:       "short ID",
			customTag: ctrShortID,
			expected:  "abcdef012345",
		},
		{
			doc:       "full ID",
			customTag: ctrFullID,
			expected:  "abcdef01234567890abcdef01234567890abcdef01234567890abcdef0123456",
		},
		{
			// TODO(thaJeztah): not documented: https://docs.docker.com/engine/logging/log_tags/
			doc:       "container command",
			customTag: ctrCommand,
			expected:  "/bin/sh -c hello",
		},
		{
			doc:       "short image ID",
			customTag: imgShortID,
			expected:  "582c496ccf79",
		},
		{
			doc:       "full image ID",
			customTag: imgFullID,
			expected:  "sha256:582c496ccf79d8aa6f8203a79d32aaf7ffd8b13362c60a701a2f9ac64886c93d",
		},
		{
			// Image name is currently formatted as specified by the user, and not
			// normalized (i.e., not made "Familiar" or "Canonical").
			doc:       "image name",
			customTag: imgName,
			expected:  "docker.io/library/hello-world:alpine",
		},
		{
			// TODO(thaJeztah): not documented: https://docs.docker.com/engine/logging/log_tags/
			doc:       "hostname",
			customTag: hostName,
			expected:  func() string { h, _ := os.Hostname(); return h }(),
		},

		// Direct use of exported Info fields.
		{
			doc:       "DaemonName",
			customTag: "{{.DaemonName}}",
			expected:  "test-dockerd",
		},

		// TODO(thaJeztah): these are undocumented: consider removing or exposing through methods
		// see: https://docs.docker.com/engine/logging/log_tags/
		{
			doc:       "ContainerID",
			customTag: "{{.ContainerID}}",
			expected:  "abcdef01234567890abcdef01234567890abcdef01234567890abcdef0123456",
		},
		{
			doc:       "ContainerName",
			customTag: "{{.ContainerName}}",
			expected:  "/test-container",
		},
		{
			doc:       "ContainerImageID",
			customTag: "{{.ContainerImageID}}",
			expected:  "sha256:582c496ccf79d8aa6f8203a79d32aaf7ffd8b13362c60a701a2f9ac64886c93d",
		},
		{
			doc:       "ContainerImageName",
			customTag: "{{.ContainerImageName}}",
			expected:  "docker.io/library/hello-world:alpine",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			attrs := map[string]string{}
			if tc.customTag != "" {
				attrs[logger.AttrLogTag] = tc.customTag
			}
			tag, err := ParseLogTag(buildContext(attrs), tc.defaultTemplate)
			assert.NilError(t, err)
			assert.Equal(t, tag, tc.expected)
		})
	}
}

func buildContext(cfg map[string]string) logger.Info {
	return logger.Info{
		ContainerID:         "abcdef01234567890abcdef01234567890abcdef01234567890abcdef0123456",
		ContainerName:       "/test-container",
		ContainerImageID:    "sha256:582c496ccf79d8aa6f8203a79d32aaf7ffd8b13362c60a701a2f9ac64886c93d",
		ContainerImageName:  "docker.io/library/hello-world:alpine",
		ContainerEntrypoint: "/bin/sh",
		ContainerArgs:       []string{"-c", "hello"},
		Config:              cfg,
		DaemonName:          "test-dockerd",
	}
}
