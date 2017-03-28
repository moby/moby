package build

import (
	"strings"

	icmd "github.com/docker/docker/pkg/testutil/cmd"
)

// WithDockerfile creates / returns a CmdOperator to set the Dockerfile for a build operation
func WithDockerfile(dockerfile string) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Command = append(cmd.Command, "-")
		cmd.Stdin = strings.NewReader(dockerfile)
		return nil
	}
}

// WithoutCache makes the build ignore cache
func WithoutCache(cmd *icmd.Cmd) func() {
	cmd.Command = append(cmd.Command, "--no-cache")
	return nil
}

// WithContextPath sets the build context path
func WithContextPath(path string) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Command = append(cmd.Command, path)
		return nil
	}
}
