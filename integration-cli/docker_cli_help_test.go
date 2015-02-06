package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

func TestMainHelpWidth(t *testing.T) {
	// Make sure main help text fits within 80 chars and that
	// on non-windows system we use ~ when possible (to shorten things)

	var home string
	if runtime.GOOS != "windows" {
		home = os.Getenv("HOME")
	}

	helpCmd := exec.Command(dockerBinary, "help")
	out, ec, err := runCommandWithOutput(helpCmd)
	if err != nil || ec != 0 {
		t.Fatalf("docker help should have worked\nout:%s\nec:%d", out, ec)
	}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if len(line) > 80 {
			t.Fatalf("Line is too long(%d chars):\n%s", len(line), line)
		}
		if home != "" && strings.Contains(line, home) {
			t.Fatalf("Line should use ~ instead of %q:\n%s", home, line)
		}
	}
	logDone("help - verify main width")
}

func TestCmdHelpWidth(t *testing.T) {
	// Make sure main help text fits within 80 chars and that
	// on non-windows system we use ~ when possible (to shorten things)

	var home string
	if runtime.GOOS != "windows" {
		home = os.Getenv("HOME")
	}

	// Pull the list of commands from the "Commands:" section of docker help
	helpCmd := exec.Command(dockerBinary, "help")
	out, ec, err := runCommandWithOutput(helpCmd)
	if err != nil || ec != 0 {
		t.Fatalf("docker help should have worked\nout:%s\nec:%d", out, ec)
	}
	i := strings.Index(out, "Commands:")
	if i < 0 {
		t.Fatalf("Missing 'Commands:' in:\n%s", out)
	}

	// Grab all chars starting at "Commands:"
	// Skip first line, its "Commands:"
	count := 0
	cmds := ""
	for _, command := range strings.Split(out[i:], "\n")[1:] {
		// Stop on blank line or non-idented line
		if command == "" || !unicode.IsSpace(rune(command[0])) {
			break
		}

		// Grab just the first word of each line
		command = strings.Split(strings.TrimSpace(command), " ")[0]

		count++
		cmds = cmds + "\n" + command

		helpCmd := exec.Command(dockerBinary, command, "--help")
		out, ec, err := runCommandWithOutput(helpCmd)
		if err != nil || ec != 0 {
			t.Fatalf("docker help should have worked\nout:%s\nec:%d", out, ec)
		}
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			if len(line) > 80 {
				t.Fatalf("Help for %q is too long(%d chars):\n%s", command, len(line), line)
			}
			if home != "" && strings.Contains(line, home) {
				t.Fatalf("Help for %q should use ~ instead of %q on:\n%s", command, home, line)
			}
		}
	}

	expected := 39
	if count != expected {
		t.Fatalf("Wrong # of commands (%d), it should be: %d\nThe list:\n%s",
			len(cmds), expected, cmds)
	}

	logDone("help - cmd widths")
}
