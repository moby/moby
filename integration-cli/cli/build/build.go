package build

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"gotest.tools/v3/icmd"
)

// WithStdinContext sets the build context from the standard input with the specified reader
func WithStdinContext(closer io.ReadCloser) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Command = append(cmd.Command, "-")
		cmd.Stdin = closer
		return func() {
			// FIXME(vdemeester) we should not ignore the error here…
			closer.Close()
		}
	}
}

// WithDockerfile creates / returns a CmdOperator to set the Dockerfile for a build operation
func WithDockerfile(dockerfile string) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Command = append(cmd.Command, "-")
		cmd.Stdin = strings.NewReader(dockerfile)
		return nil
	}
}

// WithBuildkit sets an DOCKER_BUILDKIT environment variable to make the build use buildkit (or not)
func WithBuildkit(useBuildkit bool) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		val := "0"
		if useBuildkit {
			val = "1"
		}
		if cmd.Env == nil {
			cmd.Env = os.Environ()
		}
		cmd.Env = append(cmd.Env, "DOCKER_BUILDKIT="+val)
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

// WithExternalBuildContext use the specified context as build context
func WithExternalBuildContext(ctx *fakecontext.Fake) func(*icmd.Cmd) func() {
	return func(cmd *icmd.Cmd) func() {
		cmd.Dir = ctx.Dir
		cmd.Command = append(cmd.Command, ".")
		return nil
	}
}

// WithBuildContext sets up the build context
func WithBuildContext(t testing.TB, contextOperators ...func(*fakecontext.Fake) error) func(*icmd.Cmd) func() {
	// FIXME(vdemeester) de-duplicate that
	buildCtx := fakecontext.New(t, "", contextOperators...)
	return func(cmd *icmd.Cmd) func() {
		cmd.Dir = buildCtx.Dir
		cmd.Command = append(cmd.Command, ".")
		return closeBuildContext(t, buildCtx)
	}
}

// WithFile adds the specified file (with content) in the build context
func WithFile(name, content string) func(*fakecontext.Fake) error {
	return fakecontext.WithFile(name, content)
}

func closeBuildContext(t testing.TB, ctx *fakecontext.Fake) func() {
	return func() {
		if err := ctx.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
