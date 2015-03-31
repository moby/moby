package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"unicode"

	"github.com/docker/docker/pkg/homedir"
)

func TestHelpTextVerify(t *testing.T) {
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
		out, ec, err := runCommandWithOutput(helpCmd)
		if err != nil || ec != 0 {
			t.Fatalf("docker help should have worked\nout:%s\nec:%d", out, ec)
		}
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			if len(line) > 80 {
				t.Fatalf("Line is too long(%d chars):\n%s", len(line), line)
			}

			// All lines should not end with a space
			if strings.HasSuffix(line, " ") {
				t.Fatalf("Line should not end with a space: %s", line)
			}

			if scanForHome && strings.Contains(line, `=`+home) {
				t.Fatalf("Line should use '%q' instead of %q:\n%s", homedir.GetShortcutString(), home, line)
			}
			if runtime.GOOS != "windows" {
				i := strings.Index(line, homedir.GetShortcutString())
				if i >= 0 && i != len(line)-1 && line[i+1] != '/' {
					t.Fatalf("Main help should not have used home shortcut:\n%s", line)
				}
			}
		}

		// Make sure each cmd's help text fits within 80 chars and that
		// on non-windows system we use ~ when possible (to shorten things).
		// Pull the list of commands from the "Commands:" section of docker help
		helpCmd = exec.Command(dockerBinary, "help")
		helpCmd.Env = newEnvs
		out, ec, err = runCommandWithOutput(helpCmd)
		if err != nil || ec != 0 {
			t.Fatalf("docker help should have worked\nout:%s\nec:%d", out, ec)
		}
		i := strings.Index(out, "Commands:")
		if i < 0 {
			t.Fatalf("Missing 'Commands:' in:\n%s", out)
		}

		// Grab all chars starting at "Commands:"
		// Skip first line, its "Commands:"
		cmds := []string{}
		for _, cmd := range strings.Split(out[i:], "\n")[1:] {
			// Stop on blank line or non-idented line
			if cmd == "" || !unicode.IsSpace(rune(cmd[0])) {
				break
			}

			// Grab just the first word of each line
			cmd = strings.Split(strings.TrimSpace(cmd), " ")[0]
			cmds = append(cmds, cmd)

			helpCmd := exec.Command(dockerBinary, cmd, "--help")
			helpCmd.Env = newEnvs
			out, ec, err := runCommandWithOutput(helpCmd)
			if err != nil || ec != 0 {
				t.Fatalf("Error on %q help: %s\nexit code:%d", cmd, out, ec)
			}
			lines := strings.Split(out, "\n")
			for _, line := range lines {
				if len(line) > 80 {
					t.Fatalf("Help for %q is too long(%d chars):\n%s", cmd,
						len(line), line)
				}

				if scanForHome && strings.Contains(line, `"`+home) {
					t.Fatalf("Help for %q should use ~ instead of %q on:\n%s",
						cmd, home, line)
				}
				i := strings.Index(line, "~")
				if i >= 0 && i != len(line)-1 && line[i+1] != '/' {
					t.Fatalf("Help for %q should not have used ~:\n%s", cmd, line)
				}

				// If a line starts with 4 spaces then assume someone
				// added a multi-line description for an option and we need
				// to flag it
				if strings.HasPrefix(line, "    ") {
					t.Fatalf("Help for %q should not have a multi-line option: %s", cmd, line)
				}

				// Options should NOT end with a period
				if strings.HasPrefix(line, "  -") && strings.HasSuffix(line, ".") {
					t.Fatalf("Help for %q should not end with a period: %s", cmd, line)
				}

				// Options should NOT end with a space
				if strings.HasSuffix(line, " ") {
					t.Fatalf("Help for %q should not end with a space: %s", cmd, line)
				}

			}
		}

		expected := 40
		if len(cmds) != expected {
			t.Fatalf("Wrong # of cmds(%d), it should be: %d\nThe list:\n%q",
				len(cmds), expected, cmds)
		}
	}

	logDone("help - verify text")
}
