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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/internal/testutils/specialimage"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/testutil"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
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

// Deprecated: use cli.Docker or cli.DockerCmd
func dockerCmdWithResult(args ...string) *icmd.Result {
	return cli.Docker(cli.Args(args...))
}

func findContainerIP(c *testing.T, id string, network string) string {
	c.Helper()
	out := cli.DockerCmd(c, "inspect", fmt.Sprintf("--format='{{ .NetworkSettings.Networks.%s.IPAddress }}'", network), id).Stdout()
	return strings.Trim(out, " \r\n'")
}

func getContainerCount(c *testing.T) int {
	c.Helper()
	const containers = "Containers:"

	result := icmd.RunCommand(dockerBinary, "info")
	result.Assert(c, icmd.Success)

	lines := strings.Split(result.Combined(), "\n")
	for _, line := range lines {
		if strings.Contains(line, containers) {
			output := strings.TrimSpace(line)
			output = strings.TrimPrefix(output, containers)
			output = strings.Trim(output, " ")
			containerCount, err := strconv.Atoi(output)
			assert.NilError(c, err)
			return containerCount
		}
	}
	return 0
}

func inspectFieldAndUnmarshall(c *testing.T, name, field string, output interface{}) {
	c.Helper()
	str := inspectFieldJSON(c, name, field)
	err := json.Unmarshal([]byte(str), output)
	assert.Assert(c, err == nil, "failed to unmarshal: %v", err)
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
func inspectFieldWithError(name, field string) (string, error) {
	return inspectFilter(name, "."+field)
}

// Deprecated: use cli.Docker
func inspectField(c *testing.T, name, field string) string {
	c.Helper()
	out, err := inspectFilter(name, "."+field)
	assert.NilError(c, err)
	return out
}

// Deprecated: use cli.Docker
func inspectFieldJSON(c *testing.T, name, field string) string {
	c.Helper()
	out, err := inspectFilter(name, "json ."+field)
	assert.NilError(c, err)
	return out
}

// Deprecated: use cli.Docker
func inspectFieldMap(c *testing.T, name, path, field string) string {
	c.Helper()
	out, err := inspectFilter(name, fmt.Sprintf("index .%s %q", path, field))
	assert.NilError(c, err)
	return out
}

// Deprecated: use cli.Docker
func inspectMountSourceField(name, destination string) (string, error) {
	m, err := inspectMountPoint(name, destination)
	if err != nil {
		return "", err
	}
	return m.Source, nil
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

func getIDByName(c *testing.T, name string) string {
	c.Helper()
	id, err := inspectFieldWithError(name, "Id")
	assert.NilError(c, err)
	return id
}

// Deprecated: use cli.Docker
func buildImageSuccessfully(c *testing.T, name string, cmdOperators ...cli.CmdOperator) {
	c.Helper()
	buildImage(name, cmdOperators...).Assert(c, icmd.Success)
}

// Deprecated: use cli.Docker
func buildImage(name string, cmdOperators ...cli.CmdOperator) *icmd.Result {
	return cli.Docker(cli.Args("build", "-t", name), cmdOperators...)
}

// Write `content` to the file at path `dst`, creating it if necessary,
// as well as any missing directories.
// The file is truncated if it already exists.
// Fail the test when error occurs.
func writeFile(dst, content string, c *testing.T) {
	c.Helper()
	// Create subdirectories if necessary
	assert.NilError(c, os.MkdirAll(path.Dir(dst), 0o700))
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	assert.NilError(c, err)
	defer f.Close()
	// Write content (truncate if it exists)
	_, err = io.Copy(f, strings.NewReader(content))
	assert.NilError(c, err)
}

// Return the contents of file at path `src`.
// Fail the test when error occurs.
func readFile(src string, c *testing.T) (content string) {
	c.Helper()
	data, err := os.ReadFile(src)
	assert.NilError(c, err)

	return string(data)
}

func containerStorageFile(containerID, basename string) string {
	return filepath.Join(testEnv.PlatformDefaults.ContainerStoragePath, containerID, basename)
}

// docker commands that use this function must be run with the '-d' switch.
func runCommandAndReadContainerFile(c *testing.T, filename string, command string, args ...string) []byte {
	c.Helper()
	result := icmd.RunCommand(command, args...)
	result.Assert(c, icmd.Success)
	contID := strings.TrimSpace(result.Combined())
	cli.WaitRun(c, contID)
	return readContainerFile(c, contID, filename)
}

func readContainerFile(c *testing.T, containerID, filename string) []byte {
	c.Helper()
	f, err := os.Open(containerStorageFile(containerID, filename))
	assert.NilError(c, err)
	defer f.Close()

	content, err := io.ReadAll(f)
	assert.NilError(c, err)
	return content
}

func readContainerFileWithExec(c *testing.T, containerID, filename string) []byte {
	c.Helper()
	result := icmd.RunCommand(dockerBinary, "exec", containerID, "cat", filename)
	result.Assert(c, icmd.Success)
	return []byte(result.Combined())
}

// daemonTime provides the current time on the daemon host
func daemonTime(c *testing.T) time.Time {
	c.Helper()
	if testEnv.IsLocalDaemon() {
		return time.Now()
	}
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	info, err := apiClient.Info(testutil.GetContext(c))
	assert.NilError(c, err)

	dt, err := time.Parse(time.RFC3339Nano, info.SystemTime)
	assert.Assert(c, err == nil, "invalid time format in GET /info response")
	return dt
}

// daemonUnixTime returns the current time on the daemon host with nanoseconds precision.
// It return the time formatted how the client sends timestamps to the server.
func daemonUnixTime(c *testing.T) string {
	c.Helper()
	return parseEventTime(daemonTime(c))
}

func parseEventTime(t time.Time) string {
	return fmt.Sprintf("%d.%09d", t.Unix(), int64(t.Nanosecond()))
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

func createTmpFile(c *testing.T, content string) string {
	c.Helper()
	f, err := os.CreateTemp("", "testfile")
	assert.NilError(c, err)

	filename := f.Name()

	err = os.WriteFile(filename, []byte(content), 0o644)
	assert.NilError(c, err)

	return filename
}

// waitInspect will wait for the specified container to have the specified string
// in the inspect output. It will wait until the specified timeout (in seconds)
// is reached.
// Deprecated: use cli.WaitFor
func waitInspect(name, expr, expected string, timeout time.Duration) error {
	return daemon.WaitInspectWithArgs(dockerBinary, name, expr, expected, timeout)
}

func getInspectBody(c *testing.T, version, id string) []byte {
	c.Helper()
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion(version))
	assert.NilError(c, err)
	defer apiClient.Close()
	_, body, err := apiClient.ContainerInspectWithRaw(testutil.GetContext(c), id, false)
	assert.NilError(c, err)
	return body
}

// Run a long running idle task in a background container using the
// system-specific default image and command.
func runSleepingContainer(c *testing.T, extraArgs ...string) string {
	c.Helper()
	return runSleepingContainerInImage(c, "busybox", extraArgs...)
}

// Run a long running idle task in a background container using the specified
// image and the system-specific command.
func runSleepingContainerInImage(c *testing.T, image string, extraArgs ...string) string {
	c.Helper()
	args := []string{"run", "-d"}
	args = append(args, extraArgs...)
	args = append(args, image)
	args = append(args, sleepCommandForDaemonPlatform()...)
	return strings.TrimSpace(cli.DockerCmd(c, args...).Combined())
}

// minimalBaseImage returns the name of the minimal base image for the current
// daemon platform.
func minimalBaseImage() string {
	return testEnv.PlatformDefaults.BaseImage
}

func getGoroutineNumber(ctx context.Context, apiClient client.APIClient) (int, error) {
	info, err := apiClient.Info(ctx)
	if err != nil {
		return 0, err
	}
	return info.NGoroutines, nil
}

func waitForStableGourtineCount(ctx context.Context, t poll.TestingT, apiClient client.APIClient) int {
	var out int
	poll.WaitOn(t, stableGoroutineCount(ctx, apiClient, &out), poll.WithTimeout(30*time.Second))
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

		if numStable > 3 {
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
func getErrorMessage(c *testing.T, body []byte) string {
	c.Helper()
	var resp types.ErrorResponse
	assert.NilError(c, json.Unmarshal(body, &resp))
	return strings.TrimSpace(resp.Message)
}

type (
	checkF  func(*testing.T) (interface{}, string)
	reducer func(...interface{}) interface{}
)

func pollCheck(t *testing.T, f checkF, compare func(x interface{}) assert.BoolOrComparison) poll.Check {
	return func(poll.LogT) poll.Result {
		t.Helper()
		v, comment := f(t)
		r := compare(v)
		switch r := r.(type) {
		case bool:
			if r {
				return poll.Success()
			}
		case cmp.Comparison:
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
	return func(c *testing.T) (interface{}, string) {
		c.Helper()
		var values []interface{}
		var comments []string
		for _, f := range funcs {
			v, comment := f(c)
			values = append(values, v)
			if len(comment) > 0 {
				comments = append(comments, comment)
			}
		}
		return r(values...), fmt.Sprintf("%v", strings.Join(comments, ", "))
	}
}

func sumAsIntegers(vals ...interface{}) interface{} {
	var s int
	for _, v := range vals {
		s += v.(int)
	}
	return s
}

func loadSpecialImage(c *testing.T, imageFunc specialimage.SpecialImageFunc) string {
	tmpDir := c.TempDir()

	imgDir := filepath.Join(tmpDir, "image")
	assert.NilError(c, os.Mkdir(imgDir, 0o755))

	_, err := imageFunc(imgDir)
	assert.NilError(c, err)

	rc, err := archive.TarWithOptions(imgDir, &archive.TarOptions{})
	assert.NilError(c, err)
	defer rc.Close()

	imgTar := filepath.Join(tmpDir, "image.tar")
	tarFile, err := os.OpenFile(imgTar, os.O_CREATE|os.O_WRONLY, 0o644)
	assert.NilError(c, err)

	defer tarFile.Close()

	_, err = io.Copy(tarFile, rc)
	assert.NilError(c, err)

	tarFile.Close()

	out := cli.DockerCmd(c, "load", "-i", imgTar).Stdout()

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)

		if _, imageID, hasID := strings.Cut(line, "Loaded image ID: "); hasID {
			return imageID
		}
		if _, imageRef, hasRef := strings.Cut(line, "Loaded image: "); hasRef {
			return imageRef
		}
	}

	c.Fatalf("failed to extract image ref from %q", out)
	return ""
}
