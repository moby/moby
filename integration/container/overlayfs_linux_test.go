package container

import (
	"io"
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/archive"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestNoOverlayfsWarningsAboutUndefinedBehaviors(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux", "overlayfs is only available on linux")
	skip.If(t, testEnv.IsRemoteDaemon(), "local daemon is needed for kernel log access")
	skip.If(t, testEnv.IsRootless(), "root is needed for reading kernel log")

	ctx := setupTest(t)
	client := testEnv.APIClient()

	cID := container.Run(ctx, t, client, container.WithCmd("sh", "-c", `while true; do echo $RANDOM >>/file; sleep 0.1; done`))

	tests := []struct {
		name      string
		operation func(t *testing.T) error
	}{
		{name: "diff", operation: func(*testing.T) error {
			_, err := client.ContainerDiff(ctx, cID)
			return err
		}},
		{name: "export", operation: func(*testing.T) error {
			rc, err := client.ContainerExport(ctx, cID)
			if err == nil {
				defer rc.Close()
				_, err = io.Copy(io.Discard, rc)
			}
			return err
		}},
		{name: "cp to container", operation: func(t *testing.T) error {
			archive, err := archive.Generate("new-file", "hello-world")
			assert.NilError(t, err, "failed to create a temporary archive")
			return client.CopyToContainer(ctx, cID, "/", archive, containertypes.CopyToContainerOptions{})
		}},
		{name: "cp from container", operation: func(*testing.T) error {
			rc, _, err := client.CopyFromContainer(ctx, cID, "/file")
			if err == nil {
				defer rc.Close()
				_, err = io.Copy(io.Discard, rc)
			}

			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prev := dmesgLines(256)

			err := tc.operation(t)
			assert.NilError(t, err)

			after := dmesgLines(2048)

			diff := diffDmesg(prev, after)
			for _, line := range diff {
				overlayfs := strings.Contains(line, "overlayfs: ")
				lowerDirInUse := strings.Contains(line, "lowerdir is in-use as ")
				upperDirInUse := strings.Contains(line, "upperdir is in-use as ")
				workDirInuse := strings.Contains(line, "workdir is in-use as ")
				undefinedBehavior := strings.Contains(line, "will result in undefined behavior")

				if overlayfs && (lowerDirInUse || upperDirInUse || workDirInuse) && undefinedBehavior {
					t.Errorf("%s caused overlayfs kernel warning: %s", tc.name, line)
				}
			}
		})
	}
}

// dmesgLines returns last messages from the kernel log, up to size bytes,
// and splits the output by newlines.
func dmesgLines(size int) []string {
	data := make([]byte, size)
	amt, err := unix.Klogctl(unix.SYSLOG_ACTION_READ_ALL, data)
	if err != nil {
		return []string{}
	}
	return strings.Split(strings.TrimSpace(string(data[:amt])), "\n")
}

func diffDmesg(prev, next []string) []string {
	// All lines have a timestamp, so just take the last one from the previous
	// log and find it in the new log.
	lastPrev := prev[len(prev)-1]

	for idx := len(next) - 1; idx >= 0; idx-- {
		line := next[idx]

		if line == lastPrev {
			nextIdx := idx + 1
			if nextIdx < len(next) {
				return next[nextIdx:]
			} else {
				// Found at the last position, log is the same.
				return nil
			}
		}
	}

	return next
}
