package main

import (
	"github.com/go-check/check"
	"strings"
)

func (s *DockerSuite) TestAliasCreateAndList(c *check.C) {
	testRequires(c, DaemonIsLinux)

	//create 1 alias
	dockerCmd(c, "alias", "r", "run -it")

	//get it
	out, _ := dockerCmd(c, "alias")

	//verify
	c.Assert(out, check.Equals, "alias r=run -it\n")
}

func (s *DockerSuite) TestAliasCreateSeveralAndList(c *check.C) {
	testRequires(c, DaemonIsLinux)

	//create 2 aliases
	dockerCmd(c, "alias", "r", "run", "-it")

	dockerCmd(c, "alias", "join", "!f(){ docker exec -it $1 /bin/sh; }; f")

	//get them
	out, _ := dockerCmd(c, "alias")
	allAliases := strings.Split(out, "\n")

	//verify
	//each line ends with a \n, there should be 3 parts
	c.Assert(len(allAliases), check.Equals, 3)

	//alias are displayed in alphabetical order
	c.Assert(allAliases[0], check.Equals, "alias join=!f(){ docker exec -it $1 /bin/sh; }; f")
	c.Assert(allAliases[1], check.Equals, "alias r=run -it")
}

func (s *DockerSuite) TestAliasCreateDeleteAndList(c *check.C) {
	testRequires(c, DaemonIsLinux)

	//create 2 aliases
	dockerCmd(c, "alias", "r", "run -it")
	dockerCmd(c, "alias", "join", "!f(){ docker exec -it $1 /bin/sh; }; f")

	//delete one
	dockerCmd(c, "alias", "-d", "r")

	//get aliases
	out, _ := dockerCmd(c, "alias")
	allAliases := strings.Split(out, "\n")

	//verify
	c.Assert(len(allAliases), check.Equals, 2)
	c.Assert(allAliases[0], check.Equals, "alias join=!f(){ docker exec -it $1 /bin/sh; }; f")

}

func (s *DockerSuite) TestAliasCreateAndRedefine(c *check.C) {
	testRequires(c, DaemonIsLinux)

	//create 1 alias
	dockerCmd(c, "alias", "r", "run -it")

	//redefine it
	dockerCmd(c, "alias", "r", "run -d")

	//get it
	out, _ := dockerCmd(c, "alias")

	//verify
	c.Assert(out, check.Equals, "alias r=run -d\n")
}

func (s *DockerSuite) TestAliasSimpleRun(c *check.C) {

	dockerCmd(c, "alias", "r", "run", "-d")

	out, _ := dockerCmd(c, "r", "busybox", "top")
	containerId := strings.TrimSpace(out)

	c.Assert(waitRun(containerId), check.IsNil)
}

func (s *DockerSuite) TestAliasComplexRun(c *check.C) {

	dockerCmd(c, "alias", "r", "!f(){ docker run -d $*; }; f")

	out, _ := dockerCmd(c, "alias")
	c.Assert(out, check.Equals, "alias r=!f(){ docker run -d $*; }; f\n")

	containerId, _ := dockerCmd(c, "r", "busybox", "top")
	containerId = strings.TrimSpace(containerId)

	c.Assert(waitRun(containerId), check.IsNil)
}
