package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	icmd "github.com/docker/docker/pkg/integration/cmd"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestVolumeCLICreate(c *check.C) {
	dockerCmd(c, "volume", "create")

	_, _, err := dockerCmdWithError("volume", "create", "-d", "nosuchdriver")
	c.Assert(err, check.NotNil)

	// test using hidden --name option
	out, _ := dockerCmd(c, "volume", "create", "--name=test")
	name := strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test")

	out, _ = dockerCmd(c, "volume", "create", "test2")
	name = strings.TrimSpace(out)
	c.Assert(name, check.Equals, "test2")
}

func (s *DockerSuite) TestVolumeCLIInspect(c *check.C) {
	c.Assert(
		exec.Command(dockerBinary, "volume", "inspect", "doesntexist").Run(),
		check.Not(check.IsNil),
		check.Commentf("volume inspect should error on non-existent volume"),
	)

	out, _ := dockerCmd(c, "volume", "create")
	name := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "volume", "inspect", "--format={{ .Name }}", name)
	c.Assert(strings.TrimSpace(out), check.Equals, name)

	dockerCmd(c, "volume", "create", "test")
	out, _ = dockerCmd(c, "volume", "inspect", "--format={{ .Name }}", "test")
	c.Assert(strings.TrimSpace(out), check.Equals, "test")
}

func (s *DockerSuite) TestVolumeCLIInspectMulti(c *check.C) {
	dockerCmd(c, "volume", "create", "test1")
	dockerCmd(c, "volume", "create", "test2")
	dockerCmd(c, "volume", "create", "not-shown")

	result := dockerCmdWithResult("volume", "inspect", "--format={{ .Name }}", "test1", "test2", "doesntexist", "not-shown")
	c.Assert(result, icmd.Matches, icmd.Expected{
		ExitCode: 1,
		Err:      "No such volume: doesntexist",
	})

	out := result.Stdout()
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 2, check.Commentf("\n%s", out))

	c.Assert(out, checker.Contains, "test1")
	c.Assert(out, checker.Contains, "test2")
	c.Assert(out, checker.Not(checker.Contains), "not-shown")
}

func (s *DockerSuite) TestVolumeCLILs(c *check.C) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	dockerCmd(c, "volume", "create", "aaa")

	dockerCmd(c, "volume", "create", "test")

	dockerCmd(c, "volume", "create", "soo")
	dockerCmd(c, "run", "-v", "soo:"+prefix+"/foo", "busybox", "ls", "/")

	out, _ := dockerCmd(c, "volume", "ls")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 4, check.Commentf("\n%s", out))

	assertVolList(c, out, []string{"aaa", "soo", "test"})
}

func (s *DockerSuite) TestVolumeLsFormat(c *check.C) {
	dockerCmd(c, "volume", "create", "aaa")
	dockerCmd(c, "volume", "create", "test")
	dockerCmd(c, "volume", "create", "soo")

	out, _ := dockerCmd(c, "volume", "ls", "--format", "{{.Name}}")
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	expected := []string{"aaa", "soo", "test"}
	var names []string
	names = append(names, lines...)
	c.Assert(expected, checker.DeepEquals, names, check.Commentf("Expected array with truncated names: %v, got: %v", expected, names))
}

func (s *DockerSuite) TestVolumeLsFormatDefaultFormat(c *check.C) {
	dockerCmd(c, "volume", "create", "aaa")
	dockerCmd(c, "volume", "create", "test")
	dockerCmd(c, "volume", "create", "soo")

	config := `{
		"volumesFormat": "{{ .Name }} default"
}`
	d, err := ioutil.TempDir("", "integration-cli-")
	c.Assert(err, checker.IsNil)
	defer os.RemoveAll(d)

	err = ioutil.WriteFile(filepath.Join(d, "config.json"), []byte(config), 0644)
	c.Assert(err, checker.IsNil)

	out, _ := dockerCmd(c, "--config", d, "volume", "ls")
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	expected := []string{"aaa default", "soo default", "test default"}
	var names []string
	names = append(names, lines...)
	c.Assert(expected, checker.DeepEquals, names, check.Commentf("Expected array with truncated names: %v, got: %v", expected, names))
}

// assertVolList checks volume retrieved with ls command
// equals to expected volume list
// note: out should be `volume ls [option]` result
func assertVolList(c *check.C, out string, expectVols []string) {
	lines := strings.Split(out, "\n")
	var volList []string
	for _, line := range lines[1 : len(lines)-1] {
		volFields := strings.Fields(line)
		// wrap all volume name in volList
		volList = append(volList, volFields[1])
	}

	// volume ls should contains all expected volumes
	c.Assert(volList, checker.DeepEquals, expectVols)
}

func (s *DockerSuite) TestVolumeCLILsFilterDangling(c *check.C) {
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
	c.Assert(out, checker.Contains, "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse1\n", check.Commentf("expected volume 'testisinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=false")

	// Explicitly disabling dangling
	c.Assert(out, check.Not(checker.Contains), "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse1\n", check.Commentf("expected volume 'testisinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=true")

	// Filter "dangling" volumes; only "dangling" (unused) volumes should be in the output
	c.Assert(out, checker.Contains, "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, check.Not(checker.Contains), "testisinuse1\n", check.Commentf("volume 'testisinuse1' in output, but not expected"))
	c.Assert(out, check.Not(checker.Contains), "testisinuse2\n", check.Commentf("volume 'testisinuse2' in output, but not expected"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=1")
	// Filter "dangling" volumes; only "dangling" (unused) volumes should be in the output, dangling also accept 1
	c.Assert(out, checker.Contains, "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, check.Not(checker.Contains), "testisinuse1\n", check.Commentf("volume 'testisinuse1' in output, but not expected"))
	c.Assert(out, check.Not(checker.Contains), "testisinuse2\n", check.Commentf("volume 'testisinuse2' in output, but not expected"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "dangling=0")
	// dangling=0 is same as dangling=false case
	c.Assert(out, check.Not(checker.Contains), "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse1\n", check.Commentf("expected volume 'testisinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "name=testisin")
	c.Assert(out, check.Not(checker.Contains), "testnotinuse1\n", check.Commentf("expected volume 'testnotinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse1\n", check.Commentf("execpeted volume 'testisinuse1' in output"))
	c.Assert(out, checker.Contains, "testisinuse2\n", check.Commentf("expected volume 'testisinuse2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "driver=invalidDriver")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 1, check.Commentf("%s\n", out))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "driver=local")
	outArr = strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 4, check.Commentf("\n%s", out))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "driver=loc")
	outArr = strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 4, check.Commentf("\n%s", out))

}

func (s *DockerSuite) TestVolumeCLILsErrorWithInvalidFilterName(c *check.C) {
	out, _, err := dockerCmdWithError("volume", "ls", "-f", "FOO=123")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "Invalid filter")
}

func (s *DockerSuite) TestVolumeCLILsWithIncorrectFilterValue(c *check.C) {
	out, _, err := dockerCmdWithError("volume", "ls", "-f", "dangling=invalid")
	c.Assert(err, check.NotNil)
	c.Assert(out, checker.Contains, "Invalid filter")
}

func (s *DockerSuite) TestVolumeCLIRm(c *check.C) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	out, _ := dockerCmd(c, "volume", "create")
	id := strings.TrimSpace(out)

	dockerCmd(c, "volume", "create", "test")
	dockerCmd(c, "volume", "rm", id)
	dockerCmd(c, "volume", "rm", "test")

	out, _ = dockerCmd(c, "volume", "ls")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 1, check.Commentf("%s\n", out))

	volumeID := "testing"
	dockerCmd(c, "run", "-v", volumeID+":"+prefix+"/foo", "--name=test", "busybox", "sh", "-c", "echo hello > /foo/bar")
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "volume", "rm", "testing"))
	c.Assert(
		err,
		check.Not(check.IsNil),
		check.Commentf("Should not be able to remove volume that is in use by a container\n%s", out))

	out, _ = dockerCmd(c, "run", "--volumes-from=test", "--name=test2", "busybox", "sh", "-c", "cat /foo/bar")
	c.Assert(strings.TrimSpace(out), check.Equals, "hello")
	dockerCmd(c, "rm", "-fv", "test2")
	dockerCmd(c, "volume", "inspect", volumeID)
	dockerCmd(c, "rm", "-f", "test")

	out, _ = dockerCmd(c, "run", "--name=test2", "-v", volumeID+":"+prefix+"/foo", "busybox", "sh", "-c", "cat /foo/bar")
	c.Assert(strings.TrimSpace(out), check.Equals, "hello", check.Commentf("volume data was removed"))
	dockerCmd(c, "rm", "test2")

	dockerCmd(c, "volume", "rm", volumeID)
	c.Assert(
		exec.Command("volume", "rm", "doesntexist").Run(),
		check.Not(check.IsNil),
		check.Commentf("volume rm should fail with non-existent volume"),
	)
}

// FIXME(vdemeester) should be a unit test in cli/command/volume package
func (s *DockerSuite) TestVolumeCLINoArgs(c *check.C) {
	out, _ := dockerCmd(c, "volume")
	// no args should produce the cmd usage output
	usage := "Usage:	docker volume COMMAND"
	c.Assert(out, checker.Contains, usage)

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
	c.Assert(result.Stderr(), checker.Contains, "unknown flag: --no-such-flag")
}

func (s *DockerSuite) TestVolumeCLIInspectTmplError(c *check.C) {
	out, _ := dockerCmd(c, "volume", "create")
	name := strings.TrimSpace(out)

	out, exitCode, err := dockerCmdWithError("volume", "inspect", "--format='{{ .FooBar }}'", name)
	c.Assert(err, checker.NotNil, check.Commentf("Output: %s", out))
	c.Assert(exitCode, checker.Equals, 1, check.Commentf("Output: %s", out))
	c.Assert(out, checker.Contains, "Template parsing error")
}

func (s *DockerSuite) TestVolumeCLICreateWithOpts(c *check.C) {
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
			c.Assert(info[0], checker.Equals, "tmpfs")
			c.Assert(info[2], checker.Equals, "/foo")
			c.Assert(info[4], checker.Equals, "tmpfs")
			c.Assert(info[5], checker.Contains, "uid=1000")
			c.Assert(info[5], checker.Contains, "size=1024k")
			break
		}
	}
	c.Assert(found, checker.Equals, true)
}

func (s *DockerSuite) TestVolumeCLICreateLabel(c *check.C) {
	testVol := "testvolcreatelabel"
	testLabel := "foo"
	testValue := "bar"

	out, _, err := dockerCmdWithError("volume", "create", "--label", testLabel+"="+testValue, testVol)
	c.Assert(err, check.IsNil)

	out, _ = dockerCmd(c, "volume", "inspect", "--format={{ .Labels."+testLabel+" }}", testVol)
	c.Assert(strings.TrimSpace(out), check.Equals, testValue)
}

func (s *DockerSuite) TestVolumeCLICreateLabelMultiple(c *check.C) {
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

	out, _, err := dockerCmdWithError(args...)
	c.Assert(err, check.IsNil)

	for k, v := range testLabels {
		out, _ = dockerCmd(c, "volume", "inspect", "--format={{ .Labels."+k+" }}", testVol)
		c.Assert(strings.TrimSpace(out), check.Equals, v)
	}
}

func (s *DockerSuite) TestVolumeCLILsFilterLabels(c *check.C) {
	testVol1 := "testvolcreatelabel-1"
	out, _, err := dockerCmdWithError("volume", "create", "--label", "foo=bar1", testVol1)
	c.Assert(err, check.IsNil)

	testVol2 := "testvolcreatelabel-2"
	out, _, err = dockerCmdWithError("volume", "create", "--label", "foo=bar2", testVol2)
	c.Assert(err, check.IsNil)

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "label=foo")

	// filter with label=key
	c.Assert(out, checker.Contains, "testvolcreatelabel-1\n", check.Commentf("expected volume 'testvolcreatelabel-1' in output"))
	c.Assert(out, checker.Contains, "testvolcreatelabel-2\n", check.Commentf("expected volume 'testvolcreatelabel-2' in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "label=foo=bar1")

	// filter with label=key=value
	c.Assert(out, checker.Contains, "testvolcreatelabel-1\n", check.Commentf("expected volume 'testvolcreatelabel-1' in output"))
	c.Assert(out, check.Not(checker.Contains), "testvolcreatelabel-2\n", check.Commentf("expected volume 'testvolcreatelabel-2 in output"))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "label=non-exist")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 1, check.Commentf("\n%s", out))

	out, _ = dockerCmd(c, "volume", "ls", "--filter", "label=foo=non-exist")
	outArr = strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 1, check.Commentf("\n%s", out))
}

func (s *DockerSuite) TestVolumeCLIRmForceUsage(c *check.C) {
	out, _ := dockerCmd(c, "volume", "create")
	id := strings.TrimSpace(out)

	dockerCmd(c, "volume", "rm", "-f", id)
	dockerCmd(c, "volume", "rm", "--force", "nonexist")

	out, _ = dockerCmd(c, "volume", "ls")
	outArr := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(outArr), check.Equals, 1, check.Commentf("%s\n", out))
}

func (s *DockerSuite) TestVolumeCLIRmForce(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	name := "test"
	out, _ := dockerCmd(c, "volume", "create", name)
	id := strings.TrimSpace(out)
	c.Assert(id, checker.Equals, name)

	out, _ = dockerCmd(c, "volume", "inspect", "--format", "{{.Mountpoint}}", name)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")
	// Mountpoint is in the form of "/var/lib/docker/volumes/.../_data", removing `/_data`
	path := strings.TrimSuffix(strings.TrimSpace(out), "/_data")
	out, _, err := runCommandWithOutput(exec.Command("rm", "-rf", path))
	c.Assert(err, check.IsNil)

	dockerCmd(c, "volume", "rm", "-f", "test")
	out, _ = dockerCmd(c, "volume", "ls")
	c.Assert(out, checker.Not(checker.Contains), name)
	dockerCmd(c, "volume", "create", "test")
	out, _ = dockerCmd(c, "volume", "ls")
	c.Assert(out, checker.Contains, name)
}

func (s *DockerSuite) TestVolumeCliInspectWithVolumeOpts(c *check.C) {
	testRequires(c, DaemonIsLinux)

	// Without options
	name := "test1"
	dockerCmd(c, "volume", "create", "-d", "local", name)
	out, _ := dockerCmd(c, "volume", "inspect", "--format={{ .Options }}", name)
	c.Assert(strings.TrimSpace(out), checker.Contains, "map[]")

	// With options
	name = "test2"
	k1, v1 := "type", "tmpfs"
	k2, v2 := "device", "tmpfs"
	k3, v3 := "o", "size=1m,uid=1000"
	dockerCmd(c, "volume", "create", "-d", "local", name, "--opt", fmt.Sprintf("%s=%s", k1, v1), "--opt", fmt.Sprintf("%s=%s", k2, v2), "--opt", fmt.Sprintf("%s=%s", k3, v3))
	out, _ = dockerCmd(c, "volume", "inspect", "--format={{ .Options }}", name)
	c.Assert(strings.TrimSpace(out), checker.Contains, fmt.Sprintf("%s:%s", k1, v1))
	c.Assert(strings.TrimSpace(out), checker.Contains, fmt.Sprintf("%s:%s", k2, v2))
	c.Assert(strings.TrimSpace(out), checker.Contains, fmt.Sprintf("%s:%s", k3, v3))
}
