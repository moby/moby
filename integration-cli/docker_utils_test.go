package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/go-archive"
	"github.com/moby/moby/api/types/common"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration-cli/cli"
	"github.com/moby/moby/v2/integration-cli/daemon"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/specialimage"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
)

func deleteImages(images ...string) error {
	args := []string{dockerBinary, "rmi", "-f"}
	return icmd.RunCmd(icmd.Cmd{Command: append(args, images...)}).Error
}

// Deprecated: use cli.Docker or cli.DockerCmd
func dockerCmdWithError(args ...string) (string, int, error) {
	result := cli.Docker(cli.Args(args...))
	if result.Error != nil {
		return result.Combined(), result.ExitCode, result.Compare(icmd.Success)
	}
	return result.Combined(), result.ExitCode, result.Error
}

func findContainerIP(t *testing.T, id string, network string) string {
	t.Helper()
	out := cli.DockerCmd(t, "inspect", fmt.Sprintf("--format='{{ .NetworkSettings.Networks.%s.IPAddress }}'", network), id).Stdout()
	return strings.Trim(out, " \r\n'")
}

func getContainerCount(t *testing.T) int {
	t.Helper()
	const containers = "Containers:"

	result := icmd.RunCommand(dockerBinary, "info")
	result.Assert(t, icmd.Success)

	lines := strings.SplitSeq(result.Combined(), "\n")
	for line := range lines {
		if strings.Contains(line, containers) {
			output := strings.TrimSpace(line)
			output = strings.TrimPrefix(output, containers)
			output = strings.Trim(output, " ")
			containerCount, err := strconv.Atoi(output)
			assert.NilError(t, err)
			return containerCount
		}
	}
	return 0
}

func inspectFieldAndUnmarshall(t *testing.T, name, field string, output any) {
	t.Helper()
	str := inspectFieldJSON(t, name, field)
	err := json.Unmarshal([]byte(str), output)
	assert.Assert(t, err == nil, "failed to unmarshal: %v", err)
}

// Deprecated: use cli.Docker
func inspectFilter(name, filter string) (string, error) {
	format := fmt.Sprintf("{{%s}}", filter)
	result := icmd.RunCommand(dockerBinary, "inspect", "-f", format, name)
	if result.Error != nil || result.ExitCode != 0 {
		return "", fmt.Errorf("failed to inspect %s: %s", name, result.Combined())
	}
	return strings.TrimSpace(result.Combined()), nil
}

// Deprecated: use cli.Docker
func inspectField(t *testing.T, name, field string) string {
	t.Helper()
	out, err := inspectFilter(name, "."+field)
	assert.NilError(t, err)
	return out
}

// Deprecated: use cli.Docker
func inspectFieldJSON(t *testing.T, name, field string) string {
	t.Helper()
	out, err := inspectFilter(name, "json ."+field)
	assert.NilError(t, err)
	return out
}

var errMountNotFound = errors.New("mount point not found")

// Deprecated: use cli.Docker
func inspectMountPoint(name, destination string) (container.MountPoint, error) {
	out, err := inspectFilter(name, "json .Mounts")
	if err != nil {
		return container.MountPoint{}, err
	}

	var mp []container.MountPoint
	if err := json.Unmarshal([]byte(out), &mp); err != nil {
		return container.MountPoint{}, err
	}

	for _, c := range mp {
		if c.Destination == destination {
			return c, nil
		}
	}

	return container.MountPoint{}, errMountNotFound
}

func getIDByName(t *testing.T, name string) string {
	t.Helper()
	id, err := inspectFilter(name, ".Id")
	assert.NilError(t, err)
	return id
}

// Write `content` to the file at path `dst`, creating it if necessary,
// as well as any missing directories.
// The file is truncated if it already exists.
// Fail the test when error occurs.
func writeFile(dst, content string, t *testing.T) {
	t.Helper()
	// Create subdirectories if necessary
	assert.NilError(t, os.MkdirAll(path.Dir(dst), 0o700))
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	assert.NilError(t, err)
	defer f.Close()
	// Write content (truncate if it exists)
	_, err = io.Copy(f, strings.NewReader(content))
	assert.NilError(t, err)
}

// Return the contents of file at path `src`.
// Fail the test when error occurs.
func readFile(src string, t *testing.T) (content string) {
	t.Helper()
	data, err := os.ReadFile(src)
	assert.NilError(t, err)

	return string(data)
}

func containerStorageFile(containerID, basename string) string {
	return filepath.Join(testEnv.PlatformDefaults.ContainerStoragePath, containerID, basename)
}

// docker commands that use this function must be run with the '-d' switch.
func runCommandAndReadContainerFile(t *testing.T, filename string, command string, args ...string) []byte {
	t.Helper()
	result := icmd.RunCommand(command, args...)
	result.Assert(t, icmd.Success)
	contID := strings.TrimSpace(result.Combined())
	cli.WaitRun(t, contID)
	return readContainerFile(t, contID, filename)
}

func readContainerFile(t *testing.T, containerID, filename string) []byte {
	t.Helper()
	f, err := os.Open(containerStorageFile(containerID, filename))
	assert.NilError(t, err)
	defer f.Close()

	content, err := io.ReadAll(f)
	assert.NilError(t, err)
	return content
}

func readContainerFileWithExec(t *testing.T, containerID, filename string) []byte {
	t.Helper()
	result := icmd.RunCommand(dockerBinary, "exec", containerID, "cat", filename)
	result.Assert(t, icmd.Success)
	return []byte(result.Combined())
}

// daemonTime provides the current time on the daemon host
func daemonTime(t *testing.T) time.Time {
	t.Helper()
	if testEnv.IsLocalDaemon() {
		return time.Now()
	}
	apiClient, err := client.New(client.FromEnv)
	assert.NilError(t, err)
	defer apiClient.Close()

	result, err := apiClient.Info(testutil.GetContext(t), client.InfoOptions{})
	assert.NilError(t, err)
	info := result.Info

	dt, err := time.Parse(time.RFC3339Nano, info.SystemTime)
	assert.Assert(t, err == nil, "invalid time format in GET /info response")
	return dt
}

// daemonUnixTime returns the current time on the daemon host with nanoseconds precision.
// It return the time formatted how the client sends timestamps to the server.
func daemonUnixTime(t *testing.T) string {
	t.Helper()
	dt := daemonTime(t)
	return fmt.Sprintf("%d.%09d", dt.Unix(), int64(dt.Nanosecond()))
}

// appendBaseEnv appends the minimum set of environment variables to exec the
// docker cli binary for testing with correct configuration to the given env
// list.
func appendBaseEnv(isTLS bool, env ...string) []string {
	preserveList := []string{
		// preserve remote test host
		"DOCKER_HOST",

		// windows: requires preserving SystemRoot, otherwise dial tcp fails
		// with "GetAddrInfoW: A non-recoverable error occurred during a database lookup."
		"SystemRoot",

		// testing help text requires the $PATH to dockerd is set
		"PATH",
	}
	if isTLS {
		preserveList = append(preserveList, "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH")
	}

	for _, key := range preserveList {
		if val := os.Getenv(key); val != "" {
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return env
}

// waitInspect will wait for the specified container to have the specified string
// in the inspect output. It will wait until the specified timeout (in seconds)
// is reached.
//
// Deprecated: use cli.WaitFor
func waitInspect(name, expr, expected string, timeout time.Duration) error {
	return daemon.WaitInspectWithArgs(dockerBinary, name, expr, expected, timeout)
}

func getInspectBody(t *testing.T, version, id string) json.RawMessage {
	t.Helper()
	apiClient, err := client.New(client.FromEnv, client.WithAPIVersion(version))
	assert.NilError(t, err)
	defer apiClient.Close()
	inspect, err := apiClient.ContainerInspect(testutil.GetContext(t), id, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	return inspect.Raw
}

// Run a long running idle task in a background container using the
// system-specific default image and command.
func runSleepingContainer(t *testing.T, extraArgs ...string) string {
	t.Helper()
	return runSleepingContainerInImage(t, "busybox", extraArgs...)
}

// Run a long running idle task in a background container using the specified
// image and the system-specific command.
func runSleepingContainerInImage(t *testing.T, image string, extraArgs ...string) string {
	t.Helper()
	args := []string{"run", "-d"}
	args = append(args, extraArgs...)
	args = append(args, image)
	args = append(args, sleepCommandForDaemonPlatform()...)
	return strings.TrimSpace(cli.DockerCmd(t, args...).Combined())
}

// minimalBaseImage returns the name of the minimal base image for the current
// daemon platform.
func minimalBaseImage() string {
	return testEnv.PlatformDefaults.BaseImage
}

func getGoroutineNumber(ctx context.Context, apiClient client.APIClient) (int, error) {
	result, err := apiClient.Info(ctx, client.InfoOptions{})
	if err != nil {
		return 0, err
	}
	return result.Info.NGoroutines, nil
}

func waitForStableGoroutineCount(ctx context.Context, t poll.TestingT, apiClient client.APIClient) int {
	var out int
	poll.WaitOn(t, stableGoroutineCount(ctx, apiClient, &out), poll.WithDelay(time.Second), poll.WithTimeout(30*time.Second))
	return out
}

func stableGoroutineCount(ctx context.Context, apiClient client.APIClient, count *int) poll.Check {
	var (
		numStable int
		nRoutines int
	)

	return func(t poll.LogT) poll.Result {
		n, err := getGoroutineNumber(ctx, apiClient)
		if err != nil {
			return poll.Error(err)
		}

		last := nRoutines

		if nRoutines == n {
			numStable++
		} else {
			numStable = 0
			nRoutines = n
		}

		if numStable > 6 {
			*count = n
			return poll.Success()
		}
		return poll.Continue("goroutine count is not stable: last %d, current %d, stable iters: %d", last, n, numStable)
	}
}

func checkGoroutineCount(ctx context.Context, apiClient client.APIClient, expected int) poll.Check {
	first := true
	return func(t poll.LogT) poll.Result {
		n, err := getGoroutineNumber(ctx, apiClient)
		if err != nil {
			return poll.Error(err)
		}
		if n > expected {
			if first {
				t.Log("Waiting for goroutines to stabilize")
				first = false
			}
			return poll.Continue("expected %d goroutines, got %d", expected, n)
		}
		return poll.Success()
	}
}

func waitForGoroutines(ctx context.Context, t poll.TestingT, apiClient client.APIClient, expected int) {
	poll.WaitOn(t, checkGoroutineCount(ctx, apiClient, expected), poll.WithDelay(500*time.Millisecond), poll.WithTimeout(30*time.Second))
}

// getErrorMessage returns the error message from an error API response
func getErrorMessage(t *testing.T, body []byte) string {
	t.Helper()
	var resp common.ErrorResponse
	assert.NilError(t, json.Unmarshal(body, &resp))
	return strings.TrimSpace(resp.Message)
}

type (
	checkF  func(*testing.T) (any, string)
	reducer func(...any) any
)

func pollCheck(t *testing.T, f checkF, compare func(x any) assert.BoolOrComparison) poll.Check {
	return func(poll.LogT) poll.Result {
		t.Helper()
		v, comment := f(t)
		r := compare(v)
		switch r := r.(type) {
		case bool:
			if r {
				return poll.Success()
			}
		case is.Comparison:
			if r().Success() {
				return poll.Success()
			}
		default:
			panic(fmt.Errorf("pollCheck: type %T not implemented", r))
		}
		return poll.Continue("%v", comment)
	}
}

func reducedCheck(r reducer, funcs ...checkF) checkF {
	return func(t *testing.T) (any, string) {
		t.Helper()
		var values []any
		var comments []string
		for _, f := range funcs {
			v, comment := f(t)
			values = append(values, v)
			if comment != "" {
				comments = append(comments, comment)
			}
		}
		return r(values...), fmt.Sprintf("%v", strings.Join(comments, ", "))
	}
}

func sumAsIntegers(vals ...any) any {
	var s int
	for _, v := range vals {
		s += v.(int)
	}
	return s
}

func loadSpecialImage(t *testing.T, imageFunc specialimage.SpecialImageFunc) string {
	tmpDir := t.TempDir()

	imgDir := filepath.Join(tmpDir, "image")
	assert.NilError(t, os.Mkdir(imgDir, 0o755))

	_, err := imageFunc(imgDir)
	assert.NilError(t, err)

	rc, err := archive.TarWithOptions(imgDir, &archive.TarOptions{})
	assert.NilError(t, err)
	defer rc.Close()

	imgTar := filepath.Join(tmpDir, "image.tar")
	tarFile, err := os.OpenFile(imgTar, os.O_CREATE|os.O_WRONLY, 0o644)
	assert.NilError(t, err)

	defer tarFile.Close()

	_, err = io.Copy(tarFile, rc)
	assert.NilError(t, err)

	tarFile.Close()

	out := cli.DockerCmd(t, "load", "-i", imgTar).Stdout()

	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)

		if _, imageID, hasID := strings.Cut(line, "Loaded image ID: "); hasID {
			return imageID
		}
		if _, imageRef, hasRef := strings.Cut(line, "Loaded image: "); hasRef {
			return imageRef
		}
	}

	t.Fatalf("failed to extract image ref from %q", out)
	return ""
}
