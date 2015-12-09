package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildJSONEmptyRun(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildjsonemptyrun"

	_, err := buildImage(
		name,
		`
    FROM busybox
    RUN []
    `,
		true)

	if err != nil {
		c.Fatal("error when dealing with a RUN statement with empty JSON array")
	}

}

func (s *DockerSuite) TestBuildEmptyWhitespace(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildemptywhitespace"

	_, err := buildImage(
		name,
		`
    FROM busybox
    COPY
      quux \
      bar
    `,
		true)

	if err == nil {
		c.Fatal("no error when dealing with a COPY statement with no content on the same line")
	}

}

func (s *DockerSuite) TestBuildShCmdJSONEntrypoint(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildshcmdjsonentrypoint"

	_, err := buildImage(
		name,
		`
    FROM busybox
    ENTRYPOINT ["/bin/echo"]
    CMD echo test
    `,
		true)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "--rm", name)

	if strings.TrimSpace(out) != "/bin/sh -c echo test" {
		c.Fatalf("CMD did not contain /bin/sh -c : %s", out)
	}

}

func (s *DockerSuite) TestBuildHandleEscapes(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildhandleescapes"

	_, err := buildImage(name,
		`
  FROM scratch
  ENV FOO bar
  VOLUME ${FOO}
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	var result map[string]map[string]struct{}

	res, err := inspectFieldJSON(name, "Config.Volumes")
	if err != nil {
		c.Fatal(err)
	}

	if err = unmarshalJSON([]byte(res), &result); err != nil {
		c.Fatal(err)
	}

	if _, ok := result["bar"]; !ok {
		c.Fatal("Could not find volume bar set from env foo in volumes table")
	}

	deleteImages(name)

	_, err = buildImage(name,
		`
  FROM scratch
  ENV FOO bar
  VOLUME \${FOO}
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	res, err = inspectFieldJSON(name, "Config.Volumes")
	if err != nil {
		c.Fatal(err)
	}

	if err = unmarshalJSON([]byte(res), &result); err != nil {
		c.Fatal(err)
	}

	if _, ok := result["${FOO}"]; !ok {
		c.Fatal("Could not find volume ${FOO} set from env foo in volumes table")
	}

	deleteImages(name)

	// this test in particular provides *7* backslashes and expects 6 to come back.
	// Like above, the first escape is swallowed and the rest are treated as
	// literals, this one is just less obvious because of all the character noise.

	_, err = buildImage(name,
		`
  FROM scratch
  ENV FOO bar
  VOLUME \\\\\\\${FOO}
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	res, err = inspectFieldJSON(name, "Config.Volumes")
	if err != nil {
		c.Fatal(err)
	}

	if err = unmarshalJSON([]byte(res), &result); err != nil {
		c.Fatal(err)
	}

	if _, ok := result[`\\\${FOO}`]; !ok {
		c.Fatal(`Could not find volume \\\${FOO} set from env foo in volumes table`, result)
	}

}

func (s *DockerSuite) TestBuildLastModified(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildlastmodified"

	server, err := fakeStorage(map[string]string{
		"file": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	var out, out2 string

	dFmt := `FROM busybox
ADD %s/file /
RUN ls -le /file`

	dockerfile := fmt.Sprintf(dFmt, server.URL())

	if _, out, err = buildImageWithOut(name, dockerfile, false); err != nil {
		c.Fatal(err)
	}

	originMTime := regexp.MustCompile(`root.*/file.*\n`).FindString(out)
	// Make sure our regexp is correct
	if strings.Index(originMTime, "/file") < 0 {
		c.Fatalf("Missing ls info on 'file':\n%s", out)
	}

	// Build it again and make sure the mtime of the file didn't change.
	// Wait a few seconds to make sure the time changed enough to notice
	time.Sleep(2 * time.Second)

	if _, out2, err = buildImageWithOut(name, dockerfile, false); err != nil {
		c.Fatal(err)
	}

	newMTime := regexp.MustCompile(`root.*/file.*\n`).FindString(out2)
	if newMTime != originMTime {
		c.Fatalf("MTime changed:\nOrigin:%s\nNew:%s", originMTime, newMTime)
	}

	// Now 'touch' the file and make sure the timestamp DID change this time
	// Create a new fakeStorage instead of just using Add() to help windows
	server, err = fakeStorage(map[string]string{
		"file": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	dockerfile = fmt.Sprintf(dFmt, server.URL())

	if _, out2, err = buildImageWithOut(name, dockerfile, false); err != nil {
		c.Fatal(err)
	}

	newMTime = regexp.MustCompile(`root.*/file.*\n`).FindString(out2)
	if newMTime == originMTime {
		c.Fatalf("MTime didn't change:\nOrigin:%s\nNew:%s", originMTime, newMTime)
	}

}

func (s *DockerSuite) TestBuildSixtySteps(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "foobuildsixtysteps"
	ctx, err := fakeContext("FROM scratch\n"+strings.Repeat("ADD foo /\n", 60),
		map[string]string{
			"foo": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildForceRm(c *check.C) {
	testRequires(c, DaemonIsLinux)
	containerCountBefore, err := getContainerCount()
	if err != nil {
		c.Fatalf("failed to get the container count: %s", err)
	}
	name := "testbuildforcerm"
	ctx, err := fakeContext("FROM scratch\nRUN true\nRUN thiswillfail", nil)
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	dockerCmdInDir(c, ctx.Dir, "build", "-t", name, "--force-rm", ".")

	containerCountAfter, err := getContainerCount()
	if err != nil {
		c.Fatalf("failed to get the container count: %s", err)
	}

	if containerCountBefore != containerCountAfter {
		c.Fatalf("--force-rm shouldn't have left containers behind")
	}

}

// Test that an infinite sleep during a build is killed if the client disconnects.
// This test is fairly hairy because there are lots of ways to race.
// Strategy:
// * Monitor the output of docker events starting from before
// * Run a 1-year-long sleep from a docker build.
// * When docker events sees container start, close the "docker build" command
// * Wait for docker events to emit a dying event.
func (s *DockerSuite) TestBuildCancellationKillsSleep(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildcancellation"

	// (Note: one year, will never finish)
	ctx, err := fakeContext("FROM busybox\nRUN sleep 31536000", nil)
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	finish := make(chan struct{})
	defer close(finish)

	eventStart := make(chan struct{})
	eventDie := make(chan struct{})
	containerID := make(chan string)

	startEpoch := daemonTime(c).Unix()
	// Watch for events since epoch.
	eventsCmd := exec.Command(dockerBinary, "events", "--since", strconv.FormatInt(startEpoch, 10))
	stdout, err := eventsCmd.StdoutPipe()
	if err != nil {
		c.Fatal(err)
	}
	if err := eventsCmd.Start(); err != nil {
		c.Fatal(err)
	}
	defer eventsCmd.Process.Kill()

	// Goroutine responsible for watching start/die events from `docker events`
	go func() {
		cid := <-containerID

		matchStart := regexp.MustCompile(cid + `(.*) start$`)
		matchDie := regexp.MustCompile(cid + `(.*) die$`)

		//
		// Read lines of `docker events` looking for container start and stop.
		//
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			switch {
			case matchStart.MatchString(scanner.Text()):
				close(eventStart)
			case matchDie.MatchString(scanner.Text()):
				close(eventDie)
			}
		}
	}()

	buildCmd := exec.Command(dockerBinary, "build", "-t", name, ".")
	buildCmd.Dir = ctx.Dir

	stdoutBuild, err := buildCmd.StdoutPipe()
	if err := buildCmd.Start(); err != nil {
		c.Fatalf("failed to run build: %s", err)
	}

	matchCID := regexp.MustCompile("Running in (.+)")
	scanner := bufio.NewScanner(stdoutBuild)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := matchCID.FindStringSubmatch(line); len(matches) > 0 {
			containerID <- matches[1]
			break
		}
	}

	select {
	case <-time.After(5 * time.Second):
		c.Fatal("failed to observe build container start in timely fashion")
	case <-eventStart:
		// Proceeds from here when we see the container fly past in the
		// output of "docker events".
		// Now we know the container is running.
	}

	// Send a kill to the `docker build` command.
	// Causes the underlying build to be cancelled due to socket close.
	if err := buildCmd.Process.Kill(); err != nil {
		c.Fatalf("error killing build command: %s", err)
	}

	// Get the exit status of `docker build`, check it exited because killed.
	if err := buildCmd.Wait(); err != nil && !isKilled(err) {
		c.Fatalf("wait failed during build run: %T %s", err, err)
	}

	select {
	case <-time.After(5 * time.Second):
		// If we don't get here in a timely fashion, it wasn't killed.
		c.Fatal("container cancel did not succeed")
	case <-eventDie:
		// We saw the container shut down in the `docker events` stream,
		// as expected.
	}

}

func (s *DockerSuite) TestBuildRm(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildrm"
	ctx, err := fakeContext("FROM scratch\nADD foo /\nADD foo /", map[string]string{"foo": "bar"})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "--rm", "-t", name, ".")

		if err != nil {
			c.Fatal("failed to build the image", out)
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore != containerCountAfter {
			c.Fatalf("-rm shouldn't have left containers behind")
		}
		deleteImages(name)
	}

	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-t", name, ".")

		if err != nil {
			c.Fatal("failed to build the image", out)
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore != containerCountAfter {
			c.Fatalf("--rm shouldn't have left containers behind")
		}
		deleteImages(name)
	}

	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "--rm=false", "-t", name, ".")

		if err != nil {
			c.Fatal("failed to build the image", out)
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore == containerCountAfter {
			c.Fatalf("--rm=false should have left containers behind")
		}
		deleteImages(name)

	}

}

func (s *DockerSuite) TestBuildWithVolumes(c *check.C) {
	testRequires(c, DaemonIsLinux)
	var (
		result   map[string]map[string]struct{}
		name     = "testbuildvolumes"
		emptyMap = make(map[string]struct{})
		expected = map[string]map[string]struct{}{
			"/test1":  emptyMap,
			"/test2":  emptyMap,
			"/test3":  emptyMap,
			"/test4":  emptyMap,
			"/test5":  emptyMap,
			"/test6":  emptyMap,
			"[/test7": emptyMap,
			"/test8]": emptyMap,
		}
	)
	_, err := buildImage(name,
		`FROM scratch
		VOLUME /test1
		VOLUME /test2
    VOLUME /test3 /test4
    VOLUME ["/test5", "/test6"]
    VOLUME [/test7 /test8]
    `,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectFieldJSON(name, "Config.Volumes")
	if err != nil {
		c.Fatal(err)
	}

	err = unmarshalJSON([]byte(res), &result)
	if err != nil {
		c.Fatal(err)
	}

	equal := reflect.DeepEqual(&result, &expected)

	if !equal {
		c.Fatalf("Volumes %s, expected %s", result, expected)
	}

}

func (s *DockerSuite) TestBuildMaintainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildmaintainer"
	expected := "dockerio"
	_, err := buildImage(name,
		`FROM scratch
        MAINTAINER dockerio`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Author")
	if err != nil {
		c.Fatal(err)
	}
	if res != expected {
		c.Fatalf("Maintainer %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildUser(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuilduser"
	expected := "dockerio"
	_, err := buildImage(name,
		`FROM busybox
		RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
		USER dockerio
		RUN [ $(whoami) = 'dockerio' ]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Config.User")
	if err != nil {
		c.Fatal(err)
	}
	if res != expected {
		c.Fatalf("User %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildRelativeWorkdir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildrelativeworkdir"
	expected := "/test2/test3"
	_, err := buildImage(name,
		`FROM busybox
		RUN [ "$PWD" = '/' ]
		WORKDIR test1
		RUN [ "$PWD" = '/test1' ]
		WORKDIR /test2
		RUN [ "$PWD" = '/test2' ]
		WORKDIR test3
		RUN [ "$PWD" = '/test2/test3' ]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Config.WorkingDir")
	if err != nil {
		c.Fatal(err)
	}
	if res != expected {
		c.Fatalf("Workdir %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildCmd(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildcmd"
	expected := "{[/bin/echo Hello World]}"
	_, err := buildImage(name,
		`FROM scratch
        CMD ["/bin/echo", "Hello World"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Config.Cmd")
	if err != nil {
		c.Fatal(err)
	}
	if res != expected {
		c.Fatalf("Cmd %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildEmptyEntrypointInheritance(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildentrypointinheritance"
	name2 := "testbuildentrypointinheritance2"

	_, err := buildImage(name,
		`FROM busybox
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Config.Entrypoint")
	if err != nil {
		c.Fatal(err)
	}

	expected := "{[/bin/echo]}"
	if res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

	_, err = buildImage(name2,
		fmt.Sprintf(`FROM %s
        ENTRYPOINT []`, name),
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err = inspectField(name2, "Config.Entrypoint")
	if err != nil {
		c.Fatal(err)
	}

	expected = "{[]}"

	if res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

}

func (s *DockerSuite) TestBuildEmptyEntrypoint(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildentrypoint"
	expected := "{[]}"

	_, err := buildImage(name,
		`FROM busybox
        ENTRYPOINT []`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Config.Entrypoint")
	if err != nil {
		c.Fatal(err)
	}
	if res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

}

func (s *DockerSuite) TestBuildEntrypoint(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildentrypoint"
	expected := "{[/bin/echo]}"
	_, err := buildImage(name,
		`FROM scratch
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Config.Entrypoint")
	if err != nil {
		c.Fatal(err)
	}
	if res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

}

func (s *DockerSuite) TestBuildWithCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildwithcache"
	id1, err := buildImage(name,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImage(name,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	if id1 != id2 {
		c.Fatal("The cache should have been used but hasn't.")
	}
}

func (s *DockerSuite) TestBuildWithoutCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildwithoutcache"
	name2 := "testbuildwithoutcache2"
	id1, err := buildImage(name,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	id2, err := buildImage(name2,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		false)
	if err != nil {
		c.Fatal(err)
	}
	if id1 == id2 {
		c.Fatal("The cache should have been invalided but hasn't.")
	}
}

func (s *DockerSuite) TestBuildConditionalCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildconditionalcache"

	dockerfile := `
		FROM busybox
        ADD foo /tmp/`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatalf("Error building #1: %s", err)
	}

	if err := ctx.Add("foo", "bye"); err != nil {
		c.Fatalf("Error modifying foo: %s", err)
	}

	id2, err := buildImageFromContext(name, ctx, false)
	if err != nil {
		c.Fatalf("Error building #2: %s", err)
	}
	if id2 == id1 {
		c.Fatal("Should not have used the cache")
	}

	id3, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatalf("Error building #3: %s", err)
	}
	if id3 != id2 {
		c.Fatal("Should have used the cache")
	}
}

func (s *DockerSuite) TestBuildWithVolumeOwnership(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildimg"

	_, err := buildImage(name,
		`FROM busybox:latest
        RUN mkdir /test && chown daemon:daemon /test && chmod 0600 /test
        VOLUME /test`,
		true)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "--rm", "testbuildimg", "ls", "-la", "/test")

	if expected := "drw-------"; !strings.Contains(out, expected) {
		c.Fatalf("expected %s received %s", expected, out)
	}

	if expected := "daemon   daemon"; !strings.Contains(out, expected) {
		c.Fatalf("expected %s received %s", expected, out)
	}

}

// testing #1405 - config.Cmd does not get cleaned up if
// utilizing cache
func (s *DockerSuite) TestBuildEntrypointRunCleanup(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildcmdcleanup"
	if _, err := buildImage(name,
		`FROM busybox
        RUN echo "hello"`,
		true); err != nil {
		c.Fatal(err)
	}

	ctx, err := fakeContext(`FROM busybox
        RUN echo "hello"
        ADD foo /foo
        ENTRYPOINT ["/bin/echo"]`,
		map[string]string{
			"foo": "hello",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Config.Cmd")
	if err != nil {
		c.Fatal(err)
	}
	// Cmd must be cleaned up
	if res != "<nil>" {
		c.Fatalf("Cmd %s, expected nil", res)
	}
}

func (s *DockerSuite) TestBuildInheritance(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildinheritance"

	_, err := buildImage(name,
		`FROM scratch
		EXPOSE 2375`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	ports1, err := inspectField(name, "Config.ExposedPorts")
	if err != nil {
		c.Fatal(err)
	}

	_, err = buildImage(name,
		fmt.Sprintf(`FROM %s
		ENTRYPOINT ["/bin/echo"]`, name),
		true)
	if err != nil {
		c.Fatal(err)
	}

	res, err := inspectField(name, "Config.Entrypoint")
	if err != nil {
		c.Fatal(err)
	}
	if expected := "{[/bin/echo]}"; res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}
	ports2, err := inspectField(name, "Config.ExposedPorts")
	if err != nil {
		c.Fatal(err)
	}
	if ports1 != ports2 {
		c.Fatalf("Ports must be same: %s != %s", ports1, ports2)
	}
}

func (s *DockerSuite) TestBuildFails(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildfails"
	_, err := buildImage(name,
		`FROM busybox
		RUN sh -c "exit 23"`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "returned a non-zero code: 23") {
			c.Fatalf("Wrong error %v, must be about non-zero code 23", err)
		}
	} else {
		c.Fatal("Error must not be nil")
	}
}

func (s *DockerSuite) TestBuildFailsDockerfileEmpty(c *check.C) {
	name := "testbuildfails"
	_, err := buildImage(name, ``, true)
	if err != nil {
		if !strings.Contains(err.Error(), "The Dockerfile (Dockerfile) cannot be empty") {
			c.Fatalf("Wrong error %v, must be about empty Dockerfile", err)
		}
	} else {
		c.Fatal("Error must not be nil")
	}
}

func (s *DockerSuite) TestBuildEscapeWhitespace(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildescaping"

	_, err := buildImage(name, `
  FROM busybox
  MAINTAINER "Docker \
IO <io@\
docker.com>"
  `, true)
	if err != nil {
		c.Fatal(err)
	}

	res, err := inspectField(name, "Author")

	if err != nil {
		c.Fatal(err)
	}

	if res != "\"Docker IO <io@docker.com>\"" {
		c.Fatalf("Parsed string did not match the escaped string. Got: %q", res)
	}

}

func (s *DockerSuite) TestBuildVerifyIntString(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Verify that strings that look like ints are still passed as strings
	name := "testbuildstringing"

	_, err := buildImage(name, `
  FROM busybox
  MAINTAINER 123
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "inspect", name)

	if !strings.Contains(out, "\"123\"") {
		c.Fatalf("Output does not contain the int as a string:\n%s", out)
	}

}

func (s *DockerSuite) TestBuildLineBreak(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildlinebreak"
	_, err := buildImage(name,
		`FROM  busybox
RUN    sh -c 'echo root:testpass \
	> /tmp/passwd'
RUN    mkdir -p /var/run/sshd
RUN    [ "$(cat /tmp/passwd)" = "root:testpass" ]
RUN    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildEOLInLine(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildeolinline"
	_, err := buildImage(name,
		`FROM   busybox
RUN    sh -c 'echo root:testpass > /tmp/passwd'
RUN    echo "foo \n bar"; echo "baz"
RUN    mkdir -p /var/run/sshd
RUN    [ "$(cat /tmp/passwd)" = "root:testpass" ]
RUN    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCommentsShebangs(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildcomments"
	_, err := buildImage(name,
		`FROM busybox
# This is an ordinary comment.
RUN { echo '#!/bin/sh'; echo 'echo hello world'; } > /hello.sh
RUN [ ! -x /hello.sh ]
# comment with line break \
RUN chmod +x /hello.sh
RUN [ -x /hello.sh ]
RUN [ "$(cat /hello.sh)" = $'#!/bin/sh\necho hello world' ]
RUN [ "$(/hello.sh)" = "hello world" ]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildUsersAndGroups(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildusers"
	_, err := buildImage(name,
		`FROM busybox

# Make sure our defaults work
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)" = '0:0/root:root' ]

# TODO decide if "args.user = strconv.Itoa(syscall.Getuid())" is acceptable behavior for changeUser in sysvinit instead of "return nil" when "USER" isn't specified (so that we get the proper group list even if that is the empty list, even in the default case of not supplying an explicit USER to run as, which implies USER 0)
USER root
RUN [ "$(id -G):$(id -Gn)" = '0 10:root wheel' ]

# Setup dockerio user and group
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group

# Make sure we can switch to our user and all the information is exactly as we expect it to be
USER dockerio
RUN id -G
RUN id -Gn
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]

# Switch back to root and double check that worked exactly as we might expect it to
USER root
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '0:0/root:root/0 10:root wheel' ]

# Add a "supplementary" group for our dockerio user
RUN echo 'supplementary:x:1002:dockerio' >> /etc/group

# ... and then go verify that we get it like we expect
USER dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001 1002:dockerio supplementary' ]
USER 1001
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001 1002:dockerio supplementary' ]

# super test the new "user:group" syntax
USER dockerio:dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]
USER 1001:dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]
USER dockerio:1001
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]
USER 1001:1001
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]
USER dockerio:supplementary
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1002/dockerio:supplementary/1002:supplementary' ]
USER dockerio:1002
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1002/dockerio:supplementary/1002:supplementary' ]
USER 1001:supplementary
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1002/dockerio:supplementary/1002:supplementary' ]
USER 1001:1002
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1002/dockerio:supplementary/1002:supplementary' ]

# make sure unknown uid/gid still works properly
USER 1042:1043
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1042:1043/1042:1043/1043:1043' ]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCleanupCmdOnEntrypoint(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildcmdcleanuponentrypoint"
	if _, err := buildImage(name,
		`FROM scratch
        CMD ["test"]
		ENTRYPOINT ["echo"]`,
		true); err != nil {
		c.Fatal(err)
	}
	if _, err := buildImage(name,
		fmt.Sprintf(`FROM %s
		ENTRYPOINT ["cat"]`, name),
		true); err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Config.Cmd")
	if err != nil {
		c.Fatal(err)
	}
	if res != "<nil>" {
		c.Fatalf("Cmd %s, expected nil", res)
	}

	res, err = inspectField(name, "Config.Entrypoint")
	if err != nil {
		c.Fatal(err)
	}
	if expected := "{[cat]}"; res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildClearCmd(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildclearcmd"
	_, err := buildImage(name,
		`From scratch
   ENTRYPOINT ["/bin/bash"]
   CMD []`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectFieldJSON(name, "Config.Cmd")
	if err != nil {
		c.Fatal(err)
	}
	if res != "[]" {
		c.Fatalf("Cmd %s, expected %s", res, "[]")
	}
}

func (s *DockerSuite) TestBuildEmptyCmd(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildemptycmd"
	if _, err := buildImage(name, "FROM scratch\nMAINTAINER quux\n", true); err != nil {
		c.Fatal(err)
	}
	res, err := inspectFieldJSON(name, "Config.Cmd")
	if err != nil {
		c.Fatal(err)
	}
	if res != "null" {
		c.Fatalf("Cmd %s, expected %s", res, "null")
	}
}

func (s *DockerSuite) TestBuildInvalidTag(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "abcd:" + stringutils.GenerateRandomAlphaOnlyString(200)
	_, out, err := buildImageWithOut(name, "FROM scratch\nMAINTAINER quux\n", true)
	// if the error doesnt check for illegal tag name, or the image is built
	// then this should fail
	if !strings.Contains(out, "invalid reference format") || strings.Contains(out, "Sending build context to Docker daemon") {
		c.Fatalf("failed to stop before building. Error: %s, Output: %s", err, out)
	}
}

func (s *DockerSuite) TestBuildCmdShDashC(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildcmdshc"
	if _, err := buildImage(name, "FROM busybox\nCMD echo cmd\n", true); err != nil {
		c.Fatal(err)
	}

	res, err := inspectFieldJSON(name, "Config.Cmd")
	if err != nil {
		c.Fatal(err, res)
	}

	expected := `["/bin/sh","-c","echo cmd"]`

	if res != expected {
		c.Fatalf("Expected value %s not in Config.Cmd: %s", expected, res)
	}

}

func (s *DockerSuite) TestBuildCmdSpaces(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Test to make sure that when we strcat arrays we take into account
	// the arg separator to make sure ["echo","hi"] and ["echo hi"] don't
	// look the same
	name := "testbuildcmdspaces"
	var id1 string
	var id2 string
	var err error

	if id1, err = buildImage(name, "FROM busybox\nCMD [\"echo hi\"]\n", true); err != nil {
		c.Fatal(err)
	}

	if id2, err = buildImage(name, "FROM busybox\nCMD [\"echo\", \"hi\"]\n", true); err != nil {
		c.Fatal(err)
	}

	if id1 == id2 {
		c.Fatal("Should not have resulted in the same CMD")
	}

	// Now do the same with ENTRYPOINT
	if id1, err = buildImage(name, "FROM busybox\nENTRYPOINT [\"echo hi\"]\n", true); err != nil {
		c.Fatal(err)
	}

	if id2, err = buildImage(name, "FROM busybox\nENTRYPOINT [\"echo\", \"hi\"]\n", true); err != nil {
		c.Fatal(err)
	}

	if id1 == id2 {
		c.Fatal("Should not have resulted in the same ENTRYPOINT")
	}

}

func (s *DockerSuite) TestBuildCmdJSONNoShDashC(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildcmdjson"
	if _, err := buildImage(name, "FROM busybox\nCMD [\"echo\", \"cmd\"]", true); err != nil {
		c.Fatal(err)
	}

	res, err := inspectFieldJSON(name, "Config.Cmd")
	if err != nil {
		c.Fatal(err, res)
	}

	expected := `["echo","cmd"]`

	if res != expected {
		c.Fatalf("Expected value %s not in Config.Cmd: %s", expected, res)
	}

}

func (s *DockerSuite) TestBuildErrorInvalidInstruction(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildignoreinvalidinstruction"

	out, _, err := buildImageWithOut(name, "FROM busybox\nfoo bar", true)
	if err == nil {
		c.Fatalf("Should have failed: %s", out)
	}

}

func (s *DockerSuite) TestBuildEntrypointInheritance(c *check.C) {
	testRequires(c, DaemonIsLinux)

	if _, err := buildImage("parent", `
    FROM busybox
    ENTRYPOINT exit 130
    `, true); err != nil {
		c.Fatal(err)
	}

	if _, status, _ := dockerCmdWithError("run", "parent"); status != 130 {
		c.Fatalf("expected exit code 130 but received %d", status)
	}

	if _, err := buildImage("child", `
    FROM parent
    ENTRYPOINT exit 5
    `, true); err != nil {
		c.Fatal(err)
	}

	if _, status, _ := dockerCmdWithError("run", "child"); status != 5 {
		c.Fatalf("expected exit code 5 but received %d", status)
	}

}

func (s *DockerSuite) TestBuildEntrypointInheritanceInspect(c *check.C) {
	testRequires(c, DaemonIsLinux)
	var (
		name     = "testbuildepinherit"
		name2    = "testbuildepinherit2"
		expected = `["/bin/sh","-c","echo quux"]`
	)

	if _, err := buildImage(name, "FROM busybox\nENTRYPOINT /foo/bar", true); err != nil {
		c.Fatal(err)
	}

	if _, err := buildImage(name2, fmt.Sprintf("FROM %s\nENTRYPOINT echo quux", name), true); err != nil {
		c.Fatal(err)
	}

	res, err := inspectFieldJSON(name2, "Config.Entrypoint")
	if err != nil {
		c.Fatal(err, res)
	}

	if res != expected {
		c.Fatalf("Expected value %s not in Config.Entrypoint: %s", expected, res)
	}

	out, _ := dockerCmd(c, "run", "-t", name2)

	expected = "quux"

	if strings.TrimSpace(out) != expected {
		c.Fatalf("Expected output is %s, got %s", expected, out)
	}

}

func (s *DockerSuite) TestBuildRunShEntrypoint(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildentrypoint"
	_, err := buildImage(name,
		`FROM busybox
                                ENTRYPOINT /bin/echo`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	dockerCmd(c, "run", "--rm", name)
}

func (s *DockerSuite) TestBuildExoticShellInterpolation(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildexoticshellinterpolation"

	_, err := buildImage(name, `
		FROM busybox

		ENV SOME_VAR a.b.c

		RUN [ "$SOME_VAR"       = 'a.b.c' ]
		RUN [ "${SOME_VAR}"     = 'a.b.c' ]
		RUN [ "${SOME_VAR%.*}"  = 'a.b'   ]
		RUN [ "${SOME_VAR%%.*}" = 'a'     ]
		RUN [ "${SOME_VAR#*.}"  = 'b.c'   ]
		RUN [ "${SOME_VAR##*.}" = 'c'     ]
		RUN [ "${SOME_VAR/c/d}" = 'a.b.d' ]
		RUN [ "${#SOME_VAR}"    = '5'     ]

		RUN [ "${SOME_UNSET_VAR:-$SOME_VAR}" = 'a.b.c' ]
		RUN [ "${SOME_VAR:+Version: ${SOME_VAR}}" = 'Version: a.b.c' ]
		RUN [ "${SOME_UNSET_VAR:+${SOME_VAR}}" = '' ]
		RUN [ "${SOME_UNSET_VAR:-${SOME_VAR:-d.e.f}}" = 'a.b.c' ]
	`, false)
	if err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestBuildVerifySingleQuoteFails(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// This testcase is supposed to generate an error because the
	// JSON array we're passing in on the CMD uses single quotes instead
	// of double quotes (per the JSON spec). This means we interpret it
	// as a "string" insead of "JSON array" and pass it on to "sh -c" and
	// it should barf on it.
	name := "testbuildsinglequotefails"

	if _, err := buildImage(name,
		`FROM busybox
		CMD [ '/bin/sh', '-c', 'echo hi' ]`,
		true); err != nil {
		c.Fatal(err)
	}

	if _, _, err := dockerCmdWithError("run", "--rm", name); err == nil {
		c.Fatal("The image was not supposed to be able to run")
	}

}

func (s *DockerSuite) TestBuildVerboseOut(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildverboseout"

	_, out, err := buildImageWithOut(name,
		`FROM busybox
RUN echo 123`,
		false)

	if err != nil {
		c.Fatal(err)
	}
	if !strings.Contains(out, "\n123\n") {
		c.Fatalf("Output should contain %q: %q", "123", out)
	}

}

func (s *DockerSuite) TestBuildWithTabs(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildwithtabs"
	_, err := buildImage(name,
		"FROM busybox\nRUN echo\tone\t\ttwo", true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectFieldJSON(name, "ContainerConfig.Cmd")
	if err != nil {
		c.Fatal(err)
	}
	expected1 := `["/bin/sh","-c","echo\tone\t\ttwo"]`
	expected2 := `["/bin/sh","-c","echo\u0009one\u0009\u0009two"]` // syntactically equivalent, and what Go 1.3 generates
	if res != expected1 && res != expected2 {
		c.Fatalf("Missing tabs.\nGot: %s\nExp: %s or %s", res, expected1, expected2)
	}
}

func (s *DockerSuite) TestBuildLabels(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildlabel"
	expected := `{"License":"GPL","Vendor":"Acme"}`
	_, err := buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme
                LABEL License GPL`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectFieldJSON(name, "Config.Labels")
	if err != nil {
		c.Fatal(err)
	}
	if res != expected {
		c.Fatalf("Labels %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildLabelsCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildlabelcache"

	id1, err := buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme`, false)
	if err != nil {
		c.Fatalf("Build 1 should have worked: %v", err)
	}

	id2, err := buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme`, true)
	if err != nil || id1 != id2 {
		c.Fatalf("Build 2 should have worked & used cache(%s,%s): %v", id1, id2, err)
	}

	id2, err = buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme1`, true)
	if err != nil || id1 == id2 {
		c.Fatalf("Build 3 should have worked & NOT used cache(%s,%s): %v", id1, id2, err)
	}

	id2, err = buildImage(name,
		`FROM busybox
		LABEL Vendor Acme`, true) // Note: " " and "=" should be same
	if err != nil || id1 != id2 {
		c.Fatalf("Build 4 should have worked & used cache(%s,%s): %v", id1, id2, err)
	}

	// Now make sure the cache isn't used by mistake
	id1, err = buildImage(name,
		`FROM busybox
       LABEL f1=b1 f2=b2`, false)
	if err != nil {
		c.Fatalf("Build 5 should have worked: %q", err)
	}

	id2, err = buildImage(name,
		`FROM busybox
       LABEL f1="b1 f2=b2"`, true)
	if err != nil || id1 == id2 {
		c.Fatalf("Build 6 should have worked & NOT used the cache(%s,%s): %q", id1, id2, err)
	}

}

func (s *DockerSuite) TestBuildStderr(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// This test just makes sure that no non-error output goes
	// to stderr
	name := "testbuildstderr"
	_, _, stderr, err := buildImageWithStdoutStderr(name,
		"FROM busybox\nRUN echo one", true)
	if err != nil {
		c.Fatal(err)
	}

	if runtime.GOOS == "windows" {
		// stderr might contain a security warning on windows
		lines := strings.Split(stderr, "\n")
		for _, v := range lines {
			if v != "" && !strings.Contains(v, "SECURITY WARNING:") {
				c.Fatalf("Stderr contains unexpected output line: %q", v)
			}
		}
	} else {
		if stderr != "" {
			c.Fatalf("Stderr should have been empty, instead its: %q", stderr)
		}
	}
}

func (s *DockerSuite) TestBuildChownSingleFile(c *check.C) {
	testRequires(c, UnixCli) // test uses chown: not available on windows
	testRequires(c, DaemonIsLinux)

	name := "testbuildchownsinglefile"

	ctx, err := fakeContext(`
FROM busybox
COPY test /
RUN ls -l /test
RUN [ $(ls -l /test | awk '{print $3":"$4}') = 'root:root' ]
`, map[string]string{
		"test": "test",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if err := os.Chown(filepath.Join(ctx.Dir, "test"), 4242, 4242); err != nil {
		c.Fatal(err)
	}

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestBuildXZHost(c *check.C) {
	// /usr/local/sbin/xz gets permission denied for the user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	name := "testbuildxzhost"

	ctx, err := fakeContext(`
FROM busybox
ADD xz /usr/local/sbin/
RUN chmod 755 /usr/local/sbin/xz
ADD test.xz /
RUN [ ! -e /injected ]`,
		map[string]string{
			"test.xz": "\xfd\x37\x7a\x58\x5a\x00\x00\x04\xe6\xd6\xb4\x46\x02\x00" +
				"\x21\x01\x16\x00\x00\x00\x74\x2f\xe5\xa3\x01\x00\x3f\xfd" +
				"\x37\x7a\x58\x5a\x00\x00\x04\xe6\xd6\xb4\x46\x02\x00\x21",
			"xz": "#!/bin/sh\ntouch /injected",
		})

	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestBuildVolumesRetainContents(c *check.C) {
	// /foo/file gets permission denied for the user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	var (
		name     = "testbuildvolumescontent"
		expected = "some text"
	)
	ctx, err := fakeContext(`
FROM busybox
COPY content /foo/file
VOLUME /foo
CMD cat /foo/file`,
		map[string]string{
			"content": expected,
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, false); err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "--rm", name)
	if out != expected {
		c.Fatalf("expected file contents for /foo/file to be %q but received %q", expected, out)
	}

}

func (s *DockerSuite) TestBuildRenamedDockerfile(c *check.C) {
	testRequires(c, DaemonIsLinux)

	ctx, err := fakeContext(`FROM busybox
	RUN echo from Dockerfile`,
		map[string]string{
			"Dockerfile":       "FROM busybox\nRUN echo from Dockerfile",
			"files/Dockerfile": "FROM busybox\nRUN echo from files/Dockerfile",
			"files/dFile":      "FROM busybox\nRUN echo from files/dFile",
			"dFile":            "FROM busybox\nRUN echo from dFile",
			"files/dFile2":     "FROM busybox\nRUN echo from files/dFile2",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-t", "test1", ".")
	if err != nil {
		c.Fatalf("Failed to build: %s\n%s", out, err)
	}
	if !strings.Contains(out, "from Dockerfile") {
		c.Fatalf("test1 should have used Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, ctx.Dir, "build", "-f", filepath.Join("files", "Dockerfile"), "-t", "test2", ".")
	if err != nil {
		c.Fatal(err)
	}
	if !strings.Contains(out, "from files/Dockerfile") {
		c.Fatalf("test2 should have used files/Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, ctx.Dir, "build", fmt.Sprintf("--file=%s", filepath.Join("files", "dFile")), "-t", "test3", ".")
	if err != nil {
		c.Fatal(err)
	}
	if !strings.Contains(out, "from files/dFile") {
		c.Fatalf("test3 should have used files/dFile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, ctx.Dir, "build", "--file=dFile", "-t", "test4", ".")
	if err != nil {
		c.Fatal(err)
	}
	if !strings.Contains(out, "from dFile") {
		c.Fatalf("test4 should have used dFile, output:%s", out)
	}

	dirWithNoDockerfile, err := ioutil.TempDir(os.TempDir(), "test5")
	c.Assert(err, check.IsNil)
	nonDockerfileFile := filepath.Join(dirWithNoDockerfile, "notDockerfile")
	if _, err = os.Create(nonDockerfileFile); err != nil {
		c.Fatal(err)
	}
	out, _, err = dockerCmdInDir(c, ctx.Dir, "build", fmt.Sprintf("--file=%s", nonDockerfileFile), "-t", "test5", ".")

	if err == nil {
		c.Fatalf("test5 was supposed to fail to find passwd")
	}

	if expected := fmt.Sprintf("The Dockerfile (%s) must be within the build context (.)", nonDockerfileFile); !strings.Contains(out, expected) {
		c.Fatalf("wrong error messsage:%v\nexpected to contain=%v", out, expected)
	}

	out, _, err = dockerCmdInDir(c, filepath.Join(ctx.Dir, "files"), "build", "-f", filepath.Join("..", "Dockerfile"), "-t", "test6", "..")
	if err != nil {
		c.Fatalf("test6 failed: %s", err)
	}
	if !strings.Contains(out, "from Dockerfile") {
		c.Fatalf("test6 should have used root Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, filepath.Join(ctx.Dir, "files"), "build", "-f", filepath.Join(ctx.Dir, "files", "Dockerfile"), "-t", "test7", "..")
	if err != nil {
		c.Fatalf("test7 failed: %s", err)
	}
	if !strings.Contains(out, "from files/Dockerfile") {
		c.Fatalf("test7 should have used files Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, filepath.Join(ctx.Dir, "files"), "build", "-f", filepath.Join("..", "Dockerfile"), "-t", "test8", ".")
	if err == nil || !strings.Contains(out, "must be within the build context") {
		c.Fatalf("test8 should have failed with Dockerfile out of context: %s", err)
	}

	tmpDir := os.TempDir()
	out, _, err = dockerCmdInDir(c, tmpDir, "build", "-t", "test9", ctx.Dir)
	if err != nil {
		c.Fatalf("test9 - failed: %s", err)
	}
	if !strings.Contains(out, "from Dockerfile") {
		c.Fatalf("test9 should have used root Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, filepath.Join(ctx.Dir, "files"), "build", "-f", "dFile2", "-t", "test10", ".")
	if err != nil {
		c.Fatalf("test10 should have worked: %s", err)
	}
	if !strings.Contains(out, "from files/dFile2") {
		c.Fatalf("test10 should have used files/dFile2, output:%s", out)
	}

}

func (s *DockerSuite) TestBuildFromMixedcaseDockerfile(c *check.C) {
	testRequires(c, UnixCli) // Dockerfile overwrites dockerfile on windows
	testRequires(c, DaemonIsLinux)

	ctx, err := fakeContext(`FROM busybox
	RUN echo from dockerfile`,
		map[string]string{
			"dockerfile": "FROM busybox\nRUN echo from dockerfile",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-t", "test1", ".")
	if err != nil {
		c.Fatalf("Failed to build: %s\n%s", out, err)
	}

	if !strings.Contains(out, "from dockerfile") {
		c.Fatalf("Missing proper output: %s", out)
	}

}

func (s *DockerSuite) TestBuildWithTwoDockerfiles(c *check.C) {
	testRequires(c, UnixCli) // Dockerfile overwrites dockerfile on windows
	testRequires(c, DaemonIsLinux)

	ctx, err := fakeContext(`FROM busybox
RUN echo from Dockerfile`,
		map[string]string{
			"dockerfile": "FROM busybox\nRUN echo from dockerfile",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-t", "test1", ".")
	if err != nil {
		c.Fatalf("Failed to build: %s\n%s", out, err)
	}

	if !strings.Contains(out, "from Dockerfile") {
		c.Fatalf("Missing proper output: %s", out)
	}

}

func (s *DockerSuite) TestBuildFromURLWithF(c *check.C) {
	testRequires(c, DaemonIsLinux)

	server, err := fakeStorage(map[string]string{"baz": `FROM busybox
RUN echo from baz
COPY * /tmp/
RUN find /tmp/`})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(`FROM busybox
RUN echo from Dockerfile`,
		map[string]string{})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	// Make sure that -f is ignored and that we don't use the Dockerfile
	// that's in the current dir
	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-f", "baz", "-t", "test1", server.URL()+"/baz")
	if err != nil {
		c.Fatalf("Failed to build: %s\n%s", out, err)
	}

	if !strings.Contains(out, "from baz") ||
		strings.Contains(out, "/tmp/baz") ||
		!strings.Contains(out, "/tmp/Dockerfile") {
		c.Fatalf("Missing proper output: %s", out)
	}

}

func (s *DockerSuite) TestBuildFromStdinWithF(c *check.C) {
	testRequires(c, DaemonIsLinux)
	ctx, err := fakeContext(`FROM busybox
RUN echo from Dockerfile`,
		map[string]string{})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	// Make sure that -f is ignored and that we don't use the Dockerfile
	// that's in the current dir
	dockerCommand := exec.Command(dockerBinary, "build", "-f", "baz", "-t", "test1", "-")
	dockerCommand.Dir = ctx.Dir
	dockerCommand.Stdin = strings.NewReader(`FROM busybox
RUN echo from baz
COPY * /tmp/
RUN find /tmp/`)
	out, status, err := runCommandWithOutput(dockerCommand)
	if err != nil || status != 0 {
		c.Fatalf("Error building: %s", err)
	}

	if !strings.Contains(out, "from baz") ||
		strings.Contains(out, "/tmp/baz") ||
		!strings.Contains(out, "/tmp/Dockerfile") {
		c.Fatalf("Missing proper output: %s", out)
	}

}

func (s *DockerSuite) TestBuildFromOfficialNames(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildfromofficial"
	fromNames := []string{
		"busybox",
		"docker.io/busybox",
		"index.docker.io/busybox",
		"library/busybox",
		"docker.io/library/busybox",
		"index.docker.io/library/busybox",
	}
	for idx, fromName := range fromNames {
		imgName := fmt.Sprintf("%s%d", name, idx)
		_, err := buildImage(imgName, "FROM "+fromName, true)
		if err != nil {
			c.Errorf("Build failed using FROM %s: %s", fromName, err)
		}
		deleteImages(imgName)
	}
}

func (s *DockerSuite) TestBuildSpaces(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Test to make sure that leading/trailing spaces on a command
	// doesn't change the error msg we get
	var (
		err1 error
		err2 error
	)

	name := "testspaces"
	ctx, err := fakeContext("FROM busybox\nCOPY\n",
		map[string]string{
			"Dockerfile": "FROM busybox\nCOPY\n",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err1 = buildImageFromContext(name, ctx, false); err1 == nil {
		c.Fatal("Build 1 was supposed to fail, but didn't")
	}

	ctx.Add("Dockerfile", "FROM busybox\nCOPY    ")
	if _, err2 = buildImageFromContext(name, ctx, false); err2 == nil {
		c.Fatal("Build 2 was supposed to fail, but didn't")
	}

	removeLogTimestamps := func(s string) string {
		return regexp.MustCompile(`time="(.*?)"`).ReplaceAllString(s, `time=[TIMESTAMP]`)
	}

	// Skip over the times
	e1 := removeLogTimestamps(err1.Error())
	e2 := removeLogTimestamps(err2.Error())

	// Ignore whitespace since that's what were verifying doesn't change stuff
	if strings.Replace(e1, " ", "", -1) != strings.Replace(e2, " ", "", -1) {
		c.Fatalf("Build 2's error wasn't the same as build 1's\n1:%s\n2:%s", err1, err2)
	}

	ctx.Add("Dockerfile", "FROM busybox\n   COPY")
	if _, err2 = buildImageFromContext(name, ctx, false); err2 == nil {
		c.Fatal("Build 3 was supposed to fail, but didn't")
	}

	// Skip over the times
	e1 = removeLogTimestamps(err1.Error())
	e2 = removeLogTimestamps(err2.Error())

	// Ignore whitespace since that's what were verifying doesn't change stuff
	if strings.Replace(e1, " ", "", -1) != strings.Replace(e2, " ", "", -1) {
		c.Fatalf("Build 3's error wasn't the same as build 1's\n1:%s\n3:%s", err1, err2)
	}

	ctx.Add("Dockerfile", "FROM busybox\n   COPY    ")
	if _, err2 = buildImageFromContext(name, ctx, false); err2 == nil {
		c.Fatal("Build 4 was supposed to fail, but didn't")
	}

	// Skip over the times
	e1 = removeLogTimestamps(err1.Error())
	e2 = removeLogTimestamps(err2.Error())

	// Ignore whitespace since that's what were verifying doesn't change stuff
	if strings.Replace(e1, " ", "", -1) != strings.Replace(e2, " ", "", -1) {
		c.Fatalf("Build 4's error wasn't the same as build 1's\n1:%s\n4:%s", err1, err2)
	}

}

func (s *DockerSuite) TestBuildSpacesWithQuotes(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Test to make sure that spaces in quotes aren't lost
	name := "testspacesquotes"

	dockerfile := `FROM busybox
RUN echo "  \
  foo  "`

	_, out, err := buildImageWithOut(name, dockerfile, false)
	if err != nil {
		c.Fatal("Build failed:", err)
	}

	expecting := "\n    foo  \n"
	if !strings.Contains(out, expecting) {
		c.Fatalf("Bad output: %q expecting to contain %q", out, expecting)
	}

}

// #4393
func (s *DockerSuite) TestBuildVolumeFileExistsinContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	buildCmd := exec.Command(dockerBinary, "build", "-t", "docker-test-errcreatevolumewithfile", "-")
	buildCmd.Stdin = strings.NewReader(`
	FROM busybox
	RUN touch /foo
	VOLUME /foo
	`)

	out, _, err := runCommandWithOutput(buildCmd)
	if err == nil || !strings.Contains(out, "file exists") {
		c.Fatalf("expected build to fail when file exists in container at requested volume path")
	}

}

func (s *DockerSuite) TestBuildMissingArgs(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Test to make sure that all Dockerfile commands (except the ones listed
	// in skipCmds) will generate an error if no args are provided.
	// Note: INSERT is deprecated so we exclude it because of that.
	skipCmds := map[string]struct{}{
		"CMD":        {},
		"RUN":        {},
		"ENTRYPOINT": {},
		"INSERT":     {},
	}

	for cmd := range command.Commands {
		cmd = strings.ToUpper(cmd)
		if _, ok := skipCmds[cmd]; ok {
			continue
		}

		var dockerfile string
		if cmd == "FROM" {
			dockerfile = cmd
		} else {
			// Add FROM to make sure we don't complain about it missing
			dockerfile = "FROM busybox\n" + cmd
		}

		ctx, err := fakeContext(dockerfile, map[string]string{})
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()
		var out string
		if out, err = buildImageFromContext("args", ctx, true); err == nil {
			c.Fatalf("%s was supposed to fail. Out:%s", cmd, out)
		}
		if !strings.Contains(err.Error(), cmd+" requires") {
			c.Fatalf("%s returned the wrong type of error:%s", cmd, err)
		}
	}

}

func (s *DockerSuite) TestBuildEmptyScratch(c *check.C) {
	testRequires(c, DaemonIsLinux)
	_, out, err := buildImageWithOut("sc", "FROM scratch", true)
	if err == nil {
		c.Fatalf("Build was supposed to fail")
	}
	if !strings.Contains(out, "No image was generated") {
		c.Fatalf("Wrong error message: %v", out)
	}
}

func (s *DockerSuite) TestBuildDotDotFile(c *check.C) {
	testRequires(c, DaemonIsLinux)

	ctx, err := fakeContext("FROM busybox\n",
		map[string]string{
			"..gitme": "",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err = buildImageFromContext("sc", ctx, false); err != nil {
		c.Fatalf("Build was supposed to work: %s", err)
	}
}

func (s *DockerSuite) TestBuildNotVerbose(c *check.C) {
	testRequires(c, DaemonIsLinux)
	ctx, err := fakeContext("FROM busybox\nENV abc=hi\nRUN echo $abc there", map[string]string{})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	// First do it w/verbose - baseline
	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "--no-cache", "-t", "verbose", ".")
	if err != nil {
		c.Fatalf("failed to build the image w/o -q: %s, %v", out, err)
	}
	if !strings.Contains(out, "hi there") {
		c.Fatalf("missing output:%s\n", out)
	}

	// Now do it w/o verbose
	out, _, err = dockerCmdInDir(c, ctx.Dir, "build", "--no-cache", "-q", "-t", "verbose", ".")
	if err != nil {
		c.Fatalf("failed to build the image w/ -q: %s, %v", out, err)
	}
	if strings.Contains(out, "hi there") {
		c.Fatalf("Bad output, should not contain 'hi there':%s", out)
	}

}

func (s *DockerSuite) TestBuildRUNoneJSON(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildrunonejson"

	ctx, err := fakeContext(`FROM hello-world:frozen
RUN [ "/hello" ]`, map[string]string{})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "--no-cache", "-t", name, ".")
	if err != nil {
		c.Fatalf("failed to build the image: %s, %v", out, err)
	}

	if !strings.Contains(out, "Hello from Docker") {
		c.Fatalf("bad output: %s", out)
	}

}

func (s *DockerSuite) TestBuildEmptyStringVolume(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildemptystringvolume"

	_, err := buildImage(name, `
  FROM busybox
  ENV foo=""
  VOLUME $foo
  `, false)
	if err == nil {
		c.Fatal("Should have failed to build")
	}

}

func (s *DockerSuite) TestBuildContainerWithCgroupParent(c *check.C) {
	testRequires(c, SameHostDaemon)
	testRequires(c, DaemonIsLinux)

	cgroupParent := "test"
	data, err := ioutil.ReadFile("/proc/self/cgroup")
	if err != nil {
		c.Fatalf("failed to read '/proc/self/cgroup - %v", err)
	}
	selfCgroupPaths := parseCgroupPaths(string(data))
	_, found := selfCgroupPaths["memory"]
	if !found {
		c.Fatalf("unable to find self memory cgroup path. CgroupsPath: %v", selfCgroupPaths)
	}
	cmd := exec.Command(dockerBinary, "build", "--cgroup-parent", cgroupParent, "-")
	cmd.Stdin = strings.NewReader(`
FROM busybox
RUN cat /proc/self/cgroup
`)

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("unexpected failure when running container with --cgroup-parent option - %s\n%v", string(out), err)
	}
	m, err := regexp.MatchString(fmt.Sprintf("memory:.*/%s/.*", cgroupParent), out)
	c.Assert(err, check.IsNil)
	if !m {
		c.Fatalf("There is no expected memory cgroup with parent /%s/: %s", cgroupParent, out)
	}
}

func (s *DockerSuite) TestBuildNoDupOutput(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Check to make sure our build output prints the Dockerfile cmd
	// property - there was a bug that caused it to be duplicated on the
	// Step X  line
	name := "testbuildnodupoutput"

	_, out, err := buildImageWithOut(name, `
  FROM busybox
  RUN env`, false)
	if err != nil {
		c.Fatalf("Build should have worked: %q", err)
	}

	exp := "\nStep 2 : RUN env\n"
	if !strings.Contains(out, exp) {
		c.Fatalf("Bad output\nGot:%s\n\nExpected to contain:%s\n", out, exp)
	}
}

// GH15826
func (s *DockerSuite) TestBuildStartsFromOne(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Explicit check to ensure that build starts from step 1 rather than 0
	name := "testbuildstartsfromone"

	_, out, err := buildImageWithOut(name, `
  FROM busybox`, false)
	if err != nil {
		c.Fatalf("Build should have worked: %q", err)
	}

	exp := "\nStep 1 : FROM busybox\n"
	if !strings.Contains(out, exp) {
		c.Fatalf("Bad output\nGot:%s\n\nExpected to contain:%s\n", out, exp)
	}
}

func (s *DockerSuite) TestBuildBadCmdFlag(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildbadcmdflag"

	_, out, err := buildImageWithOut(name, `
  FROM busybox
  MAINTAINER --boo joe@example.com`, false)
	if err == nil {
		c.Fatal("Build should have failed")
	}

	exp := "\nUnknown flag: boo\n"
	if !strings.Contains(out, exp) {
		c.Fatalf("Bad output\nGot:%s\n\nExpected to contain:%s\n", out, exp)
	}
}

func (s *DockerSuite) TestBuildRUNErrMsg(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Test to make sure the bad command is quoted with just "s and
	// not as a Go []string
	name := "testbuildbadrunerrmsg"
	_, out, err := buildImageWithOut(name, `
  FROM busybox
  RUN badEXE a1 \& a2	a3`, false) // tab between a2 and a3
	if err == nil {
		c.Fatal("Should have failed to build")
	}

	exp := `The command '/bin/sh -c badEXE a1 \& a2	a3' returned a non-zero code: 127`
	if !strings.Contains(out, exp) {
		c.Fatalf("RUN doesn't have the correct output:\nGot:%s\nExpected:%s", out, exp)
	}
}

func (s *DockerTrustSuite) TestTrustedBuild(c *check.C) {
	repoName := s.setupTrustedImage(c, "trusted-build")
	dockerFile := fmt.Sprintf(`
  FROM %s
  RUN []
    `, repoName)

	name := "testtrustedbuild"

	buildCmd := buildImageCmd(name, dockerFile, true)
	s.trustedCmd(buildCmd)
	out, _, err := runCommandWithOutput(buildCmd)
	if err != nil {
		c.Fatalf("Error running trusted build: %s\n%s", err, out)
	}

	if !strings.Contains(out, fmt.Sprintf("FROM %s@sha", repoName[:len(repoName)-7])) {
		c.Fatalf("Unexpected output on trusted build:\n%s", out)
	}

	// We should also have a tag reference for the image.
	if out, exitCode := dockerCmd(c, "inspect", repoName); exitCode != 0 {
		c.Fatalf("unexpected exit code inspecting image %q: %d: %s", repoName, exitCode, out)
	}

	// We should now be able to remove the tag reference.
	if out, exitCode := dockerCmd(c, "rmi", repoName); exitCode != 0 {
		c.Fatalf("unexpected exit code inspecting image %q: %d: %s", repoName, exitCode, out)
	}
}

func (s *DockerTrustSuite) TestTrustedBuildUntrustedTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/build-untrusted-tag:latest", privateRegistryURL)
	dockerFile := fmt.Sprintf(`
  FROM %s
  RUN []
    `, repoName)

	name := "testtrustedbuilduntrustedtag"

	buildCmd := buildImageCmd(name, dockerFile, true)
	s.trustedCmd(buildCmd)
	out, _, err := runCommandWithOutput(buildCmd)
	if err == nil {
		c.Fatalf("Expected error on trusted build with untrusted tag: %s\n%s", err, out)
	}

	if !strings.Contains(out, fmt.Sprintf("no trust data available")) {
		c.Fatalf("Unexpected output on trusted build with untrusted tag:\n%s", out)
	}
}

// Issue #15634: COPY fails when path starts with "null"
func (s *DockerSuite) TestBuildNullStringInAddCopyVolume(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildnullstringinaddcopyvolume"

	ctx, err := fakeContext(`
		FROM busybox

		ADD null /
		COPY nullfile /
		VOLUME nullvolume
		`,
		map[string]string{
			"null":     "test1",
			"nullfile": "test2",
		},
	)
	c.Assert(err, check.IsNil)
	defer ctx.Close()

	_, err = buildImageFromContext(name, ctx, true)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestBuildStopSignal(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "test_build_stop_signal"
	_, err := buildImage(name,
		`FROM busybox
		 STOPSIGNAL SIGKILL`,
		true)
	c.Assert(err, check.IsNil)
	res, err := inspectFieldJSON(name, "Config.StopSignal")
	c.Assert(err, check.IsNil)

	if res != `"SIGKILL"` {
		c.Fatalf("Signal %s, expected SIGKILL", res)
	}
}

func (s *DockerSuite) TestBuildNoNamedVolume(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-v", "testname:/foo", "busybox", "sh", "-c", "touch /foo/oops")

	dockerFile := `FROM busybox
	VOLUME testname:/foo
	RUN ls /foo/oops
	`
	_, err := buildImage("test", dockerFile, false)
	c.Assert(err, check.NotNil, check.Commentf("image build should have failed"))
}

func (s *DockerSuite) TestBuildTagEvent(c *check.C) {
	testRequires(c, DaemonIsLinux)
	resp, rc, err := sockRequestRaw("GET", `/events?filters={"event":["tag"]}`, nil, "application/json")
	c.Assert(err, check.IsNil)
	defer rc.Close()
	c.Assert(resp.StatusCode, check.Equals, http.StatusOK)

	type event struct {
		Status string `json:"status"`
		ID     string `json:"id"`
	}
	ch := make(chan event)
	go func() {
		ev := event{}
		if err := json.NewDecoder(rc).Decode(&ev); err == nil {
			ch <- ev
		}
	}()

	dockerFile := `FROM busybox
	RUN echo events
	`
	_, err = buildImage("test", dockerFile, false)
	c.Assert(err, check.IsNil)

	select {
	case ev := <-ch:
		c.Assert(ev.Status, check.Equals, "tag")
		c.Assert(ev.ID, check.Equals, "test:latest")
	case <-time.After(time.Second):
		c.Fatal("The 'tag' event not heard from the server")
	}
}

// #15780
func (s *DockerSuite) TestBuildMultipleTags(c *check.C) {
	dockerfile := `
	FROM busybox
	MAINTAINER test-15780
	`
	cmd := exec.Command(dockerBinary, "build", "-t", "tag1", "-t", "tag2:v2",
		"-t", "tag1:latest", "-t", "tag1", "--no-cache", "-")
	cmd.Stdin = strings.NewReader(dockerfile)
	_, err := runCommand(cmd)
	c.Assert(err, check.IsNil)

	id1, err := getIDByName("tag1")
	c.Assert(err, check.IsNil)
	id2, err := getIDByName("tag2:v2")
	c.Assert(err, check.IsNil)
	c.Assert(id1, check.Equals, id2)
}

// #17827
func (s *DockerSuite) TestBuildCacheRootSource(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildrootsource"
	ctx, err := fakeContext(`
	FROM busybox
	COPY / /data`,
		map[string]string{
			"foo": "bar",
		})
	c.Assert(err, checker.IsNil)
	defer ctx.Close()

	// warm up cache
	_, err = buildImageFromContext(name, ctx, true)
	c.Assert(err, checker.IsNil)

	// change file, should invalidate cache
	err = ioutil.WriteFile(filepath.Join(ctx.Dir, "foo"), []byte("baz"), 0644)
	c.Assert(err, checker.IsNil)

	_, out, err := buildImageFromContextWithOut(name, ctx, true)
	c.Assert(err, checker.IsNil)

	c.Assert(out, checker.Not(checker.Contains), "Using cache")
}
