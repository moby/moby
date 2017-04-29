// +build !windows

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	"github.com/docker/docker/integration-cli/cli"
	icmd "github.com/docker/docker/pkg/testutil/cmd"
	"github.com/go-check/check"
)

func (s *DockerDaemonSuite) TestSSHClientWithSSHAgent(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)
	sshAuthSock, dockerHost, cleanup := setupSSHTest(s, c, 60022)
	defer cleanup()
	type testCase struct {
		Args     []string
		Env      []string
		Expected icmd.Expected
	}
	cases := []testCase{
		{
			Args:     []string{"-H", dockerHost},
			Expected: icmd.Expected{Err: "ssh: handshake failed", ExitCode: 1},
		},
		{
			Args:     []string{"-H", dockerHost},
			Env:      []string{"SSH_AUTH_SOCK=" + sshAuthSock},
			Expected: icmd.Expected{Out: "hello ssh", ExitCode: 0},
		},
	}
	for _, x := range cases {
		result := cli.Docker(
			cli.Args(
				append(x.Args, []string{"run", "--rm", "busybox",
					"echo", "hello ssh"}...)...),
			// FIXME: go-connections requires HOME to be set even when unneeded
			cli.WithEnvironmentVariables(
				append(os.Environ(), x.Env...)...))
		c.Assert(result, icmd.Matches, x.Expected)
	}
}

// setupSSHTest returns sshAuthSock, dockerHost, and cleanup
func setupSSHTest(s *DockerDaemonSuite, c *check.C, sshPort int) (string, string, func()) {
	tempDir, err := ioutil.TempDir("", "test-ssh-client")
	if err != nil {
		c.Fatal(err)
	}
	populateSSHFixturesDir(c, tempDir)
	dockerSock := filepath.Join(tempDir, "docker.sock")
	sshAuthSock := filepath.Join(tempDir, "ssh-agent.sock")
	idRSA := filepath.Join(tempDir, "home", "dummy", "id_rsa")
	dockerHost := fmt.Sprintf("ssh://localhost:%d%s", sshPort, dockerSock)

	s.d.StartWithBusybox(c, "-H", "unix://"+dockerSock)
	shutdownSSHD := startSSHD(c, tempDir, sshPort)
	shutdownSSHAgent := startSSHAgent(c, tempDir, sshAuthSock)
	addKeyToSSHAgent(c, idRSA, sshAuthSock)
	cleanup := func() {
		shutdownSSHAgent()
		shutdownSSHD()
		s.d.Stop(c)
		os.RemoveAll(tempDir)
	}
	return sshAuthSock, dockerHost, cleanup
}

// populateSSHFixturesDir populates files under fixture/ssh to tempdir
func populateSSHFixturesDir(c *check.C, tempDir string) {
	// we don't use map here, because map is not ordered
	dirs := []string{"etc", "etc/ssh", "home", "home/dummy"}
	dirsPerm := os.ModeDir | 0700
	files := []string{
		"etc/ssh/ssh_host_rsa_key",
		"home/dummy/id_rsa",
		"home/dummy/authorized_keys",
	}
	filesPerm := os.FileMode(0400)
	wd, err := os.Getwd()
	if err != nil {
		c.Fatal(err)
	}
	srcDir := filepath.Join(wd, "fixtures", "ssh")
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tempDir, dir), dirsPerm); err != nil {
			c.Fatal(err)
		}
	}
	for _, file := range files {
		b, err := ioutil.ReadFile(filepath.Join(srcDir, file))
		if err != nil {
			c.Fatal(err)
		}
		if err = ioutil.WriteFile(filepath.Join(tempDir, file), b, filesPerm); err != nil {
			c.Fatal(err)
		}
	}
}

var sshdConfigTemplate = template.Must(template.New("").Parse(`Protocol 2
UsePrivilegeSeparation no
HostKey {{.TempDir}}/etc/ssh/ssh_host_rsa_key
AuthorizedKeysFile {{.TempDir}}/home/dummy/authorized_keys
StrictModes no
AllowStreamLocalForwarding yes
`))

func absoluteSSHDPath(c *check.C) string {
	sshd, err := exec.LookPath("sshd")
	if err != nil {
		c.Skip("sshd not installed")
	}
	return sshd
}

func startSSHD(c *check.C, tempDir string, port int) func() {
	sshdConfig := filepath.Join(tempDir, "sshd_config")
	sshdConfigWriter, err := os.Create(sshdConfig)
	if err != nil {
		c.Fatal(err)
	}
	if err = sshdConfigTemplate.Execute(sshdConfigWriter,
		map[string]string{"TempDir": tempDir}); err != nil {
		c.Fatal(err)
	}

	// sshd requires argv0 to be absolute path
	cmd := exec.Command(absoluteSSHDPath(c), "-f", sshdConfig, "-p", strconv.Itoa(port), "-D")
	cmd.Stdout, _ = os.Create(filepath.Join(tempDir, "sshd.log"))
	cmd.Stderr = cmd.Stdout
	cmd.Env = []string{filepath.Join(tempDir, "home", "dummy")}
	if err = cmd.Start(); err != nil {
		c.Fatal(err)
	}
	cleanup := func() {
		// FIXME: can we use icmd pkg for Start/Kill?
		if err = cmd.Process.Kill(); err != nil {
			c.Fatal(err)
		}
	}
	return cleanup
}

func startSSHAgent(c *check.C, tempDir, sshAuthSock string) func() {
	// -D (foreground) is not supported in older ssh-agent;
	// so we use -d (foreground + debug)
	cmd := exec.Command("ssh-agent", "-a", sshAuthSock, "-d")
	cmd.Stdout, _ = os.Create(filepath.Join(tempDir, "ssh-agent.log"))
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		c.Fatal(err)
	}
	cleanup := func() {
		// FIXME: can we use icmd pkg for Start/Kill?
		if err := cmd.Process.Kill(); err != nil {
			c.Fatal(err)
		}
	}
	time.Sleep(3 * time.Second) // FIXME
	return cleanup
}

func addKeyToSSHAgent(c *check.C, idRSA, sshAuthSock string) {
	cmd := icmd.Cmd{
		Command: []string{"ssh-add", idRSA},
		Env:     []string{"SSH_AUTH_SOCK=" + sshAuthSock},
	}
	if res := icmd.RunCmd(cmd); res.Error != nil {
		c.Fatal(res.String())
	}
}
