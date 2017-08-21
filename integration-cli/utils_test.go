package main

import (
	"os/exec"

	"strings"

	"github.com/docker/docker/pkg/testutil/cmd"
)

func getPrefixAndSlashFromDaemonPlatform() (prefix, slash string) {
	if testEnv.DaemonPlatform() == "windows" {
		return "c:", `\`
	}
	return "", "/"
}

// TODO: update code to call cmd.RunCmd directly, and remove this function
// Deprecated: use pkg/testutil/cmd instead
func runCommandWithOutput(execCmd *exec.Cmd) (string, int, error) {
	result := cmd.RunCmd(transformCmd(execCmd))
	return result.Combined(), result.ExitCode, result.Error
}

// Temporary shim for migrating commands to the new function
func transformCmd(execCmd *exec.Cmd) cmd.Cmd {
	return cmd.Cmd{
		Command: execCmd.Args,
		Env:     execCmd.Env,
		Dir:     execCmd.Dir,
		Stdin:   execCmd.Stdin,
		Stdout:  execCmd.Stdout,
	}
}

// ParseCgroupPaths parses 'procCgroupData', which is output of '/proc/<pid>/cgroup', and returns
// a map which cgroup name as key and path as value.
func ParseCgroupPaths(procCgroupData string) map[string]string {
	cgroupPaths := map[string]string{}
	for _, line := range strings.Split(procCgroupData, "\n") {
		parts := strings.Split(line, ":")
		if len(parts) != 3 {
			continue
		}
		cgroupPaths[parts[1]] = parts[2]
	}
	return cgroupPaths
}
