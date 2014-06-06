package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/image"
	"github.com/dotcloud/docker/nat"
	"github.com/dotcloud/docker/server"
	"github.com/dotcloud/docker/utils"
)

// A testContextTemplate describes a build context and how to test it
type testContextTemplate struct {
	// Contents of the Dockerfile
	dockerfile string
	// Additional files in the context, eg [][2]string{"./passwd", "gordon"}
	files [][2]string
	// Additional remote files to host on a local HTTP server.
	remoteFiles [][2]string
}

func (context testContextTemplate) Archive(dockerfile string, t *testing.T) archive.Archive {
	input := []string{"Dockerfile", dockerfile}
	for _, pair := range context.files {
		input = append(input, pair[0], pair[1])
	}
	a, err := archive.Generate(input...)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// A table of all the contexts to build and test.
// A new docker runtime will be created and torn down for each context.
var testContexts = []testContextTemplate{
	{
		`
from   {IMAGE}
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd
run    [ "$(cat /tmp/passwd)" = "root:testpass" ]
run    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]
`,
		nil,
		nil,
	},

	// Exactly the same as above, except uses a line split with a \ to test
	// multiline support.
	{
		`
from   {IMAGE}
run    sh -c 'echo root:testpass \
	> /tmp/passwd'
run    mkdir -p /var/run/sshd
run    [ "$(cat /tmp/passwd)" = "root:testpass" ]
run    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]
`,
		nil,
		nil,
	},

	// Line containing literal "\n"
	{
		`
from   {IMAGE}
run    sh -c 'echo root:testpass > /tmp/passwd'
run    echo "foo \n bar"; echo "baz"
run    mkdir -p /var/run/sshd
run    [ "$(cat /tmp/passwd)" = "root:testpass" ]
run    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]
`,
		nil,
		nil,
	},
	{
		`
from {IMAGE}
add foo /usr/lib/bla/bar
run [ "$(cat /usr/lib/bla/bar)" = 'hello' ]
add http://{SERVERADDR}/baz /usr/lib/baz/quux
run [ "$(cat /usr/lib/baz/quux)" = 'world!' ]
`,
		[][2]string{{"foo", "hello"}},
		[][2]string{{"/baz", "world!"}},
	},

	{
		`
from {IMAGE}
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
		nil,
	},

	{
		`
from {IMAGE}
add http://{SERVERADDR}/x /a/b/c
run [ "$(cat /a/b/c)" = "hello" ]
add http://{SERVERADDR}/x?foo=bar /
run [ "$(cat /x)" = "hello" ]
add http://{SERVERADDR}/x /d/
run [ "$(cat /d/x)" = "hello" ]
add http://{SERVERADDR} /e
run [ "$(cat /e)" = "blah" ]
`,
		nil,
		[][2]string{{"/x", "hello"}, {"/", "blah"}},
	},

	// Comments, shebangs, and executability, oh my!
	{
		`
FROM {IMAGE}
# This is an ordinary comment.
RUN { echo '#!/bin/sh'; echo 'echo hello world'; } > /hello.sh
RUN [ ! -x /hello.sh ]
RUN chmod +x /hello.sh
RUN [ -x /hello.sh ]
RUN [ "$(cat /hello.sh)" = $'#!/bin/sh\necho hello world' ]
RUN [ "$(/hello.sh)" = "hello world" ]
`,
		nil,
		nil,
	},

	// Users and groups
	{
		`
FROM {IMAGE}

# Make sure our defaults work
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)" = '0:0/root:root' ]

# TODO decide if "args.user = strconv.Itoa(syscall.Getuid())" is acceptable behavior for changeUser in sysvinit instead of "return nil" when "USER" isn't specified (so that we get the proper group list even if that is the empty list, even in the default case of not supplying an explicit USER to run as, which implies USER 0)
USER root
RUN [ "$(id -G):$(id -Gn)" = '0:root' ]

# Setup dockerio user and group
RUN echo 'dockerio:x:1000:1000::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1000:' >> /etc/group

# Make sure we can switch to our user and all the information is exactly as we expect it to be
USER dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1000/dockerio:dockerio/1000:dockerio' ]

# Switch back to root and double check that worked exactly as we might expect it to
USER root
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '0:0/root:root/0:root' ]

# Add a "supplementary" group for our dockerio user
RUN echo 'supplementary:x:1001:dockerio' >> /etc/group

# ... and then go verify that we get it like we expect
USER dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1000/dockerio:dockerio/1000 1001:dockerio supplementary' ]
USER 1000
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1000/dockerio:dockerio/1000 1001:dockerio supplementary' ]

# super test the new "user:group" syntax
USER dockerio:dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1000/dockerio:dockerio/1000:dockerio' ]
USER 1000:dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1000/dockerio:dockerio/1000:dockerio' ]
USER dockerio:1000
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1000/dockerio:dockerio/1000:dockerio' ]
USER 1000:1000
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1000/dockerio:dockerio/1000:dockerio' ]
USER dockerio:supplementary
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1001/dockerio:supplementary/1001:supplementary' ]
USER dockerio:1001
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1001/dockerio:supplementary/1001:supplementary' ]
USER 1000:supplementary
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1001/dockerio:supplementary/1001:supplementary' ]
USER 1000:1001
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1000:1001/dockerio:supplementary/1001:supplementary' ]

# make sure unknown uid/gid still works properly
USER 1042:1043
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1042:1043/1042:1043/1043:1043' ]
`,
		nil,
		nil,
	},

	// Environment variable
	{
		`
from   {IMAGE}
env    FOO BAR
run    [ "$FOO" = "BAR" ]
`,
		nil,
		nil,
	},

	// Environment overwriting
	{
		`
from   {IMAGE}
env    FOO BAR
run    [ "$FOO" = "BAR" ]
env    FOO BAZ
run    [ "$FOO" = "BAZ" ]
`,
		nil,
		nil,
	},

	{
		`
from {IMAGE}
ENTRYPOINT /bin/echo
CMD Hello world
`,
		nil,
		nil,
	},

	{
		`
from {IMAGE}
VOLUME /test
CMD Hello world
`,
		nil,
		nil,
	},

	{
		`
from {IMAGE}
env    FOO /foo/baz
env    BAR /bar
env    BAZ $BAR
env    FOOPATH $PATH:$FOO
run    [ "$BAR" = "$BAZ" ]
run    [ "$FOOPATH" = "$PATH:/foo/baz" ]
`,
		nil,
		nil,
	},

	{
		`
from {IMAGE}
env    FOO /bar
env    TEST testdir
env    BAZ /foobar
add    testfile $BAZ/
add    $TEST $FOO
run    [ "$(cat /foobar/testfile)" = "test1" ]
run    [ "$(cat /bar/withfile)" = "test2" ]
`,
		[][2]string{
			{"testfile", "test1"},
			{"testdir/withfile", "test2"},
		},
		nil,
	},

	// JSON!
	{
		`
FROM {IMAGE}
RUN ["/bin/echo","hello","world"]
CMD ["/bin/true"]
ENTRYPOINT ["/bin/echo","your command -->"]
`,
		nil,
		nil,
	},
	{
		`
FROM {IMAGE}
ADD test /test
RUN ["chmod","+x","/test"]
RUN ["/test"]
RUN [ "$(cat /testfile)" = 'test!' ]
`,
		[][2]string{
			{"test", "#!/bin/sh\necho 'test!' > /testfile"},
		},
		nil,
	},
	{
		`
FROM {IMAGE}
# what \
RUN mkdir /testing
RUN touch /testing/other
`,
		nil,
		nil,
	},
}

// FIXME: test building with 2 successive overlapping ADD commands

func constructDockerfile(template string, ip net.IP, port string) string {
	serverAddr := fmt.Sprintf("%s:%s", ip, port)
	replacer := strings.NewReplacer("{IMAGE}", unitTestImageID, "{SERVERADDR}", serverAddr)
	return replacer.Replace(template)
}

func mkTestingFileServer(files [][2]string) (*httptest.Server, error) {
	mux := http.NewServeMux()
	for _, file := range files {
		name, contents := file[0], file[1]
		mux.HandleFunc(name, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(contents))
		})
	}

	// This is how httptest.NewServer sets up a net.Listener, except that our listener must accept remote
	// connections (from the container).
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	s := httptest.NewUnstartedServer(mux)
	s.Listener = listener
	s.Start()
	return s, nil
}

func TestBuild(t *testing.T) {
	for _, ctx := range testContexts {
		_, err := buildImage(ctx, t, nil, true)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func buildImage(context testContextTemplate, t *testing.T, eng *engine.Engine, useCache bool) (*image.Image, error) {
	if eng == nil {
		eng = NewTestEngine(t)
		runtime := mkDaemonFromEngine(eng, t)
		// FIXME: we might not need runtime, why not simply nuke
		// the engine?
		defer nuke(runtime)
	}
	srv := mkServerFromEngine(eng, t)

	httpServer, err := mkTestingFileServer(context.remoteFiles)
	if err != nil {
		t.Fatal(err)
	}
	defer httpServer.Close()

	idx := strings.LastIndex(httpServer.URL, ":")
	if idx < 0 {
		t.Fatalf("could not get port from test http server address %s", httpServer.URL)
	}
	port := httpServer.URL[idx+1:]

	iIP := eng.Hack_GetGlobalVar("httpapi.bridgeIP")
	if iIP == nil {
		t.Fatal("Legacy bridgeIP field not set in engine")
	}
	ip, ok := iIP.(net.IP)
	if !ok {
		panic("Legacy bridgeIP field in engine does not cast to net.IP")
	}
	dockerfile := constructDockerfile(context.dockerfile, ip, port)

	buildfile := server.NewBuildFile(srv, ioutil.Discard, ioutil.Discard, false, useCache, false, false, ioutil.Discard, utils.NewStreamFormatter(false), nil, nil)
	id, err := buildfile.Build(context.Archive(dockerfile, t))
	if err != nil {
		return nil, err
	}

	job := eng.Job("image_inspect", id)
	buffer := bytes.NewBuffer(nil)
	image := &image.Image{}
	job.Stdout.Add(buffer)
	if err := job.Run(); err != nil {
		return nil, err
	}
	err = json.NewDecoder(buffer).Decode(image)
	return image, err
}

// testing #1405 - config.Cmd does not get cleaned up if
// utilizing cache
func TestBuildEntrypointRunCleanup(t *testing.T) {
	eng := NewTestEngine(t)
	defer nuke(mkDaemonFromEngine(eng, t))

	img, err := buildImage(testContextTemplate{`
        from {IMAGE}
        run echo "hello"
        `,
		nil, nil}, t, eng, true)
	if err != nil {
		t.Fatal(err)
	}

	img, err = buildImage(testContextTemplate{`
        from {IMAGE}
        run echo "hello"
        add foo /foo
        entrypoint ["/bin/echo"]
        `,
		[][2]string{{"foo", "HEYO"}}, nil}, t, eng, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(img.Config.Cmd) != 0 {
		t.Fail()
	}
}

func TestForbiddenContextPath(t *testing.T) {
	eng := NewTestEngine(t)
	defer nuke(mkDaemonFromEngine(eng, t))
	srv := mkServerFromEngine(eng, t)

	context := testContextTemplate{`
        from {IMAGE}
        maintainer dockerio
        add ../../ test/
        `,
		[][2]string{{"test.txt", "test1"}, {"other.txt", "other"}}, nil}

	httpServer, err := mkTestingFileServer(context.remoteFiles)
	if err != nil {
		t.Fatal(err)
	}
	defer httpServer.Close()

	idx := strings.LastIndex(httpServer.URL, ":")
	if idx < 0 {
		t.Fatalf("could not get port from test http server address %s", httpServer.URL)
	}
	port := httpServer.URL[idx+1:]

	iIP := eng.Hack_GetGlobalVar("httpapi.bridgeIP")
	if iIP == nil {
		t.Fatal("Legacy bridgeIP field not set in engine")
	}
	ip, ok := iIP.(net.IP)
	if !ok {
		panic("Legacy bridgeIP field in engine does not cast to net.IP")
	}
	dockerfile := constructDockerfile(context.dockerfile, ip, port)

	buildfile := server.NewBuildFile(srv, ioutil.Discard, ioutil.Discard, false, true, false, false, ioutil.Discard, utils.NewStreamFormatter(false), nil, nil)
	_, err = buildfile.Build(context.Archive(dockerfile, t))

	if err == nil {
		t.Log("Error should not be nil")
		t.Fail()
	}

	if err.Error() != "Forbidden path outside the build context: ../../ (/)" {
		t.Logf("Error message is not expected: %s", err.Error())
		t.Fail()
	}
}

func TestBuildADDFileNotFound(t *testing.T) {
	eng := NewTestEngine(t)
	defer nuke(mkDaemonFromEngine(eng, t))

	context := testContextTemplate{`
        from {IMAGE}
        add foo /usr/local/bar
        `,
		nil, nil}

	httpServer, err := mkTestingFileServer(context.remoteFiles)
	if err != nil {
		t.Fatal(err)
	}
	defer httpServer.Close()

	idx := strings.LastIndex(httpServer.URL, ":")
	if idx < 0 {
		t.Fatalf("could not get port from test http server address %s", httpServer.URL)
	}
	port := httpServer.URL[idx+1:]

	iIP := eng.Hack_GetGlobalVar("httpapi.bridgeIP")
	if iIP == nil {
		t.Fatal("Legacy bridgeIP field not set in engine")
	}
	ip, ok := iIP.(net.IP)
	if !ok {
		panic("Legacy bridgeIP field in engine does not cast to net.IP")
	}
	dockerfile := constructDockerfile(context.dockerfile, ip, port)

	buildfile := server.NewBuildFile(mkServerFromEngine(eng, t), ioutil.Discard, ioutil.Discard, false, true, false, false, ioutil.Discard, utils.NewStreamFormatter(false), nil, nil)
	_, err = buildfile.Build(context.Archive(dockerfile, t))

	if err == nil {
		t.Log("Error should not be nil")
		t.Fail()
	}

	if err.Error() != "foo: no such file or directory" {
		t.Logf("Error message is not expected: %s", err.Error())
		t.Fail()
	}
}

func TestBuildInheritance(t *testing.T) {
	eng := NewTestEngine(t)
	defer nuke(mkDaemonFromEngine(eng, t))

	img, err := buildImage(testContextTemplate{`
            from {IMAGE}
            expose 2375
            `,
		nil, nil}, t, eng, true)

	if err != nil {
		t.Fatal(err)
	}

	img2, _ := buildImage(testContextTemplate{fmt.Sprintf(`
            from %s
            entrypoint ["/bin/echo"]
            `, img.ID),
		nil, nil}, t, eng, true)

	if err != nil {
		t.Fatal(err)
	}

	// from child
	if img2.Config.Entrypoint[0] != "/bin/echo" {
		t.Fail()
	}

	// from parent
	if _, exists := img.Config.ExposedPorts[nat.NewPort("tcp", "2375")]; !exists {
		t.Fail()
	}
}

func TestBuildFails(t *testing.T) {
	_, err := buildImage(testContextTemplate{`
        from {IMAGE}
        run sh -c "exit 23"
        `,
		nil, nil}, t, nil, true)

	if err == nil {
		t.Fatal("Error should not be nil")
	}

	sterr, ok := err.(*utils.JSONError)
	if !ok {
		t.Fatalf("Error should be utils.JSONError")
	}
	if sterr.Code != 23 {
		t.Fatalf("StatusCode %d unexpected, should be 23", sterr.Code)
	}
}

func TestBuildFailsDockerfileEmpty(t *testing.T) {
	_, err := buildImage(testContextTemplate{``, nil, nil}, t, nil, true)

	if err != server.ErrDockerfileEmpty {
		t.Fatal("Expected: %v, got: %v", server.ErrDockerfileEmpty, err)
	}
}

func TestBuildOnBuildTrigger(t *testing.T) {
	_, err := buildImage(testContextTemplate{`
	from {IMAGE}
	onbuild run echo here is the trigger
	onbuild run touch foobar
	`,
		nil, nil,
	},
		t, nil, true,
	)
	if err != nil {
		t.Fatal(err)
	}
	// FIXME: test that the 'foobar' file was created in the final build.
}

func TestBuildOnBuildForbiddenChainedTrigger(t *testing.T) {
	_, err := buildImage(testContextTemplate{`
	from {IMAGE}
	onbuild onbuild run echo test
	`,
		nil, nil,
	},
		t, nil, true,
	)
	if err == nil {
		t.Fatal("Error should not be nil")
	}
}

func TestBuildOnBuildForbiddenFromTrigger(t *testing.T) {
	_, err := buildImage(testContextTemplate{`
	from {IMAGE}
	onbuild from {IMAGE}
	`,
		nil, nil,
	},
		t, nil, true,
	)
	if err == nil {
		t.Fatal("Error should not be nil")
	}
}

func TestBuildOnBuildForbiddenMaintainerTrigger(t *testing.T) {
	_, err := buildImage(testContextTemplate{`
	from {IMAGE}
	onbuild maintainer test
	`,
		nil, nil,
	},
		t, nil, true,
	)
	if err == nil {
		t.Fatal("Error should not be nil")
	}
}

// gh #2446
func TestBuildAddToSymlinkDest(t *testing.T) {
	eng := NewTestEngine(t)
	defer nuke(mkDaemonFromEngine(eng, t))

	_, err := buildImage(testContextTemplate{`
        from {IMAGE}
        run mkdir /foo
        run ln -s /foo /bar
        add foo /bar/
        run stat /bar/foo
        `,
		[][2]string{{"foo", "HEYO"}}, nil}, t, eng, true)
	if err != nil {
		t.Fatal(err)
	}
}
