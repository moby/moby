package main

import (
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/docker/docker/pkg/integration"
)

func getExitCode(err error) (int, error) {
	return integration.GetExitCode(err)
}

func processExitCode(err error) (exitCode int) {
	return integration.ProcessExitCode(err)
}

func isKilled(err error) bool {
	return integration.IsKilled(err)
}

func runCommandWithOutput(cmd *exec.Cmd) (output string, exitCode int, err error) {
	return integration.RunCommandWithOutput(cmd)
}

func runCommandWithStdoutStderr(cmd *exec.Cmd) (stdout string, stderr string, exitCode int, err error) {
	return integration.RunCommandWithStdoutStderr(cmd)
}

func runCommandWithOutputForDuration(cmd *exec.Cmd, duration time.Duration) (output string, exitCode int, timedOut bool, err error) {
	return integration.RunCommandWithOutputForDuration(cmd, duration)
}

func runCommandWithOutputAndTimeout(cmd *exec.Cmd, timeout time.Duration) (output string, exitCode int, err error) {
	return integration.RunCommandWithOutputAndTimeout(cmd, timeout)
}

func runCommand(cmd *exec.Cmd) (exitCode int, err error) {
	return integration.RunCommand(cmd)
}

func runCommandPipelineWithOutput(cmds ...*exec.Cmd) (output string, exitCode int, err error) {
	return integration.RunCommandPipelineWithOutput(cmds...)
}

func unmarshalJSON(data []byte, result interface{}) error {
	return integration.UnmarshalJSON(data, result)
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
