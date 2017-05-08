package main

import (
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/docker/docker/pkg/integration"
	"github.com/docker/docker/pkg/integration/cmd"
)

func getPrefixAndSlashFromDaemonPlatform() (prefix, slash string) {
	if daemonPlatform == "windows" {
		return "c:", `\`
	}
	return "", "/"
}

// TODO: update code to call cmd.RunCmd directly, and remove this function
func runCommandWithOutput(execCmd *exec.Cmd) (string, int, error) {
	result := cmd.RunCmd(transformCmd(execCmd))
	return result.Combined(), result.ExitCode, result.Error
}

// TODO: update code to call cmd.RunCmd directly, and remove this function
func runCommandWithStdoutStderr(execCmd *exec.Cmd) (string, string, int, error) {
	result := cmd.RunCmd(transformCmd(execCmd))
	return result.Stdout(), result.Stderr(), result.ExitCode, result.Error
}

// TODO: update code to call cmd.RunCmd directly, and remove this function
func runCommand(execCmd *exec.Cmd) (exitCode int, err error) {
	result := cmd.RunCmd(transformCmd(execCmd))
	return result.ExitCode, result.Error
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

func runCommandPipelineWithOutput(cmds ...*exec.Cmd) (output string, exitCode int, err error) {
	return integration.RunCommandPipelineWithOutput(cmds...)
}

func convertSliceOfStringsToMap(input []string) map[string]struct{} {
	return integration.ConvertSliceOfStringsToMap(input)
}

func compareDirectoryEntries(e1 []os.FileInfo, e2 []os.FileInfo) error {
	return integration.CompareDirectoryEntries(e1, e2)
}

func listTar(f io.Reader) ([]string, error) {
	return integration.ListTar(f)
}

func randomTmpDirPath(s string, platform string) string {
	return integration.RandomTmpDirPath(s, platform)
}

func consumeWithSpeed(reader io.Reader, chunkSize int, interval time.Duration, stop chan bool) (n int, err error) {
	return integration.ConsumeWithSpeed(reader, chunkSize, interval, stop)
}

func parseCgroupPaths(procCgroupData string) map[string]string {
	return integration.ParseCgroupPaths(procCgroupData)
}

func runAtDifferentDate(date time.Time, block func()) {
	integration.RunAtDifferentDate(date, block)
}
