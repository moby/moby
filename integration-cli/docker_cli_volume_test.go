package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/go-check/check"
	"gotest.tools/assert"
	"gotest.tools/icmd"
)

func (s *DockerSuite) TestVolumeCLICreate(c *testing.T) {
	dockerCmd(c, "volume", "create")

	_, _, err := dockerCmdWithError("volume", "create", "-d", "nosuchdriver")
	assert.ErrorContains(c, err, "")

	// test using hidden --name option
	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	assert.Assert(c, name, checker.Equals, "test")

	out, _ = dockerCmd(c, "volume", "create", "test2")
	name = strings.TrimSpace(out)
	assert.Assert(c, name, checker.Equals, "test2")
}

func (s *DockerSuite) TestVolumeCLIInspect(c *testing.T) {
	assert.Assert(c, exec.Command(dockerBinary, "volume", "inspect", "doesnotexist").Run() != nil, check.Commentf("volume inspect should error on non-existent volume"))
	out, _ := dockerCmd(c, "volume", "create")
	name := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "volume", "inspect", "--format={{ .Name }}", name)
	assert.Assert(c, strings.TrimSpace(out), checker.Equals, name)

	dockerCmd(c, "volume", "create", "test")
	out, _ = dockerCmd(c, "volume", "inspect", "--format={{ .Name }}", "test")
	assert.Assert(c, strings.TrimSpace(out), checker.Equals, "test")
}

func (s *DockerSuite) TestVolumeCLIInspectMulti(c *testing.T) {
	dockerCmd(c, "volume", "create", "test1")
	dockerCmd(c, "volume", "create", "test2")
	dockerCmd(c, "volume", "create", "test3")

	result := dockerCmdWithResult("volume", "inspect", "--format={{ .Name }}", "test1", "test2", "doesnotexist", "test3")
	result.Assert(c, icmd.Expected{
		ExitCode: 1,
		Err:      "No such volume: doesnotexist",
	})

	out := result.Stdout()
	assert.Assert(c, out, checker.Contains, "test1")
	assert.Assert(c, out, checker.Contains, "test2")
	assert.Assert(c, out, checker.Contains, "test3")
}

func (s *DockerSuite) TestVolumeCLILs(c *testing.T) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	dockerCmd(c, "volume", "create", "aaa")

	dockerCmd(c, "volume", "create", "test")

	dockerCmd(c, "volume", "create", "soo")
	dockerCmd(c, "run", "-v", "soo:"+prefix+"/foo", "busybox", "ls", "/")

	out, _ := dockerCmd(c, "volume", "ls", "-q")
	assertVolumesInList(c, out, []string{"aaa", "soo", "test"})
}

func (s *DockerSuite) TestVolumeLsFormat(c *testing.T) {
	dockerCmd(c, "volume", "create", "aaa")
	dockerCmd(c, "volume", "create", "test")
	dockerCmd(c, "volume", "create", "soo")

	out, _ := dockerCmd(c, "volume", "ls", "--format", "{{.Name}}")
	assertVolumesInList(c, out, []string{"aaa", "soo", "test"})
}

func (s *DockerSuite) TestVolumeLsFormatDefaultFormat(c *testing.T) {
	dockerCmd(c, "volume", "create", "aaa")
	dockerCmd(c, "volume", "create", "test")
	dockerCmd(c, "volume", "create", "soo")

	config := `{
		"volumesFormat": "{{ .Name }} default"
}`
	d, err := ioutil.TempDir("", "integration-cli-")
	assert.NilError(c, err)
	defer os.RemoveAll(d)

	err = ioutil.WriteFile(filepath.Join(d, "config.json"), []byte(config), 0644)
	assert.NilError(c, err)

	out, _ := dockerCmd(c, "--config", d, "volume", "ls")
	assertVolumesInList(c, out, []string{"aaa default", "soo default", "test default"})
}

func assertVolumesInList(c *testing.T, out string, expected []string) {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, expect := range expected {
		found := false
		for _, v := range lines {
			found = v == expect
			if found {
				break
			}
		}
		assert.Assert(c, found, "Expected volume not found: %v, got: %v", expect, lines)
	}
}

func (s *DockerSuite) TestVolumeCLILsFilterDangling(c *testing.T) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	dockerCmd(c, "volume", "create", "testnotinuse1")
	dockerCmd(c, "volume", "create", "testisinuse1")
	dockerCmd(c, "volume", "create", "testisinuse2")

	// Make sure both "created" (but not started), and started
	// containers are included in reference counting
	dockerCmd(c, "run", "--name", "volume-test1", "-v", "testisinuse1:"+prefix+"/foo", "busybox", "true")
	dockerCmd(c, "create", "--name", "volume-test2", "-v", "testisinuse2:"+prefix+"/foo", "busybox", "true")

	out, _ := dockerCmd(c, "volume", "ls")

	// No filter, all volumes should show
	assert.Assert(c, out, checker.Contains, "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	assert.Assert(c, out, checker.Contains, "testisinuse1\n", check.Commentf("expected volume 'testisinuse1' in output"))
	assert.Assert(c, out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=false")

	// Explicitly disabling dangling
	assert.Assert(c, out, checker.Not(checker.Contains), "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	assert.Assert(c, out, checker.Contains, "testisinuse1\n", check.Commentf("expected volume 'testisinuse1' in output"))
	assert.Assert(c, out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=true")

	// Filter "dangling" volumes; only "dangling" (unused) volumes should be in the output
	assert.Assert(c, out, checker.Contains, "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	assert.Assert(c, out, checker.Not(checker.Contains), "testisinuse1\n", check.Commentf("volume 'testisinuse1' in output, but not expected"))
	assert.Assert(c, out, checker.Not(checker.Contains), "testisinuse2\n", check.Commentf("volume 'testisinuse2' in output, but not expected"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=1")
	// Filter "dangling" volumes; only "dangling" (unused) volumes should be in the output, dangling also accept 1
	assert.Assert(c, out, checker.Contains, "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	assert.Assert(c, out, checker.Not(checker.Contains), "testisinuse1\n", check.Commentf("volume 'testisinuse1' in output, but not expected"))
	assert.Assert(c, out, checker.Not(checker.Contains), "testisinuse2\n", check.Commentf("volume 'testisinuse2' in output, but not expected"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=0")
	// dangling=0 is same as dangling=false case
	assert.Assert(c, out, checker.Not(checker.Contains), "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	assert.Assert(c, out, checker.Contains, "testisinuse1\n", check.Commentf("expected volume 'testisinuse1' in output"))
	assert.Assert(c, out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "name=testisin")
	assert.Assert(c, out, checker.Not(checker.Contains), "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	assert.Assert(c, out, checker.Contains, "testisinuse1\n", check.Commentf("expected volume 'testisinuse1' in output"))
	assert.Assert(c, out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))
}

func (s *DockerSuite) TestVolumeCLILsErrorWithInvalidFilterName(c *testing.T) {
	out, _, err := dockerCmdWithError("volume", "ls", "-f", "FOO=123")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, out, checker.Contains, "Invalid filter")
}

func (s *DockerSuite) TestVolumeCLILsWithIncorrectFilterValue(c *testing.T) {
	out, _, err := dockerCmdWithError("volume", "ls", "-f", "dangling=invalid")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, out, checker.Contains, "Invalid filter")
}

func (s *DockerSuite) TestVolumeCLIRm(c *testing.T) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	out, _ := dockerCmd(c, "volume", "create")
	id := strings.TrimSpace(out)

	dockerCmd(c, "volume", "create", "test")
	dockerCmd(c, "volume", "rm", id)
	dockerCmd(c, "volume", "rm", "test")

	volumeID := "testing"
	dockerCmd(c, "run", "-v", volumeID+":"+prefix+"/foo", "--name=test", "busybox", "sh", "-c", "echo hello > /foo/bar")

	icmd.RunCommand(dockerBinary, "volume", "rm", "testing").Assert(c, icmd.Expected{
		ExitCode: 1,
		Error:    "exit status 1",
	})

	out, _ = dockerCmd(c, "run", "--volumes-from=test", "--name=test2", "busybox", "sh", "-c", "cat /foo/bar")
	assert.Assert(c, strings.TrimSpace(out), checker.Equals, "hello")
	dockerCmd(c, "rm", "-fv", "test2")
	dockerCmd(c, "volume", "inspect", volumeID)
	dockerCmd(c, "rm", "-f", "test")

	out, _ = dockerCmd(c, "run", "--name=test2", "-v", volumeID+":"+prefix+"/foo", "busybox", "sh", "-c", "cat /foo/bar")
	assert.Assert(c, strings.TrimSpace(out), checker.Equals, "hello", check.Commentf("volume data was removed"))
	dockerCmd(c, "rm", "test2")

	dockerCmd(c, "volume", "rm", volumeID)
	assert.Assert(c, exec.Command("volume", "rm", "doesnotexist").Run() != nil, check.Commentf("volume rm should fail with non-existent volume"))
}

// FIXME(vdemeester) should be a unit test in cli/command/volume package
func (s *DockerSuite) TestVolumeCLINoArgs(c *testing.T) {
	out, _ := dockerCmd(c, "volume")
	// no args should produce the cmd usage output
	usage := "Usage:	docker volume COMMAND"
	assert.Assert(c, out, checker.Contains, usage)

	// invalid arg should error and show the command usage on stderr
	icmd.RunCommand(dockerBinary, "volume", "somearg").Assert(c, icmd.Expected{
		ExitCode: 1,
		Error:    "exit status 1",
		Err:      usage,
	})

	// invalid flag should error and show the flag error and cmd usage
	result := icmd.RunCommand(dockerBinary, "volume", "--no-such-flag")
	result.Assert(c, icmd.Expected{
		ExitCode: 125,
		Error:    "exit status 125",
		Err:      usage,
	})
	assert.Assert(c, result.Stderr(), checker.Contains, "unknown flag: --no-such-flag")
}

func (s *DockerSuite) TestVolumeCLIInspectTmplError(c *testing.T) {
	out, _ := dockerCmd(c, "volume", "create")
	name := strings.TrimSpace(out)

	out, exitCode, err := dockerCmdWithError("volume", "inspect", "--format='{{ .FooBar }}'", name)
	assert.Assert(c, err, checker.NotNil, check.Commentf("Output: %s", out))
	assert.Assert(c, exitCode, checker.Equals, 1, check.Commentf("Output: %s", out))
	assert.Assert(c, out, checker.Contains, "Template parsing error")
}

func (s *DockerSuite) TestVolumeCLICreateWithOpts(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	dockerCmd(c, "volume", "create", "-d", "local", "test", "--opt=type=tmpfs", "--opt=device=tmpfs", "--opt=o=size=1m,uid=1000")
	out, _ := dockerCmd(c, "run", "-v", "test:/foo", "busybox", "mount")

	mounts := strings.Split(out, "\n")
	var found bool
	for _, m := range mounts {
		if strings.Contains(m, "/foo") {
			found = true
			info := strings.Fields(m)
			// tmpfs on <path> type tmpfs (rw,relatime,size=1024k,uid=1000)
			assert.Assert(c, info[0], checker.Equals, "tmpfs")
			assert.Assert(c, info[2], checker.Equals, "/foo")
			assert.Assert(c, info[4], checker.Equals, "tmpfs")
			assert.Assert(c, info[5], checker.Contains, "uid=1000")
			assert.Assert(c, info[5], checker.Contains, "size=1024k")
			break
		}
	}
	assert.Assert(c, found, checker.Equals, true)
}

func (s *DockerSuite) TestVolumeCLICreateLabel(c *testing.T) {
	testVol := "testvolcreatelabel"
	testLabel := "foo"
	testValue := "bar"

	_, _, err := dockerCmdWithError("volume", "create", "--label", testLabel+"="+testValue, testVol)
	assert.NilError(c, err)

	out, _ := dockerCmd(c, "volume", "inspect", "--format={{ .Labels."+testLabel+" }}", testVol)
	assert.Assert(c, strings.TrimSpace(out), checker.Equals, testValue)
}

func (s *DockerSuite) TestVolumeCLICreateLabelMultiple(c *testing.T) {
	testVol := "testvolcreatelabel"

	testLabels := map[string]string{
		"foo": "bar",
		"baz": "foo",
	}

	args := []string{
		"volume",
		"create",
		testVol,
	}

	for k, v := range testLabels {
		args = append(args, "--label", k+"="+v)
	}

	_, _, err := dockerCmdWithError(args...)
	assert.NilError(c, err)

	for k, v := range testLabels {
		out, _ := dockerCmd(c, "volume", "inspect", "--format={{ .Labels."+k+" }}", testVol)
		assert.Assert(c, strings.TrimSpace(out), checker.Equals, v)
	}
}

func (s *DockerSuite) TestVolumeCLILsFilterLabels(c *testing.T) {
	testVol1 := "testvolcreatelabel-1"
	_, _, err := dockerCmdWithError("volume", "create", "--label", "foo=bar1", testVol1)
	assert.NilError(c, err)

	testVol2 := "testvolcreatelabel-2"
	_, _, err = dockerCmdWithError("volume", "create", "--label", "foo=bar2", testVol2)
	assert.NilError(c, err)

	out, _ := dockerCmd(c, "volume", "ls", "--filter", "label=foo")

	// filter with label=key
	assert.Assert(c, out, checker.Contains, "testvolcreatelabel-1\n", check.Commentf("expected volume 'testvolcreatelabel-1' in output"))
	assert.Assert(c, out, checker.Contains, "testvolcreatelabel-2\n", check.Commentf("expected volume 'testvolcreatelabel-2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "label=foo=bar1")

	// filter with label=key=value
	assert.Assert(c, out, checker.Contains, "testvolcreatelabel-1\n", check.Commentf("expected volume 'testvolcreatelabel-1' in output"))
	assert.Assert(c, out, checker.Not(checker.Contains), "testvolcreatelabel-2\n", check.Commentf("expected volume 'testvolcreatelabel-2 in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "label=non-exist")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	assert.Assert(c, len(outArr), checker.Equals, 1, check.Commentf("\n%s", out))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "label=foo=non-exist")
	outArr = strings.Split(strings.TrimSpace(out), "\n")
	assert.Assert(c, len(outArr), checker.Equals, 1, check.Commentf("\n%s", out))
}

func (s *DockerSuite) TestVolumeCLILsFilterDrivers(c *testing.T) {
	// using default volume driver local to create volumes
	testVol1 := "testvol-1"
	_, _, err := dockerCmdWithError("volume", "create", testVol1)
	assert.NilError(c, err)

	testVol2 := "testvol-2"
	_, _, err = dockerCmdWithError("volume", "create", testVol2)
	assert.NilError(c, err)

	// filter with driver=local
	out, _ := dockerCmd(c, "volume", "ls", "--filter", "driver=local")
	assert.Assert(c, out, checker.Contains, "testvol-1\n", check.Commentf("expected volume 'testvol-1' in output"))
	assert.Assert(c, out, checker.Contains, "testvol-2\n", check.Commentf("expected volume 'testvol-2' in output"))

	// filter with driver=invaliddriver
	out, _ = dockerCmd(c, "volume", "ls", "--filter", "driver=invaliddriver")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	assert.Assert(c, len(outArr), checker.Equals, 1, check.Commentf("\n%s", out))

	// filter with driver=loca
	out, _ = dockerCmd(c, "volume", "ls", "--filter", "driver=loca")
	outArr = strings.Split(strings.TrimSpace(out), "\n")
	assert.Assert(c, len(outArr), checker.Equals, 1, check.Commentf("\n%s", out))

	// filter with driver=
	out, _ = dockerCmd(c, "volume", "ls", "--filter", "driver=")
	outArr = strings.Split(strings.TrimSpace(out), "\n")
	assert.Assert(c, len(outArr), checker.Equals, 1, check.Commentf("\n%s", out))
}

func (s *DockerSuite) TestVolumeCLIRmForceUsage(c *testing.T) {
	out, _ := dockerCmd(c, "volume", "create")
	id := strings.TrimSpace(out)

	dockerCmd(c, "volume", "rm", "-f", id)
	dockerCmd(c, "volume", "rm", "--force", "nonexist")
}

func (s *DockerSuite) TestVolumeCLIRmForce(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	name := "test"
	out, _ := dockerCmd(c, "volume", "create", name)
	id := strings.TrimSpace(out)
	assert.Assert(c, id, checker.Equals, name)

	out, _ = dockerCmd(c, "volume", "inspect", "--format", "{{.Mountpoint}}", name)
	assert.Assert(c, strings.TrimSpace(out) != "")
	// Mountpoint is in the form of "/var/lib/docker/volumes/.../_data", removing `/_data`
	path := strings.TrimSuffix(strings.TrimSpace(out), "/_data")
	icmd.RunCommand("rm", "-rf", path).Assert(c, icmd.Success)

	dockerCmd(c, "volume", "rm", "-f", name)
	out, _ = dockerCmd(c, "volume", "ls")
	assert.Assert(c, out, checker.Not(checker.Contains), name)
	dockerCmd(c, "volume", "create", name)
	out, _ = dockerCmd(c, "volume", "ls")
	assert.Assert(c, out, checker.Contains, name)
}

// TestVolumeCLIRmForceInUse verifies that repeated `docker volume rm -f` calls does not remove a volume
// if it is in use. Test case for https://github.com/docker/docker/issues/31446
func (s *DockerSuite) TestVolumeCLIRmForceInUse(c *testing.T) {
	name := "testvolume"
	out, _ := dockerCmd(c, "volume", "create", name)
	id := strings.TrimSpace(out)
	assert.Assert(c, id, checker.Equals, name)

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	out, _ = dockerCmd(c, "create", "-v", "testvolume:"+prefix+slash+"foo", "busybox")
	cid := strings.TrimSpace(out)

	_, _, err := dockerCmdWithError("volume", "rm", "-f", name)
	assert.ErrorContains(c, err, "")
	assert.ErrorContains(c, err, "volume is in use")
	out, _ = dockerCmd(c, "volume", "ls")
	assert.Assert(c, out, checker.Contains, name)

	// The original issue did not _remove_ the volume from the list
	// the first time. But a second call to `volume rm` removed it.
	// Calling `volume rm` a second time to confirm it's not removed
	// when calling twice.
	_, _, err = dockerCmdWithError("volume", "rm", "-f", name)
	assert.ErrorContains(c, err, "")
	assert.ErrorContains(c, err, "volume is in use")
	out, _ = dockerCmd(c, "volume", "ls")
	assert.Assert(c, out, checker.Contains, name)

	// Verify removing the volume after the container is removed works
	_, e := dockerCmd(c, "rm", cid)
	assert.Assert(c, e, checker.Equals, 0)

	_, e = dockerCmd(c, "volume", "rm", "-f", name)
	assert.Assert(c, e, checker.Equals, 0)

	out, e = dockerCmd(c, "volume", "ls")
	assert.Assert(c, e, checker.Equals, 0)
	assert.Assert(c, out, checker.Not(checker.Contains), name)
}

func (s *DockerSuite) TestVolumeCliInspectWithVolumeOpts(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	// Without options
	name := "test1"
	dockerCmd(c, "volume", "create", "-d", "local", name)
	out, _ := dockerCmd(c, "volume", "inspect", "--format={{ .Options }}", name)
	assert.Assert(c, strings.TrimSpace(out), checker.Contains, "map[]")

	// With options
	name = "test2"
	k1, v1 := "type", "tmpfs"
	k2, v2 := "device", "tmpfs"
	k3, v3 := "o", "size=1m,uid=1000"
	dockerCmd(c, "volume", "create", "-d", "local", name, "--opt", fmt.Sprintf("%s=%s", k1, v1), "--opt", fmt.Sprintf("%s=%s", k2, v2), "--opt", fmt.Sprintf("%s=%s", k3, v3))
	out, _ = dockerCmd(c, "volume", "inspect", "--format={{ .Options }}", name)
	assert.Assert(c, strings.TrimSpace(out), checker.Contains, fmt.Sprintf("%s:%s", k1, v1))
	assert.Assert(c, strings.TrimSpace(out), checker.Contains, fmt.Sprintf("%s:%s", k2, v2))
	assert.Assert(c, strings.TrimSpace(out), checker.Contains, fmt.Sprintf("%s:%s", k3, v3))
}

// Test case (1) for 21845: duplicate targets for --volumes-from
func (s *DockerSuite) TestDuplicateMountpointsForVolumesFrom(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	image := "vimage"
	buildImageSuccessfully(c, image, build.WithDockerfile(`
		FROM busybox
		VOLUME ["/tmp/data"]`))

	dockerCmd(c, "run", "--name=data1", image, "true")
	dockerCmd(c, "run", "--name=data2", image, "true")

	out, _ := dockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data1")
	data1 := strings.TrimSpace(out)
	assert.Assert(c, data1, checker.Not(checker.Equals), "")

	out, _ = dockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data2")
	data2 := strings.TrimSpace(out)
	assert.Assert(c, data2, checker.Not(checker.Equals), "")

	// Both volume should exist
	out, _ = dockerCmd(c, "volume", "ls", "-q")
	assert.Assert(c, strings.TrimSpace(out), checker.Contains, data1)
	assert.Assert(c, strings.TrimSpace(out), checker.Contains, data2)

	out, _, err := dockerCmdWithError("run", "--name=app", "--volumes-from=data1", "--volumes-from=data2", "-d", "busybox", "top")
	assert.Assert(c, err, checker.IsNil, check.Commentf("Out: %s", out))

	// Only the second volume will be referenced, this is backward compatible
	out, _ = dockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "app")
	assert.Equal(c, strings.TrimSpace(out), data2)

	dockerCmd(c, "rm", "-f", "-v", "app")
	dockerCmd(c, "rm", "-f", "-v", "data1")
	dockerCmd(c, "rm", "-f", "-v", "data2")

	// Both volume should not exist
	out, _ = dockerCmd(c, "volume", "ls", "-q")
	assert.Assert(c, strings.TrimSpace(out), checker.Not(checker.Contains), data1)
	assert.Assert(c, strings.TrimSpace(out), checker.Not(checker.Contains), data2)
}

// Test case (2) for 21845: duplicate targets for --volumes-from and -v (bind)
func (s *DockerSuite) TestDuplicateMountpointsForVolumesFromAndBind(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	image := "vimage"
	buildImageSuccessfully(c, image, build.WithDockerfile(`
                FROM busybox
                VOLUME ["/tmp/data"]`))

	dockerCmd(c, "run", "--name=data1", image, "true")
	dockerCmd(c, "run", "--name=data2", image, "true")

	out, _ := dockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data1")
	data1 := strings.TrimSpace(out)
	assert.Assert(c, data1, checker.Not(checker.Equals), "")

	out, _ = dockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data2")
	data2 := strings.TrimSpace(out)
	assert.Assert(c, data2, checker.Not(checker.Equals), "")

	// Both volume should exist
	out, _ = dockerCmd(c, "volume", "ls", "-q")
	assert.Assert(c, strings.TrimSpace(out), checker.Contains, data1)
	assert.Assert(c, strings.TrimSpace(out), checker.Contains, data2)

	// /tmp/data is automatically created, because we are not using the modern mount API here
	out, _, err := dockerCmdWithError("run", "--name=app", "--volumes-from=data1", "--volumes-from=data2", "-v", "/tmp/data:/tmp/data", "-d", "busybox", "top")
	assert.Assert(c, err, checker.IsNil, check.Commentf("Out: %s", out))

	// No volume will be referenced (mount is /tmp/data), this is backward compatible
	out, _ = dockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "app")
	assert.Assert(c, strings.TrimSpace(out), checker.Not(checker.Contains), data1)
	assert.Assert(c, strings.TrimSpace(out), checker.Not(checker.Contains), data2)

	dockerCmd(c, "rm", "-f", "-v", "app")
	dockerCmd(c, "rm", "-f", "-v", "data1")
	dockerCmd(c, "rm", "-f", "-v", "data2")

	// Both volume should not exist
	out, _ = dockerCmd(c, "volume", "ls", "-q")
	assert.Assert(c, strings.TrimSpace(out), checker.Not(checker.Contains), data1)
	assert.Assert(c, strings.TrimSpace(out), checker.Not(checker.Contains), data2)
}

// Test case (3) for 21845: duplicate targets for --volumes-from and `Mounts` (API only)
func (s *DockerSuite) TestDuplicateMountpointsForVolumesFromAndMounts(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	image := "vimage"
	buildImageSuccessfully(c, image, build.WithDockerfile(`
                FROM busybox
                VOLUME ["/tmp/data"]`))

	dockerCmd(c, "run", "--name=data1", image, "true")
	dockerCmd(c, "run", "--name=data2", image, "true")

	out, _ := dockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data1")
	data1 := strings.TrimSpace(out)
	assert.Assert(c, data1, checker.Not(checker.Equals), "")

	out, _ = dockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data2")
	data2 := strings.TrimSpace(out)
	assert.Assert(c, data2, checker.Not(checker.Equals), "")

	// Both volume should exist
	out, _ = dockerCmd(c, "volume", "ls", "-q")
	assert.Assert(c, strings.TrimSpace(out), checker.Contains, data1)
	assert.Assert(c, strings.TrimSpace(out), checker.Contains, data2)

	err := os.MkdirAll("/tmp/data", 0755)
	assert.NilError(c, err)
	// Mounts is available in API
	cli, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer cli.Close()

	config := container.Config{
		Cmd:   []string{"top"},
		Image: "busybox",
	}

	hostConfig := container.HostConfig{
		VolumesFrom: []string{"data1", "data2"},
		Mounts: []mount.Mount{
			{
				Type:   "bind",
				Source: "/tmp/data",
				Target: "/tmp/data",
			},
		},
	}
	_, err = cli.ContainerCreate(context.Background(), &config, &hostConfig, &network.NetworkingConfig{}, "app")

	assert.NilError(c, err)

	// No volume will be referenced (mount is /tmp/data), this is backward compatible
	out, _ = dockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "app")
	assert.Assert(c, strings.TrimSpace(out), checker.Not(checker.Contains), data1)
	assert.Assert(c, strings.TrimSpace(out), checker.Not(checker.Contains), data2)

	dockerCmd(c, "rm", "-f", "-v", "app")
	dockerCmd(c, "rm", "-f", "-v", "data1")
	dockerCmd(c, "rm", "-f", "-v", "data2")

	// Both volume should not exist
	out, _ = dockerCmd(c, "volume", "ls", "-q")
	assert.Assert(c, strings.TrimSpace(out), checker.Not(checker.Contains), data1)
	assert.Assert(c, strings.TrimSpace(out), checker.Not(checker.Contains), data2)
}
