package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode"

	"github.com/docker/docker/pkg/homedir"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestHelpTextVerify(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Make sure main help text fits within 80 chars and that
	// on non-windows system we use ~ when possible (to shorten things).
	// Test for HOME set to its default value and set to "/" on linux
	// Yes on windows setting up an array and looping (right now) isn't
	// necessary because we just have one value, but we'll need the
	// array/loop on linux so we might as well set it up so that we can
	// test any number of home dirs later on and all we need to do is
	// modify the array - the rest of the testing infrastructure should work
	homes := []string{homedir.Get()}

	// Non-Windows machines need to test for this special case of $HOME
	if runtime.GOOS != "windows" {
		homes = append(homes, "/")
	}

	homeKey := homedir.Key()
	baseEnvs := os.Environ()

	// Remove HOME env var from list so we can add a new value later.
	for i, env := range baseEnvs {
		if strings.HasPrefix(env, homeKey+"=") {
			baseEnvs = append(baseEnvs[:i], baseEnvs[i+1:]...)
			break
		}
	}

	for _, home := range homes {
		// Dup baseEnvs and add our new HOME value
		newEnvs := make([]string, len(baseEnvs)+1)
		copy(newEnvs, baseEnvs)
		newEnvs[len(newEnvs)-1] = homeKey + "=" + home

		scanForHome := runtime.GOOS != "windows" && home != "/"

		// Check main help text to make sure its not over 80 chars
		helpCmd := exec.Command(dockerBinary, "help")
		helpCmd.Env = newEnvs
		out, _, err := runCommandWithOutput(helpCmd)
		c.Assert(err, checker.IsNil, check.Commentf(out))
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			c.Assert(len(line), checker.LessOrEqualThan, 80, check.Commentf("Line is too long:\n%s", line))

			// All lines should not end with a space
			c.Assert(line, checker.Not(checker.HasSuffix), " ", check.Commentf("Line should not end with a space"))

			if scanForHome && strings.Contains(line, `=`+home) {
				c.Fatalf("Line should use '%q' instead of %q:\n%s", homedir.GetShortcutString(), home, line)
			}
			if runtime.GOOS != "windows" {
				i := strings.Index(line, homedir.GetShortcutString())
				if i >= 0 && i != len(line)-1 && line[i+1] != '/' {
					c.Fatalf("Main help should not have used home shortcut:\n%s", line)
				}
			}
		}

		// Make sure each cmd's help text fits within 90 chars and that
		// on non-windows system we use ~ when possible (to shorten things).
		// Pull the list of commands from the "Commands:" section of docker help
		helpCmd = exec.Command(dockerBinary, "help")
		helpCmd.Env = newEnvs
		out, _, err = runCommandWithOutput(helpCmd)
		c.Assert(err, checker.IsNil, check.Commentf(out))
		i := strings.Index(out, "Commands:")
		c.Assert(i, checker.GreaterOrEqualThan, 0, check.Commentf("Missing 'Commands:' in:\n%s", out))

		cmds := []string{}
		// Grab all chars starting at "Commands:"
		helpOut := strings.Split(out[i:], "\n")
		// First line is just "Commands:"
		if isLocalDaemon {
			// Replace first line with "daemon" command since it's not part of the list of commands.
			helpOut[0] = " daemon"
		} else {
			// Skip first line
			helpOut = helpOut[1:]
		}

		// Create the list of commands we want to test
		cmdsToTest := []string{}
		for _, cmd := range helpOut {
			// Stop on blank line or non-idented line
			if cmd == "" || !unicode.IsSpace(rune(cmd[0])) {
				break
			}

			// Grab just the first word of each line
			cmd = strings.Split(strings.TrimSpace(cmd), " ")[0]
			cmds = append(cmds, cmd) // Saving count for later

			cmdsToTest = append(cmdsToTest, cmd)
		}

		// Add some 'two word' commands - would be nice to automatically
		// calculate this list - somehow
		cmdsToTest = append(cmdsToTest, "volume create")
		cmdsToTest = append(cmdsToTest, "volume inspect")
		cmdsToTest = append(cmdsToTest, "volume ls")
		cmdsToTest = append(cmdsToTest, "volume rm")

		for _, cmd := range cmdsToTest {
			var stderr string

			args := strings.Split(cmd+" --help", " ")

			// Check the full usage text
			helpCmd := exec.Command(dockerBinary, args...)
			helpCmd.Env = newEnvs
			out, stderr, _, err = runCommandWithStdoutStderr(helpCmd)
			c.Assert(len(stderr), checker.Equals, 0, check.Commentf("Error on %q help. non-empty stderr:%q", cmd, stderr))
			c.Assert(out, checker.Not(checker.HasSuffix), "\n\n", check.Commentf("Should not have blank line on %q\n", cmd))
			c.Assert(out, checker.Contains, "--help", check.Commentf("All commands should mention '--help'. Command '%v' did not.\n", cmd))

			c.Assert(err, checker.IsNil, check.Commentf(out))

			// Check each line for lots of stuff
			lines := strings.Split(out, "\n")
			for _, line := range lines {
				c.Assert(len(line), checker.LessOrEqualThan, 90, check.Commentf("Help for %q is too long:\n%s", cmd, line))

				if scanForHome && strings.Contains(line, `"`+home) {
					c.Fatalf("Help for %q should use ~ instead of %q on:\n%s",
						cmd, home, line)
				}
				i := strings.Index(line, "~")
				if i >= 0 && i != len(line)-1 && line[i+1] != '/' {
					c.Fatalf("Help for %q should not have used ~:\n%s", cmd, line)
				}

				// If a line starts with 4 spaces then assume someone
				// added a multi-line description for an option and we need
				// to flag it
				c.Assert(line, checker.Not(checker.HasPrefix), "    ", check.Commentf("Help for %q should not have a multi-line option", cmd))

				// Options should NOT end with a period
				if strings.HasPrefix(line, "  -") && strings.HasSuffix(line, ".") {
					c.Fatalf("Help for %q should not end with a period: %s", cmd, line)
				}

				// Options should NOT end with a space
				c.Assert(line, checker.Not(checker.HasSuffix), " ", check.Commentf("Help for %q should not end with a space", cmd))

			}

			// For each command make sure we generate an error
			// if we give a bad arg
			args = strings.Split(cmd+" --badArg", " ")

			out, _, err = dockerCmdWithError(args...)
			c.Assert(err, checker.NotNil, check.Commentf(out))
			// Be really picky
			c.Assert(stderr, checker.Not(checker.HasSuffix), "\n\n", check.Commentf("Should not have a blank line at the end of 'docker rm'\n"))

			// Now make sure that each command will print a short-usage
			// (not a full usage - meaning no opts section) if we
			// are missing a required arg or pass in a bad arg

			// These commands will never print a short-usage so don't test
			noShortUsage := map[string]string{
				"images":  "",
				"login":   "",
				"logout":  "",
				"network": "",
				"stats":   "",
			}

			if _, ok := noShortUsage[cmd]; !ok {
				// For each command run it w/o any args. It will either return
				// valid output or print a short-usage
				var dCmd *exec.Cmd
				var stdout, stderr string
				var args []string

				// skipNoArgs are ones that we don't want to try w/o
				// any args. Either because it'll hang the test or
				// lead to incorrect test result (like false negative).
				// Whatever the reason, skip trying to run w/o args and
				// jump to trying with a bogus arg.
				skipNoArgs := map[string]struct{}{
					"daemon": {},
					"events": {},
					"load":   {},
				}

				ec := 0
				if _, ok := skipNoArgs[cmd]; !ok {
					args = strings.Split(cmd, " ")
					dCmd = exec.Command(dockerBinary, args...)
					stdout, stderr, ec, err = runCommandWithStdoutStderr(dCmd)
				}

				// If its ok w/o any args then try again with an arg
				if ec == 0 {
					args = strings.Split(cmd+" badArg", " ")
					dCmd = exec.Command(dockerBinary, args...)
					stdout, stderr, ec, err = runCommandWithStdoutStderr(dCmd)
				}

				if len(stdout) != 0 || len(stderr) == 0 || ec == 0 || err == nil {
					c.Fatalf("Bad output from %q\nstdout:%q\nstderr:%q\nec:%d\nerr:%q", args, stdout, stderr, ec, err)
				}
				// Should have just short usage
				c.Assert(stderr, checker.Contains, "\nUsage:\t", check.Commentf("Missing short usage on %q\n", args))
				// But shouldn't have full usage
				c.Assert(stderr, checker.Not(checker.Contains), "--help=false", check.Commentf("Should not have full usage on %q\n", args))
				c.Assert(stderr, checker.Not(checker.HasSuffix), "\n\n", check.Commentf("Should not have a blank line on %q\n", args))
			}

		}

		// Number of commands for standard release and experimental release
		standard := 40
		experimental := 1
		expected := standard + experimental
		if isLocalDaemon {
			expected++ // for the daemon command
		}
		c.Assert(len(cmds), checker.LessOrEqualThan, expected, check.Commentf("Wrong # of cmds, it should be: %d\nThe list:\n%q", expected, cmds))
	}

}

func (s *DockerSuite) TestHelpExitCodesHelpOutput(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Test to make sure the exit code and output (stdout vs stderr) of
	// various good and bad cases are what we expect

	// docker : stdout=all, stderr=empty, rc=0
	out, _, err := dockerCmdWithError()
	c.Assert(err, checker.IsNil, check.Commentf(out))
	// Be really pick
	c.Assert(out, checker.Not(checker.HasSuffix), "\n\n", check.Commentf("Should not have a blank line at the end of 'docker'\n"))

	// docker help: stdout=all, stderr=empty, rc=0
	out, _, err = dockerCmdWithError("help")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	// Be really pick
	c.Assert(out, checker.Not(checker.HasSuffix), "\n\n", check.Commentf("Should not have a blank line at the end of 'docker help'\n"))

	// docker --help: stdout=all, stderr=empty, rc=0
	out, _, err = dockerCmdWithError("--help")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	// Be really pick
	c.Assert(out, checker.Not(checker.HasSuffix), "\n\n", check.Commentf("Should not have a blank line at the end of 'docker --help'\n"))

	// docker inspect busybox: stdout=all, stderr=empty, rc=0
	// Just making sure stderr is empty on valid cmd
	out, _, err = dockerCmdWithError("inspect", "busybox")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	// Be really pick
	c.Assert(out, checker.Not(checker.HasSuffix), "\n\n", check.Commentf("Should not have a blank line at the end of 'docker inspect busyBox'\n"))

	// docker rm: stdout=empty, stderr=all, rc!=0
	// testing the min arg error msg
	cmd := exec.Command(dockerBinary, "rm")
	stdout, stderr, _, err := runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.NotNil)
	c.Assert(stdout, checker.Equals, "")
	// Should not contain full help text but should contain info about
	// # of args and Usage line
	c.Assert(stderr, checker.Contains, "requires a minimum", check.Commentf("Missing # of args text from 'docker rm'\n"))

	// docker rm NoSuchContainer: stdout=empty, stderr=all, rc=0
	// testing to make sure no blank line on error
	cmd = exec.Command(dockerBinary, "rm", "NoSuchContainer")
	stdout, stderr, _, err = runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.NotNil)
	c.Assert(len(stderr), checker.Not(checker.Equals), 0)
	c.Assert(stdout, checker.Equals, "")
	// Be really picky
	c.Assert(stderr, checker.Not(checker.HasSuffix), "\n\n", check.Commentf("Should not have a blank line at the end of 'docker rm'\n"))

	// docker BadCmd: stdout=empty, stderr=all, rc=0
	cmd = exec.Command(dockerBinary, "BadCmd")
	stdout, stderr, _, err = runCommandWithStdoutStderr(cmd)
	c.Assert(err, checker.NotNil)
	c.Assert(stdout, checker.Equals, "")
	c.Assert(stderr, checker.Equals, "docker: 'BadCmd' is not a docker command.\nSee 'docker --help'.\n", check.Commentf("Unexcepted output for 'docker badCmd'\n"))
}
