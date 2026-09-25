package image

import (
	"context"
	"testing"

	containernamegeneratorv0 "github.com/moby/moby/v2/extpoints/containernamegenerator/v0"
	servicenamegeneratorv0 "github.com/moby/moby/v2/extpoints/servicenamegenerator/v0"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestGenerateContainerName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		retry    int64
		expected string
	}{
		{name: "first attempt", expected: "busybox-0123"},
		{name: "first retry", retry: 1, expected: "busybox-012345"},
		{name: "second retry", retry: 2, expected: "busybox-01234567"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reply, err := (generator{}).GenerateContainerName(context.Background(), &containernamegeneratorv0.GenerateContainerNameRequest{
				Retry:       tc.retry,
				ContainerID: "0123456789abcdef",
				Image:       "busybox:latest",
			})
			assert.NilError(t, err)
			assert.Equal(t, reply.Name, tc.expected)
		})
	}
}

func TestGenerateServiceName(t *testing.T) {
	t.Parallel()

	reply, err := (generator{}).GenerateServiceName(context.Background(), &servicenamegeneratorv0.GenerateServiceNameRequest{
		Image: "busybox:latest",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Regexp(`^busybox-[0-9a-f]{8}$`, reply.Name))
}
