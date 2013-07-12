package docker

import (
	"fmt"
	"io/ioutil"
	"testing"
)

// mkTestContext generates a build context from the contents of the provided dockerfile.
// This context is suitable for use as an argument to BuildFile.Build()
func mkTestContext(dockerfile string, files [][2]string, t *testing.T) Archive {
	context, err := mkBuildContext(fmt.Sprintf(dockerfile, unitTestImageID), files)
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
}

// A table of all the contexts to build and test.
// A new docker runtime will be created and torn down for each context.
var testContexts = []testContextTemplate{
	{
		`
from   %s
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd
run    [ "$(cat /tmp/passwd)" = "root:testpass" ]
run    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]
`,
		nil,
	},

	{
		`
from %s
add foo /usr/lib/bla/bar
run [ "$(cat /usr/lib/bla/bar)" = 'hello world!' ]
`,
		[][2]string{{"foo", "hello world!"}},
	},

	{
		`
from %s
add f /
run [ "$(cat /f)" = "hello" ]
add f /abc
run [ "$(cat /abc)" = "hello" ]
add f /x/y/z
run [ "$(cat /x/y/z)" = "hello" ]
add f /x/y/d/
run [ "$(cat /x/y/d/f)" = "hello" ]
add d /
run [ "$(cat /ga)" = "bu" ]
add d /somewhere
run [ "$(cat /somewhere/ga)" = "bu" ]
add d /anotherplace/
run [ "$(cat /anotherplace/ga)" = "bu" ]
add d /somewheeeere/over/the/rainbooow
run [ "$(cat /somewheeeere/over/the/rainbooow/ga)" = "bu" ]
`,
		[][2]string{
			{"f", "hello"},
			{"d/ga", "bu"},
		},
	},

	{
		`
from %s
env    FOO BAR
run    [ "$FOO" = "BAR" ]
`,
		nil,
	},

	{
		`
from %s
ENTRYPOINT /bin/echo
CMD Hello world
`,
		nil,
	},

	{
		`
from %s
VOLUME /test
CMD Hello world
`,
		nil,
	},
}

// FIXME: test building with 2 successive overlapping ADD commands

func TestBuild(t *testing.T) {
	for _, ctx := range testContexts {
		runtime, err := newTestRuntime()
		if err != nil {
			t.Fatal(err)
		}
		defer nuke(runtime)

		srv := &Server{
			runtime:     runtime,
			pullingPool: make(map[string]struct{}),
			pushingPool: make(map[string]struct{}),
		}

		buildfile := NewBuildFile(srv, ioutil.Discard, false)
		if _, err := buildfile.Build(mkTestContext(ctx.dockerfile, ctx.files, t)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVolume(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{
		runtime:     runtime,
		pullingPool: make(map[string]struct{}),
		pushingPool: make(map[string]struct{}),
	}

	buildfile := NewBuildFile(srv, ioutil.Discard, false)
	imgId, err := buildfile.Build(mkTestContext(`
from %s
VOLUME /test
CMD Hello world
`, nil, t))
	if err != nil {
		t.Fatal(err)
	}
	img, err := srv.ImageInspect(imgId)
	if err != nil {
		t.Fatal(err)
	}
	if len(img.Config.Volumes) == 0 {
		t.Fail()
	}
	for key := range img.Config.Volumes {
		if key != "/test" {
			t.Fail()
		}
	}
}
