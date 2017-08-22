package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/stringutils"
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

// RandomTmpDirPath provides a temporary path with rand string appended.
// does not create or checks if it exists.
func RandomTmpDirPath(s string, platform string) string {
	// TODO: why doesn't this use os.TempDir() ?
	tmp := "/tmp"
	if platform == "windows" {
		tmp = os.Getenv("TEMP")
	}
	path := filepath.Join(tmp, fmt.Sprintf("%s.%s", s, stringutils.GenerateRandomAlphaOnlyString(10)))
	if platform == "windows" {
		return filepath.FromSlash(path) // Using \
	}
	return filepath.ToSlash(path) // Using /
}
