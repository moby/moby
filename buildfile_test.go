package docker

import (
	"io/ioutil"
	"testing"
)

// mkTestContext generates a build context from the contents of the provided dockerfile.
// This context is suitable for use as an argument to BuildFile.Build()
func mkTestContext(dockerfile string, files [][2]string, t *testing.T) Archive {
	context, err := mkBuildContext(dockerfile, files)
	if err != nil {
		t.Fatal(err)
	}
	return context
}

// A testContextTemplate describes a build context and how to test it
type testContextTemplate struct {
	// Contents of the Dockerfile
	dockerfile string
	// Additional files in the context, eg [][2]string{"./passwd", "gordon"}
	files [][2]string
	// Test commands to run in the resulting image
	tests []testCommand
}

// A testCommand describes a command to run in a container, and the exact output required to pass the test
type testCommand struct {
	// The command to run, eg. []string{"echo", "hello", "world"}
	cmd []string
	// The exact output expected, eg. "hello world\n"
	output string
}

// A table of all the contexts to build and test.
// A new docker runtime will be created and torn down for each context.
var testContexts []testContextTemplate = []testContextTemplate{
	{
		`
# VERSION		0.1
# DOCKER-VERSION	0.2

from   docker-ut
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd
`,
		nil,
		[]testCommand{
			{[]string{"cat", "/tmp/passwd"}, "root:testpass\n"},
			{[]string{"ls", "-d", "/var/run/sshd"}, "/var/run/sshd\n"},
		},
	},

	{
		`
from docker-ut
add foo /usr/lib/bla/bar`,
		[][2]string{{"foo", "hello world!"}},
		[]testCommand{
			{[]string{"cat", "/usr/lib/bla/bar"}, "hello world!"},
		},
	},
}

// FIXME: test building with a context

// FIXME: test building with a local ADD as first command

// FIXME: test building with 2 successive overlapping ADD commands

func TestBuild(t *testing.T) {
	for _, ctx := range testContexts {
		runtime, err := newTestRuntime()
		if err != nil {
			t.Fatal(err)
		}
		defer nuke(runtime)

		srv := &Server{runtime: runtime}

		buildfile := NewBuildFile(srv, ioutil.Discard)

		imgID, err := buildfile.Build(mkTestContext(ctx.dockerfile, ctx.files, t))
		if err != nil {
			t.Fatal(err)
		}

		builder := NewBuilder(runtime)
		for _, testCmd := range ctx.tests {
			container, err := builder.Create(
				&Config{
					Image: imgID,
					Cmd:   testCmd.cmd,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Destroy(container)

			output, err := container.Output()
			if err != nil {
				t.Fatal(err)
			}
			if string(output) != testCmd.output {
				t.Fatalf("Unexpected output. Read '%s', expected '%s'", output, testCmd.output)
			}
		}
	}
}
