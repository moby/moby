package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

type DockerCLIVolumeSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIVolumeSuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerCLIVolumeSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLIVolumeSuite) TestVolumeCLICreate(c *testing.T) {
	cli.DockerCmd(c, "volume", "create")

	_, _, err := dockerCmdWithError("volume", "create", "-d", "nosuchdriver")
	assert.ErrorContains(c, err, "")

	// test using hidden --name option
	name := cli.DockerCmd(c, "volume", "create", "--name=test").Stdout()
	name = strings.TrimSpace(name)
	assert.Equal(c, name, "test")

	name = cli.DockerCmd(c, "volume", "create", "test2").Stdout()
	name = strings.TrimSpace(name)
	assert.Equal(c, name, "test2")
}

func (s *DockerCLIVolumeSuite) TestVolumeCLIInspect(c *testing.T) {
	assert.Assert(c, exec.Command(dockerBinary, "volume", "inspect", "doesnotexist").Run() != nil, "volume inspect should error on non-existent volume")
	name := cli.DockerCmd(c, "volume", "create").Stdout()
	name = strings.TrimSpace(name)
	out := cli.DockerCmd(c, "volume", "inspect", "--format={{ .Name }}", name).Stdout()
	assert.Equal(c, strings.TrimSpace(out), name)

	cli.DockerCmd(c, "volume", "create", "test")
	out = cli.DockerCmd(c, "volume", "inspect", "--format={{ .Name }}", "test").Stdout()
	assert.Equal(c, strings.TrimSpace(out), "test")
}

func (s *DockerCLIVolumeSuite) TestVolumeCLIInspectMulti(c *testing.T) {
	cli.DockerCmd(c, "volume", "create", "test1")
	cli.DockerCmd(c, "volume", "create", "test2")
	cli.DockerCmd(c, "volume", "create", "test3")

	result := dockerCmdWithResult("volume", "inspect", "--format={{ .Name }}", "test1", "test2", "doesnotexist", "test3")
	result.Assert(c, icmd.Expected{
		ExitCode: 1,
		Err:      "No such volume: doesnotexist",
	})

	out := result.Stdout()
	assert.Assert(c, is.Contains(out, "test1"))
	assert.Assert(c, is.Contains(out, "test2"))
	assert.Assert(c, is.Contains(out, "test3"))
}

func (s *DockerCLIVolumeSuite) TestVolumeCLILs(c *testing.T) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	cli.DockerCmd(c, "volume", "create", "aaa")

	cli.DockerCmd(c, "volume", "create", "test")

	cli.DockerCmd(c, "volume", "create", "soo")
	cli.DockerCmd(c, "run", "-v", "soo:"+prefix+"/foo", "busybox", "ls", "/")

	out := cli.DockerCmd(c, "volume", "ls", "-q").Stdout()
	assertVolumesInList(c, out, []string{"aaa", "soo", "test"})
}

func (s *DockerCLIVolumeSuite) TestVolumeLsFormat(c *testing.T) {
	cli.DockerCmd(c, "volume", "create", "aaa")
	cli.DockerCmd(c, "volume", "create", "test")
	cli.DockerCmd(c, "volume", "create", "soo")

	out := cli.DockerCmd(c, "volume", "ls", "--format", "{{.Name}}").Stdout()
	assertVolumesInList(c, out, []string{"aaa", "soo", "test"})
}

func (s *DockerCLIVolumeSuite) TestVolumeLsFormatDefaultFormat(c *testing.T) {
	cli.DockerCmd(c, "volume", "create", "aaa")
	cli.DockerCmd(c, "volume", "create", "test")
	cli.DockerCmd(c, "volume", "create", "soo")

	const config = `{
		"volumesFormat": "{{ .Name }} default"
}`
	d, err := os.MkdirTemp("", "integration-cli-")
	assert.NilError(c, err)
	defer os.RemoveAll(d)

	err = os.WriteFile(filepath.Join(d, "config.json"), []byte(config), 0o644)
	assert.NilError(c, err)

	out := cli.DockerCmd(c, "--config", d, "volume", "ls").Stdout()
	assertVolumesInList(c, out, []string{"aaa default", "soo default", "test default"})
}

func assertVolumesInList(c *testing.T, out string, expected []string) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
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

func (s *DockerCLIVolumeSuite) TestVolumeCLILsFilterDangling(c *testing.T) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	cli.DockerCmd(c, "volume", "create", "testnotinuse1")
	cli.DockerCmd(c, "volume", "create", "testisinuse1")
	cli.DockerCmd(c, "volume", "create", "testisinuse2")

	// Make sure both "created" (but not started), and started
	// containers are included in reference counting
	cli.DockerCmd(c, "run", "--name", "volume-test1", "-v", "testisinuse1:"+prefix+"/foo", "busybox", "true")
	cli.DockerCmd(c, "create", "--name", "volume-test2", "-v", "testisinuse2:"+prefix+"/foo", "busybox", "true")

	// No filter, all volumes should show
	out := cli.DockerCmd(c, "volume", "ls").Stdout()
	assert.Assert(c, strings.Contains(out, "testnotinuse1\n"), "expected volume 'testnotinuse1' in output")
	assert.Assert(c, strings.Contains(out, "testisinuse1\n"), "expected volume 'testisinuse1' in output")
	assert.Assert(c, strings.Contains(out, "testisinuse2\n"), "expected volume 'testisinuse2' in output")

	// Explicitly disabling dangling
	out = cli.DockerCmd(c, "volume", "ls", "--filter", "dangling=false").Stdout()
	assert.Assert(c, !strings.Contains(out, "testnotinuse1\n"), "expected volume 'testnotinuse1' in output")
	assert.Assert(c, strings.Contains(out, "testisinuse1\n"), "expected volume 'testisinuse1' in output")
	assert.Assert(c, strings.Contains(out, "testisinuse2\n"), "expected volume 'testisinuse2' in output")

	// Filter "dangling" volumes; only "dangling" (unused) volumes should be in the output
	out = cli.DockerCmd(c, "volume", "ls", "--filter", "dangling=true").Stdout()
	assert.Assert(c, strings.Contains(out, "testnotinuse1\n"), "expected volume 'testnotinuse1' in output")
	assert.Assert(c, !strings.Contains(out, "testisinuse1\n"), "volume 'testisinuse1' in output, but not expected")
	assert.Assert(c, !strings.Contains(out, "testisinuse2\n"), "volume 'testisinuse2' in output, but not expected")

	// Filter "dangling" volumes; only "dangling" (unused) volumes should be in the output, dangling also accept 1
	out = cli.DockerCmd(c, "volume", "ls", "--filter", "dangling=1").Stdout()
	assert.Assert(c, strings.Contains(out, "testnotinuse1\n"), "expected volume 'testnotinuse1' in output")
	assert.Assert(c, !strings.Contains(out, "testisinuse1\n"), "volume 'testisinuse1' in output, but not expected")
	assert.Assert(c, !strings.Contains(out, "testisinuse2\n"), "volume 'testisinuse2' in output, but not expected")

	// dangling=0 is same as dangling=false case
	out = cli.DockerCmd(c, "volume", "ls", "--filter", "dangling=0").Stdout()
	assert.Assert(c, !strings.Contains(out, "testnotinuse1\n"), "expected volume 'testnotinuse1' in output")
	assert.Assert(c, strings.Contains(out, "testisinuse1\n"), "expected volume 'testisinuse1' in output")
	assert.Assert(c, strings.Contains(out, "testisinuse2\n"), "expected volume 'testisinuse2' in output")

	out = cli.DockerCmd(c, "volume", "ls", "--filter", "name=testisin").Stdout()
	assert.Assert(c, !strings.Contains(out, "testnotinuse1\n"), "expected volume 'testnotinuse1' in output")
	assert.Assert(c, strings.Contains(out, "testisinuse1\n"), "expected volume 'testisinuse1' in output")
	assert.Assert(c, strings.Contains(out, "testisinuse2\n"), "expected volume 'testisinuse2' in output")
}

func (s *DockerCLIVolumeSuite) TestVolumeCLILsErrorWithInvalidFilterName(c *testing.T) {
	out, _, err := dockerCmdWithError("volume", "ls", "-f", "FOO=123")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, is.Contains(out, "invalid filter"))
}

func (s *DockerCLIVolumeSuite) TestVolumeCLILsWithIncorrectFilterValue(c *testing.T) {
	out, _, err := dockerCmdWithError("volume", "ls", "-f", "dangling=invalid")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, is.Contains(out, "invalid filter"))
}

func (s *DockerCLIVolumeSuite) TestVolumeCLIRm(c *testing.T) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	id := cli.DockerCmd(c, "volume", "create").Stdout()
	id = strings.TrimSpace(id)

	cli.DockerCmd(c, "volume", "create", "test")
	cli.DockerCmd(c, "volume", "rm", id)
	cli.DockerCmd(c, "volume", "rm", "test")

	volumeID := "testing"
	cli.DockerCmd(c, "run", "-v", volumeID+":"+prefix+"/foo", "--name=test", "busybox", "sh", "-c", "echo hello > /foo/bar")

	icmd.RunCommand(dockerBinary, "volume", "rm", "testing").Assert(c, icmd.Expected{
		ExitCode: 1,
		Error:    "exit status 1",
	})

	out := cli.DockerCmd(c, "run", "--volumes-from=test", "--name=test2", "busybox", "sh", "-c", "cat /foo/bar").Combined()
	assert.Equal(c, strings.TrimSpace(out), "hello")
	cli.DockerCmd(c, "rm", "-fv", "test2")
	cli.DockerCmd(c, "volume", "inspect", volumeID)
	cli.DockerCmd(c, "rm", "-f", "test")

	out = cli.DockerCmd(c, "run", "--name=test2", "-v", volumeID+":"+prefix+"/foo", "busybox", "sh", "-c", "cat /foo/bar").Combined()
	assert.Equal(c, strings.TrimSpace(out), "hello", "volume data was removed")
	cli.DockerCmd(c, "rm", "test2")

	cli.DockerCmd(c, "volume", "rm", volumeID)
	assert.Assert(c, exec.Command("volume", "rm", "doesnotexist").Run() != nil, "volume rm should fail with non-existent volume")
}

// FIXME(vdemeester) should be a unit test in cli/command/volume package
func (s *DockerCLIVolumeSuite) TestVolumeCLINoArgs(c *testing.T) {
	out := cli.DockerCmd(c, "volume").Combined()
	// no args should produce the cmd usage output
	usage := "Usage:	docker volume COMMAND"
	assert.Assert(c, is.Contains(out, usage))
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
	assert.Assert(c, is.Contains(result.Stderr(), "unknown flag: --no-such-flag"))
}

func (s *DockerCLIVolumeSuite) TestVolumeCLIInspectTmplError(c *testing.T) {
	name := cli.DockerCmd(c, "volume", "create").Stdout()
	name = strings.TrimSpace(name)

	out, exitCode, err := dockerCmdWithError("volume", "inspect", "--format='{{ .FooBar }}'", name)
	assert.Assert(c, err != nil, "Output: %s", out)
	assert.Equal(c, exitCode, 1, fmt.Sprintf("Output: %s", out))
	assert.Assert(c, is.Contains(out, "Template parsing error"))
}

func (s *DockerCLIVolumeSuite) TestVolumeCLICreateWithOpts(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	cli.DockerCmd(c, "volume", "create", "-d", "local", "test", "--opt=type=tmpfs", "--opt=device=tmpfs", "--opt=o=size=1m,uid=1000")

	out := cli.DockerCmd(c, "run", "-v", "test:/foo", "busybox", "mount").Stdout()
	mounts := strings.Split(out, "\n")
	var found bool
	for _, m := range mounts {
		if strings.Contains(m, "/foo") {
			found = true
			info := strings.Fields(m)
			// tmpfs on <path> type tmpfs (rw,relatime,size=1024k,uid=1000)
			assert.Equal(c, info[0], "tmpfs")
			assert.Equal(c, info[2], "/foo")
			assert.Equal(c, info[4], "tmpfs")
			assert.Assert(c, is.Contains(info[5], "uid=1000"))
			assert.Assert(c, is.Contains(info[5], "size=1024k"))
			break
		}
	}
	assert.Equal(c, found, true)
}

func (s *DockerCLIVolumeSuite) TestVolumeCLICreateLabel(c *testing.T) {
	const testVol = "testvolcreatelabel"
	const testLabel = "foo"
	const testValue = "bar"

	_, _, err := dockerCmdWithError("volume", "create", "--label", testLabel+"="+testValue, testVol)
	assert.NilError(c, err)

	out := cli.DockerCmd(c, "volume", "inspect", "--format={{ .Labels."+testLabel+" }}", testVol).Stdout()
	assert.Equal(c, strings.TrimSpace(out), testValue)
}

func (s *DockerCLIVolumeSuite) TestVolumeCLICreateLabelMultiple(c *testing.T) {
	const testVol = "testvolcreatelabel"

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
		out := cli.DockerCmd(c, "volume", "inspect", "--format={{ .Labels."+k+" }}", testVol).Stdout()
		assert.Equal(c, strings.TrimSpace(out), v)
	}
}

func (s *DockerCLIVolumeSuite) TestVolumeCLILsFilterLabels(c *testing.T) {
	testVol1 := "testvolcreatelabel-1"
	_, _, err := dockerCmdWithError("volume", "create", "--label", "foo=bar1", testVol1)
	assert.NilError(c, err)

	testVol2 := "testvolcreatelabel-2"
	_, _, err = dockerCmdWithError("volume", "create", "--label", "foo=bar2", testVol2)
	assert.NilError(c, err)

	// filter with label=key
	out := cli.DockerCmd(c, "volume", "ls", "--filter", "label=foo").Stdout()
	assert.Assert(c, strings.Contains(out, "testvolcreatelabel-1\n"), "expected volume 'testvolcreatelabel-1' in output")
	assert.Assert(c, strings.Contains(out, "testvolcreatelabel-2\n"), "expected volume 'testvolcreatelabel-2' in output")

	// filter with label=key=value
	out = cli.DockerCmd(c, "volume", "ls", "--filter", "label=foo=bar1").Stdout()
	assert.Assert(c, strings.Contains(out, "testvolcreatelabel-1\n"), "expected volume 'testvolcreatelabel-1' in output")
	assert.Assert(c, !strings.Contains(out, "testvolcreatelabel-2\n"), "expected volume 'testvolcreatelabel-2 in output")

	out = cli.DockerCmd(c, "volume", "ls", "--filter", "label=non-exist").Stdout()
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(c, len(outArr), 1, fmt.Sprintf("\n%s", out))

	out = cli.DockerCmd(c, "volume", "ls", "--filter", "label=foo=non-exist").Stdout()
	outArr = strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(c, len(outArr), 1, fmt.Sprintf("\n%s", out))
}

func (s *DockerCLIVolumeSuite) TestVolumeCLILsFilterDrivers(c *testing.T) {
	// using default volume driver local to create volumes
	testVol1 := "testvol-1"
	_, _, err := dockerCmdWithError("volume", "create", testVol1)
	assert.NilError(c, err)

	testVol2 := "testvol-2"
	_, _, err = dockerCmdWithError("volume", "create", testVol2)
	assert.NilError(c, err)

	// filter with driver=local
	out := cli.DockerCmd(c, "volume", "ls", "--filter", "driver=local").Stdout()
	assert.Assert(c, strings.Contains(out, "testvol-1\n"), "expected volume 'testvol-1' in output")
	assert.Assert(c, strings.Contains(out, "testvol-2\n"), "expected volume 'testvol-2' in output")

	// filter with driver=invaliddriver
	out = cli.DockerCmd(c, "volume", "ls", "--filter", "driver=invaliddriver").Stdout()
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(c, len(outArr), 1, fmt.Sprintf("\n%s", out))

	// filter with driver=loca
	out = cli.DockerCmd(c, "volume", "ls", "--filter", "driver=loca").Stdout()
	outArr = strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(c, len(outArr), 1, fmt.Sprintf("\n%s", out))

	// filter with driver=
	out = cli.DockerCmd(c, "volume", "ls", "--filter", "driver=").Stdout()
	outArr = strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(c, len(outArr), 1, fmt.Sprintf("\n%s", out))
}

func (s *DockerCLIVolumeSuite) TestVolumeCLIRmForceUsage(c *testing.T) {
	id := cli.DockerCmd(c, "volume", "create").Stdout()
	id = strings.TrimSpace(id)

	cli.DockerCmd(c, "volume", "rm", "-f", id)
	cli.DockerCmd(c, "volume", "rm", "--force", "nonexist")
}

func (s *DockerCLIVolumeSuite) TestVolumeCLIRmForce(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	const name = "test"
	id := cli.DockerCmd(c, "volume", "create", name).Stdout()
	id = strings.TrimSpace(id)
	assert.Equal(c, id, name)

	out := cli.DockerCmd(c, "volume", "inspect", "--format", "{{.Mountpoint}}", name).Stdout()
	assert.Assert(c, strings.TrimSpace(out) != "")
	// Mountpoint is in the form of "/var/lib/docker/volumes/.../_data", removing `/_data`
	path := strings.TrimSuffix(strings.TrimSpace(out), "/_data")
	icmd.RunCommand("rm", "-rf", path).Assert(c, icmd.Success)

	cli.DockerCmd(c, "volume", "rm", "-f", name)
	out = cli.DockerCmd(c, "volume", "ls").Stdout()
	assert.Assert(c, !strings.Contains(out, name))
	cli.DockerCmd(c, "volume", "create", name)
	out = cli.DockerCmd(c, "volume", "ls").Stdout()
	assert.Assert(c, is.Contains(out, name))
}

// TestVolumeCLIRmForceInUse verifies that repeated `docker volume rm -f` calls does not remove a volume
// if it is in use. Test case for https://github.com/docker/docker/issues/31446
func (s *DockerCLIVolumeSuite) TestVolumeCLIRmForceInUse(c *testing.T) {
	const name = "testvolume"
	id := cli.DockerCmd(c, "volume", "create", name).Stdout()
	id = strings.TrimSpace(id)
	assert.Equal(c, id, name)

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	cid := cli.DockerCmd(c, "create", "-v", "testvolume:"+prefix+slash+"foo", "busybox").Stdout()
	cid = strings.TrimSpace(cid)

	_, _, err := dockerCmdWithError("volume", "rm", "-f", name)
	assert.ErrorContains(c, err, "")
	assert.ErrorContains(c, err, "volume is in use")
	out := cli.DockerCmd(c, "volume", "ls").Stdout()
	assert.Assert(c, is.Contains(out, name))
	// The original issue did not _remove_ the volume from the list
	// the first time. But a second call to `volume rm` removed it.
	// Calling `volume rm` a second time to confirm it's not removed
	// when calling twice.
	_, _, err = dockerCmdWithError("volume", "rm", "-f", name)
	assert.ErrorContains(c, err, "")
	assert.ErrorContains(c, err, "volume is in use")
	out = cli.DockerCmd(c, "volume", "ls").Stdout()
	assert.Assert(c, is.Contains(out, name))
	// Verify removing the volume after the container is removed works
	e := cli.DockerCmd(c, "rm", cid).ExitCode
	assert.Equal(c, e, 0)

	e = cli.DockerCmd(c, "volume", "rm", "-f", name).ExitCode
	assert.Equal(c, e, 0)

	result := cli.DockerCmd(c, "volume", "ls")
	assert.Equal(c, result.ExitCode, 0)
	assert.Assert(c, !strings.Contains(result.Stdout(), name))
}

func (s *DockerCLIVolumeSuite) TestVolumeCliInspectWithVolumeOpts(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	// Without options
	name := "test1"
	cli.DockerCmd(c, "volume", "create", "-d", "local", name)
	out := cli.DockerCmd(c, "volume", "inspect", "--format={{ .Options }}", name).Stdout()
	assert.Assert(c, is.Contains(strings.TrimSpace(out), "map[]"))
	// With options
	name = "test2"
	k1, v1 := "type", "tmpfs"
	k2, v2 := "device", "tmpfs"
	k3, v3 := "o", "size=1m,uid=1000"
	cli.DockerCmd(c, "volume", "create", "-d", "local", name, "--opt", fmt.Sprintf("%s=%s", k1, v1), "--opt", fmt.Sprintf("%s=%s", k2, v2), "--opt", fmt.Sprintf("%s=%s", k3, v3))
	out = cli.DockerCmd(c, "volume", "inspect", "--format={{ .Options }}", name).Stdout()
	assert.Assert(c, is.Contains(strings.TrimSpace(out), fmt.Sprintf("%s:%s", k1, v1)))
	assert.Assert(c, is.Contains(strings.TrimSpace(out), fmt.Sprintf("%s:%s", k2, v2)))
	assert.Assert(c, is.Contains(strings.TrimSpace(out), fmt.Sprintf("%s:%s", k3, v3)))
}

// Test case (1) for 21845: duplicate targets for --volumes-from
func (s *DockerCLIVolumeSuite) TestDuplicateMountpointsForVolumesFrom(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	const imgName = "vimage"
	buildImageSuccessfully(c, imgName, build.WithDockerfile(`
		FROM busybox
		VOLUME ["/tmp/data"]`))

	cli.DockerCmd(c, "run", "--name=data1", imgName, "true")
	cli.DockerCmd(c, "run", "--name=data2", imgName, "true")

	data1 := cli.DockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data1").Stdout()
	data1 = strings.TrimSpace(data1)
	assert.Assert(c, data1 != "")

	data2 := cli.DockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data2").Stdout()
	data2 = strings.TrimSpace(data2)
	assert.Assert(c, data2 != "")

	// Both volume should exist
	out := cli.DockerCmd(c, "volume", "ls", "-q").Stdout()
	assert.Assert(c, is.Contains(strings.TrimSpace(out), data1))
	assert.Assert(c, is.Contains(strings.TrimSpace(out), data2))
	out, _, err := dockerCmdWithError("run", "--name=app", "--volumes-from=data1", "--volumes-from=data2", "-d", "busybox", "top")
	assert.Assert(c, err == nil, "Out: %s", out)

	// Only the second volume will be referenced, this is backward compatible
	out = cli.DockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "app").Stdout()
	assert.Equal(c, strings.TrimSpace(out), data2)

	cli.DockerCmd(c, "rm", "-f", "-v", "app")
	cli.DockerCmd(c, "rm", "-f", "-v", "data1")
	cli.DockerCmd(c, "rm", "-f", "-v", "data2")

	// Both volume should not exist
	out = cli.DockerCmd(c, "volume", "ls", "-q").Stdout()
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), data1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), data2))
}

// Test case (2) for 21845: duplicate targets for --volumes-from and -v (bind)
func (s *DockerCLIVolumeSuite) TestDuplicateMountpointsForVolumesFromAndBind(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	const imgName = "vimage"
	buildImageSuccessfully(c, imgName, build.WithDockerfile(`
                FROM busybox
                VOLUME ["/tmp/data"]`))

	cli.DockerCmd(c, "run", "--name=data1", imgName, "true")
	cli.DockerCmd(c, "run", "--name=data2", imgName, "true")

	data1 := cli.DockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data1").Stdout()
	data1 = strings.TrimSpace(data1)
	assert.Assert(c, data1 != "")

	data2 := cli.DockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data2").Stdout()
	data2 = strings.TrimSpace(data2)
	assert.Assert(c, data2 != "")

	// Both volume should exist
	out := cli.DockerCmd(c, "volume", "ls", "-q").Stdout()
	assert.Assert(c, is.Contains(strings.TrimSpace(out), data1))
	assert.Assert(c, is.Contains(strings.TrimSpace(out), data2))
	// /tmp/data is automatically created, because we are not using the modern mount API here
	out, _, err := dockerCmdWithError("run", "--name=app", "--volumes-from=data1", "--volumes-from=data2", "-v", "/tmp/data:/tmp/data", "-d", "busybox", "top")
	assert.Assert(c, err == nil, "Out: %s", out)

	// No volume will be referenced (mount is /tmp/data), this is backward compatible
	out = cli.DockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "app").Stdout()
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), data1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), data2))
	cli.DockerCmd(c, "rm", "-f", "-v", "app")
	cli.DockerCmd(c, "rm", "-f", "-v", "data1")
	cli.DockerCmd(c, "rm", "-f", "-v", "data2")

	// Both volume should not exist
	out = cli.DockerCmd(c, "volume", "ls", "-q").Stdout()
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), data1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), data2))
}

// Test case (3) for 21845: duplicate targets for --volumes-from and `Mounts` (API only)
func (s *DockerCLIVolumeSuite) TestDuplicateMountpointsForVolumesFromAndMounts(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	const imgName = "vimage"
	buildImageSuccessfully(c, imgName, build.WithDockerfile(`
                FROM busybox
                VOLUME ["/tmp/data"]`))

	cli.DockerCmd(c, "run", "--name=data1", imgName, "true")
	cli.DockerCmd(c, "run", "--name=data2", imgName, "true")

	data1 := cli.DockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data1").Stdout()
	data1 = strings.TrimSpace(data1)
	assert.Assert(c, data1 != "")

	data2 := cli.DockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "data2").Stdout()
	data2 = strings.TrimSpace(data2)
	assert.Assert(c, data2 != "")

	// Both volume should exist
	out := cli.DockerCmd(c, "volume", "ls", "-q").Stdout()
	assert.Assert(c, is.Contains(strings.TrimSpace(out), data1))
	assert.Assert(c, is.Contains(strings.TrimSpace(out), data2))
	err := os.MkdirAll("/tmp/data", 0o755)
	assert.NilError(c, err)

	// Mounts is available in API
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

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
	_, err = apiClient.ContainerCreate(testutil.GetContext(c), &config, &hostConfig, &network.NetworkingConfig{}, nil, "app")

	assert.NilError(c, err)

	// No volume will be referenced (mount is /tmp/data), this is backward compatible
	out = cli.DockerCmd(c, "inspect", "--format", "{{(index .Mounts 0).Name}}", "app").Stdout()
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), data1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), data2))
	cli.DockerCmd(c, "rm", "-f", "-v", "app")
	cli.DockerCmd(c, "rm", "-f", "-v", "data1")
	cli.DockerCmd(c, "rm", "-f", "-v", "data2")

	// Both volume should not exist
	out = cli.DockerCmd(c, "volume", "ls", "-q").Stdout()
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), data1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), data2))
}
