package capabilities

import (
	"bytes"
	"io"
	"strings"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/stdcopy"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

func TestNoNewPrivileges(t *testing.T) {
	ctx := setupTest(t)

	withFileCapability := `
		FROM debian:bullseye-slim
		RUN apt-get update && apt-get install -y libcap2-bin --no-install-recommends
		RUN setcap CAP_DAC_OVERRIDE=+eip /bin/cat
		RUN echo "hello" > /txt && chown 0:0 /txt && chmod 700 /txt
		RUN useradd -u 1500 test
	`
	imageTag := "captest"

	source := fakecontext.New(t, "", fakecontext.WithDockerfile(withFileCapability))
	defer source.Close()

	apiClient := testEnv.APIClient()

	// Build image
	resp, err := apiClient.ImageBuild(ctx, source.AsTarReader(t), client.ImageBuildOptions{
		Tags: []string{imageTag},
	})
	assert.NilError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	assert.NilError(t, err)
	resp.Body.Close()

	testCases := []struct {
		doc            string
		opts           []func(*container.TestContainerConfig)
		stdOut, stdErr string
	}{
		{
			doc: "CapabilityRequested=true",
			opts: []func(*container.TestContainerConfig){
				container.WithUser("test"),
				container.WithCapability("CAP_DAC_OVERRIDE"),
			},
			stdOut: "hello",
		},
		{
			doc: "CapabilityRequested=false",
			opts: []func(*container.TestContainerConfig){
				container.WithUser("test"),
				container.WithDropCapability("CAP_DAC_OVERRIDE"),
			},
			stdErr: "exec /bin/cat: operation not permitted",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.doc, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			// Run the container with the image
			opts := append(tc.opts,
				container.WithImage(imageTag),
				container.WithCmd("/bin/cat", "/txt"),
				container.WithSecurityOpt("no-new-privileges=true"),
			)
			cid := container.Run(ctx, t, apiClient, opts...)
			poll.WaitOn(t, container.IsInState(ctx, apiClient, cid, containertypes.StateExited))

			// Assert on outputs
			logReader, err := apiClient.ContainerLogs(ctx, cid, client.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			})
			assert.NilError(t, err)
			defer logReader.Close()

			var actualStdout, actualStderr bytes.Buffer
			_, err = stdcopy.StdCopy(&actualStdout, &actualStderr, logReader)
			assert.NilError(t, err)

			stdOut := strings.TrimSpace(actualStdout.String())
			stdErr := strings.TrimSpace(actualStderr.String())
			if stdOut != tc.stdOut {
				t.Fatalf("test produced invalid output: %q, expected %q. Stderr:%q", stdOut, tc.stdOut, stdErr)
			}
			if stdErr != tc.stdErr {
				t.Fatalf("test produced invalid error: %q, expected %q. Stdout:%q", stdErr, tc.stdErr, stdOut)
			}
		})
	}
}
