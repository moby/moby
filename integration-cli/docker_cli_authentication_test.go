package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-check/check"
)

type Krb5Env struct {
	envPath       string
	kdcEnvPath    string
	kdcPidPath    string
	oldConfig     string
	oldKdcProfile string
}

func NewKrb5Env() *Krb5Env {
	krb5 := Krb5Env{
		envPath:    "/tmp/krb5/",
		kdcEnvPath: "krb5kdc/",
		kdcPidPath: "krb5kdc.pid",
	}

	krb5.kdcEnvPath = krb5.envPath + krb5.kdcEnvPath
	krb5.kdcPidPath = krb5.kdcEnvPath + krb5.kdcPidPath

	return &krb5
}

func (krb5 *Krb5Env) Start(c *check.C) {
	krb5.oldConfig = os.Getenv("KRB5_CONFIG")
	krb5.oldKdcProfile = os.Getenv("KRB5_KDC_PROFILE")
	os.Setenv("KRB5_CONFIG", "fixtures/krb5/krb5.conf")
	os.Setenv("KRB5_KDC_PROFILE", "fixtures/krb5/krb5kdc/kdc.conf")

	if err := os.MkdirAll(krb5.kdcEnvPath, 0666); err != nil {
		c.Fatalf("Failed to mkdir for kerberos environment, err %v", err)
	}

	// Create a kerberos database and a stash file.
	if output, _, err := runCommandWithOutput(exec.Command("kdb5_util", "create", "-s", "-P", "admin")); err != nil {
		c.Fatalf("Failed to create kerberos db, err %v with output %s", err, string(output))
	}

	// Run the KDC. Note that this command is non-blocking.
	if output, _, err := runCommandWithOutput(exec.Command("krb5kdc", "-P", krb5.kdcPidPath)); err != nil {
		c.Fatalf("Failed to start kdc, err %v with output %s", err, string(output))
	}

	// Add docker client principal.
	if output, _, err := runCommandWithOutput(exec.Command("kadmin.local", "-q", "addprinc -pw docker docker")); err != nil {
		c.Fatalf("Failed to add kerberos principal, err %v with output %s", err, output)
	}

	// Add docker client principal to keytab (Unfortunately, kinit does not have a command line password parameter).
	if output, _, err := runCommandWithOutput(exec.Command("kadmin.local", "-q", "ktadd docker")); err != nil {
		c.Fatalf("Failed to add kerberos principal to keytab, err %v with output %s", err, output)
	}

	// Add docker server principal.
	if output, _, err := runCommandWithOutput(exec.Command("kadmin.local", "-q", "addprinc -randkey HTTP/localhost")); err != nil {
		c.Fatalf("Failed to add kerberos principal, err %v with output %s", err, output)
	}

	// Add docker server principal to keytab.
	if output, _, err := runCommandWithOutput(exec.Command("kadmin.local", "-q", "ktadd HTTP/localhost")); err != nil {
		c.Fatalf("Failed to add kerberos principal to keytab, err %v with output %s", err, output)
	}
}

func (krb5 *Krb5Env) Stop(c *check.C) {
	// Remove docker server principal from keytab.
	if output, _, err := runCommandWithOutput(exec.Command("kadmin.local", "-q", "ktrem HTTP/localhost")); err != nil {
		c.Logf("Failed to remove kerberos principal from keytab, err %v with output %s", err, output)
	}

	// Remove docker client principal from keytab.
	if output, _, err := runCommandWithOutput(exec.Command("kadmin.local", "-q", "ktrem docker")); err != nil {
		c.Logf("Failed to remove kerberos principal from keytab, err %v with output %s", err, output)
	}

	// Kill the KDC.
	if pidBytes, err := ioutil.ReadFile(krb5.kdcPidPath); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes))); err == nil {
			if process, err := os.FindProcess(pid); err == nil {
				process.Kill()
			}
		}
	}

	if output, _, err := runCommandWithOutput(exec.Command("kdb5_util", "destroy", "-f")); err != nil {
		c.Logf("Failed to destroy kerberos db, err %v with output %s", err, output)
	}

	os.RemoveAll(krb5.kdcEnvPath)
	os.RemoveAll(krb5.envPath)

	os.Unsetenv("KRB5_CONFIG")
	if krb5.oldConfig != "" {
		os.Setenv("KRB5_CONFIG", krb5.oldConfig)
	}

	os.Unsetenv("KRB5_KDC_PROFILE")
	if krb5.oldConfig != "" {
		os.Setenv("KRB5_KDC_PROFILE", krb5.oldKdcProfile)
	}
}

func (krb5 *Krb5Env) Kinit(c *check.C) {
	if output, _, err := runCommandWithOutput(exec.Command("kinit", "docker", "-k")); err != nil {
		c.Fatalf("Failed to add obtain kerberos ticket, err %v with output %s", err, output)
	}
}

func (krb5 *Krb5Env) Kdestroy(c *check.C) {
	if output, _, err := runCommandWithOutput(exec.Command("kdestroy")); err != nil {
		c.Logf("Failed to destroy kerberos cache, err %v with output %s", err, output)
	}
}

func (s *DockerAuthnSuite) TestKerberosAuthnRest(c *check.C) {
	s.krb5.Start(c)
	s.krb5.Kinit(c)
	defer s.krb5.Kdestroy(c)

	if err := s.ds.d.Start("-H", s.daemonAddr, "-a"); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}

	//force tcp protocol
	host := fmt.Sprintf("tcp://%s", s.daemonAddr)
	daemonArgs := []string{"--host", host}
	out, err := s.ds.d.CmdWithArgs(daemonArgs, "-D", "info")
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s", err, out)
	}
}

func (s *DockerAuthnSuite) TestKerberosAuthnRun(c *check.C) {
	s.krb5.Start(c)
	s.krb5.Kinit(c)
	defer s.krb5.Kdestroy(c)

	if err := s.ds.d.Start("-H", s.daemonAddr, "-a"); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}

	//force tcp protocol
	host := fmt.Sprintf("tcp://%s", s.daemonAddr)
	daemonArgs := []string{"--host", host}
	stdin := "echo interactive docker output"
	out, err := s.ds.d.CmdWithArgs(daemonArgs, "run", "busybox", "/bin/sh", "-c", stdin)
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s", err, out)
	}
	c.Assert(strings.Contains(out, "interactive docker output"), check.Equals, true, check.Commentf("actual output is: %s", out))
}

func (s *DockerAuthnSuite) TestKerberosAuthnNoConfig(c *check.C) {
	if err := s.ds.d.Start("-H", s.daemonAddr, "-a"); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}

	//force tcp protocol
	host := fmt.Sprintf("tcp://%s", s.daemonAddr)
	daemonArgs := []string{"--host", host}
	out, err := s.ds.d.CmdWithArgs(daemonArgs, "info")
	c.Assert(err, check.ErrorMatches, "exit status 1")
	c.Assert(strings.Contains(out, "Unable to authenticate to docker daemon"), check.Equals, true, check.Commentf("actual output is: %s", out))
}

func (s *DockerAuthnSuite) TestKerberosAuthnNoTicket(c *check.C) {
	s.krb5.Start(c)

	if err := s.ds.d.Start("-H", s.daemonAddr, "-a"); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}

	//force tcp protocol
	host := fmt.Sprintf("tcp://%s", s.daemonAddr)
	daemonArgs := []string{"--host", host}
	out, err := s.ds.d.CmdWithArgs(daemonArgs, "info")
	c.Assert(err, check.ErrorMatches, "exit status 1")
	c.Assert(strings.Contains(out, "Unable to authenticate to docker daemon"), check.Equals, true, check.Commentf("actual output is: %s", out))
}
