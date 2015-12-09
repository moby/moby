package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildOnBuildLowercase(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildonbuildlowercase"
	name2 := "testbuildonbuildlowercase2"

	_, err := buildImage(name,
		`
  FROM busybox
  onbuild run echo quux
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	_, out, err := buildImageWithOut(name2, fmt.Sprintf(`
  FROM %s
  `, name), true)

	if err != nil {
		c.Fatal(err)
	}

	if !strings.Contains(out, "quux") {
		c.Fatalf("Did not receive the expected echo text, got %s", out)
	}

	if strings.Contains(out, "ONBUILD ONBUILD") {
		c.Fatalf("Got an ONBUILD ONBUILD error with no error: got %s", out)
	}

}

func (s *DockerSuite) TestBuildOnBuildForbiddenMaintainerInSourceImage(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildonbuildforbiddenmaintainerinsourceimage"

	out, _ := dockerCmd(c, "create", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)

	dockerCmd(c, "commit", "--run", "{\"OnBuild\":[\"MAINTAINER docker.io\"]}", cleanedContainerID, "onbuild")

	_, err := buildImage(name,
		`FROM onbuild`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "maintainer isn't allowed as an ONBUILD trigger") {
			c.Fatalf("Wrong error %v, must be about MAINTAINER and ONBUILD in source image", err)
		}
	} else {
		c.Fatal("Error must not be nil")
	}

}

func (s *DockerSuite) TestBuildOnBuildForbiddenChainedInSourceImage(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildonbuildforbiddenchainedinsourceimage"

	out, _ := dockerCmd(c, "create", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)

	dockerCmd(c, "commit", "--run", "{\"OnBuild\":[\"ONBUILD RUN ls\"]}", cleanedContainerID, "onbuild")

	_, err := buildImage(name,
		`FROM onbuild`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed") {
			c.Fatalf("Wrong error %v, must be about chaining ONBUILD in source image", err)
		}
	} else {
		c.Fatal("Error must not be nil")
	}

}

func (s *DockerSuite) TestBuildOnBuildCmdEntrypointJSON(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name1 := "onbuildcmd"
	name2 := "onbuildgenerated"

	_, err := buildImage(name1, `
FROM busybox
ONBUILD CMD ["hello world"]
ONBUILD ENTRYPOINT ["echo"]
ONBUILD RUN ["true"]`,
		false)

	if err != nil {
		c.Fatal(err)
	}

	_, err = buildImage(name2, fmt.Sprintf(`FROM %s`, name1), false)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "-t", name2)

	if !regexp.MustCompile(`(?m)^hello world`).MatchString(out) {
		c.Fatal("did not get echo output from onbuild", out)
	}

}

func (s *DockerSuite) TestBuildOnBuildEntrypointJSON(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name1 := "onbuildcmd"
	name2 := "onbuildgenerated"

	_, err := buildImage(name1, `
FROM busybox
ONBUILD ENTRYPOINT ["echo"]`,
		false)

	if err != nil {
		c.Fatal(err)
	}

	_, err = buildImage(name2, fmt.Sprintf("FROM %s\nCMD [\"hello world\"]\n", name1), false)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "-t", name2)

	if !regexp.MustCompile(`(?m)^hello world`).MatchString(out) {
		c.Fatal("got malformed output from onbuild", out)
	}

}

// #6445 ensure ONBUILD triggers aren't committed to grandchildren
func (s *DockerSuite) TestBuildOnBuildLimitedInheritence(c *check.C) {
	testRequires(c, DaemonIsLinux)
	var (
		out2, out3 string
	)
	{
		name1 := "testonbuildtrigger1"
		dockerfile1 := `
		FROM busybox
		RUN echo "GRANDPARENT"
		ONBUILD RUN echo "ONBUILD PARENT"
		`
		ctx, err := fakeContext(dockerfile1, nil)
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()

		out1, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-t", name1, ".")
		if err != nil {
			c.Fatalf("build failed to complete: %s, %v", out1, err)
		}
	}
	{
		name2 := "testonbuildtrigger2"
		dockerfile2 := `
		FROM testonbuildtrigger1
		`
		ctx, err := fakeContext(dockerfile2, nil)
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()

		out2, _, err = dockerCmdInDir(c, ctx.Dir, "build", "-t", name2, ".")
		if err != nil {
			c.Fatalf("build failed to complete: %s, %v", out2, err)
		}
	}
	{
		name3 := "testonbuildtrigger3"
		dockerfile3 := `
		FROM testonbuildtrigger2
		`
		ctx, err := fakeContext(dockerfile3, nil)
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()

		out3, _, err = dockerCmdInDir(c, ctx.Dir, "build", "-t", name3, ".")
		if err != nil {
			c.Fatalf("build failed to complete: %s, %v", out3, err)
		}

	}

	// ONBUILD should be run in second build.
	if !strings.Contains(out2, "ONBUILD PARENT") {
		c.Fatalf("ONBUILD instruction did not run in child of ONBUILD parent")
	}

	// ONBUILD should *not* be run in third build.
	if strings.Contains(out3, "ONBUILD PARENT") {
		c.Fatalf("ONBUILD instruction ran in grandchild of ONBUILD parent")
	}

}

func (s *DockerSuite) TestBuildOnBuild(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildonbuild"
	_, err := buildImage(name,
		`FROM busybox
		ONBUILD RUN touch foobar`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	_, err = buildImage(name,
		fmt.Sprintf(`FROM %s
		RUN [ -f foobar ]`, name),
		true)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildOnBuildForbiddenChained(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildonbuildforbiddenchained"
	_, err := buildImage(name,
		`FROM busybox
		ONBUILD ONBUILD RUN touch foobar`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed") {
			c.Fatalf("Wrong error %v, must be about chaining ONBUILD", err)
		}
	} else {
		c.Fatal("Error must not be nil")
	}
}

func (s *DockerSuite) TestBuildOnBuildForbiddenFrom(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildonbuildforbiddenfrom"
	_, err := buildImage(name,
		`FROM busybox
		ONBUILD FROM scratch`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "FROM isn't allowed as an ONBUILD trigger") {
			c.Fatalf("Wrong error %v, must be about FROM forbidden", err)
		}
	} else {
		c.Fatal("Error must not be nil")
	}
}

func (s *DockerSuite) TestBuildOnBuildForbiddenMaintainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildonbuildforbiddenmaintainer"
	_, err := buildImage(name,
		`FROM busybox
		ONBUILD MAINTAINER docker.io`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "MAINTAINER isn't allowed as an ONBUILD trigger") {
			c.Fatalf("Wrong error %v, must be about MAINTAINER forbidden", err)
		}
	} else {
		c.Fatal("Error must not be nil")
	}
}

func (s *DockerSuite) TestBuildOnBuildOutput(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildonbuildparent"
	if _, err := buildImage(name, "FROM busybox\nONBUILD RUN echo foo\n", true); err != nil {
		c.Fatal(err)
	}

	_, out, err := buildImageWithOut(name, "FROM "+name+"\nMAINTAINER quux\n", true)
	if err != nil {
		c.Fatal(err)
	}

	if !strings.Contains(out, "# Executing 1 build trigger") {
		c.Fatal("failed to find the build trigger output", out)
	}
}
