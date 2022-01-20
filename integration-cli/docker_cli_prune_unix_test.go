//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/integration-cli/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
)

func (s *DockerCLIPruneSuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLIPruneSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func pruneNetworkAndVerify(c *testing.T, d *daemon.Daemon, kept, pruned []string) {
	_, err := d.Cmd("network", "prune", "--force")
	assert.NilError(c, err)

	for _, s := range kept {
		poll.WaitOn(c, pollCheck(c, func(*testing.T) (interface{}, string) {
			out, err := d.Cmd("network", "ls", "--format", "{{.Name}}")
			assert.NilError(c, err)
			return out, ""
		}, checker.Contains(s)), poll.WithTimeout(defaultReconciliationTimeout))
	}

	for _, s := range pruned {
		poll.WaitOn(c, pollCheck(c, func(*testing.T) (interface{}, string) {
			out, err := d.Cmd("network", "ls", "--format", "{{.Name}}")
			assert.NilError(c, err)
			return out, ""
		}, checker.Not(checker.Contains(s))), poll.WithTimeout(defaultReconciliationTimeout))
	}
}

func (s *DockerSwarmSuite) TestPruneNetwork(c *testing.T) {
	d := s.AddDaemon(c, true, true)
	_, err := d.Cmd("network", "create", "n1") // used by container (testprune)
	assert.NilError(c, err)
	_, err = d.Cmd("network", "create", "n2")
	assert.NilError(c, err)
	_, err = d.Cmd("network", "create", "n3", "--driver", "overlay") // used by service (testprunesvc)
	assert.NilError(c, err)
	_, err = d.Cmd("network", "create", "n4", "--driver", "overlay")
	assert.NilError(c, err)

	cName := "testprune"
	_, err = d.Cmd("run", "-d", "--name", cName, "--net", "n1", "busybox", "top")
	assert.NilError(c, err)

	serviceName := "testprunesvc"
	replicas := 1
	out, err := d.Cmd("service", "create", "--detach", "--no-resolve-image",
		"--name", serviceName,
		"--replicas", strconv.Itoa(replicas),
		"--network", "n3",
		"busybox", "top")
	assert.NilError(c, err)
	assert.Assert(c, strings.TrimSpace(out) != "")
	poll.WaitOn(c, pollCheck(c, d.CheckActiveContainerCount, checker.Equals(replicas+1)), poll.WithTimeout(defaultReconciliationTimeout))

	// prune and verify
	pruneNetworkAndVerify(c, d, []string{"n1", "n3"}, []string{"n2", "n4"})

	// remove containers, then prune and verify again
	_, err = d.Cmd("rm", "-f", cName)
	assert.NilError(c, err)
	_, err = d.Cmd("service", "rm", serviceName)
	assert.NilError(c, err)
	poll.WaitOn(c, pollCheck(c, d.CheckActiveContainerCount, checker.Equals(0)), poll.WithTimeout(defaultReconciliationTimeout))

	pruneNetworkAndVerify(c, d, []string{}, []string{"n1", "n3"})
}

func (s *DockerDaemonSuite) TestPruneImageDangling(c *testing.T) {
	s.d.StartWithBusybox(c)

	result := cli.BuildCmd(c, "test", cli.Daemon(s.d),
		build.WithDockerfile(`FROM busybox
                 LABEL foo=bar`),
		cli.WithFlags("-q"),
	)
	result.Assert(c, icmd.Success)
	id := strings.TrimSpace(result.Combined())

	out, err := s.d.Cmd("images", "-q", "--no-trunc")
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id))
	out, err = s.d.Cmd("image", "prune", "--force")
	assert.NilError(c, err)
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id))
	out, err = s.d.Cmd("images", "-q", "--no-trunc")
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id))
	out, err = s.d.Cmd("image", "prune", "--force", "--all")
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id))
	out, err = s.d.Cmd("images", "-q", "--no-trunc")
	assert.NilError(c, err)
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id))
}

func (s *DockerCLIPruneSuite) TestPruneContainerUntil(c *testing.T) {
	out := cli.DockerCmd(c, "run", "-d", "busybox").Combined()
	id1 := strings.TrimSpace(out)
	cli.WaitExited(c, id1, 5*time.Second)

	until := daemonUnixTime(c)

	out = cli.DockerCmd(c, "run", "-d", "busybox").Combined()
	id2 := strings.TrimSpace(out)
	cli.WaitExited(c, id2, 5*time.Second)

	out = cli.DockerCmd(c, "container", "prune", "--force", "--filter", "until="+until).Combined()
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
	out = cli.DockerCmd(c, "ps", "-a", "-q", "--no-trunc").Combined()
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id2))
}

func (s *DockerCLIPruneSuite) TestPruneContainerLabel(c *testing.T) {
	out := cli.DockerCmd(c, "run", "-d", "--label", "foo", "busybox").Combined()
	id1 := strings.TrimSpace(out)
	cli.WaitExited(c, id1, 5*time.Second)

	out = cli.DockerCmd(c, "run", "-d", "--label", "bar", "busybox").Combined()
	id2 := strings.TrimSpace(out)
	cli.WaitExited(c, id2, 5*time.Second)

	out = cli.DockerCmd(c, "run", "-d", "busybox").Combined()
	id3 := strings.TrimSpace(out)
	cli.WaitExited(c, id3, 5*time.Second)

	out = cli.DockerCmd(c, "run", "-d", "--label", "foobar", "busybox").Combined()
	id4 := strings.TrimSpace(out)
	cli.WaitExited(c, id4, 5*time.Second)

	// Add a config file of label=foobar, that will have no impact if cli is label!=foobar
	config := `{"pruneFilters": ["label=foobar"]}`
	d, err := os.MkdirTemp("", "integration-cli-")
	assert.NilError(c, err)
	defer os.RemoveAll(d)
	err = os.WriteFile(filepath.Join(d, "config.json"), []byte(config), 0o644)
	assert.NilError(c, err)

	// With config.json only, prune based on label=foobar
	out = cli.DockerCmd(c, "--config", d, "container", "prune", "--force").Combined()
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id3))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id4))
	out = cli.DockerCmd(c, "container", "prune", "--force", "--filter", "label=foo").Combined()
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id3))
	out = cli.DockerCmd(c, "ps", "-a", "-q", "--no-trunc").Combined()
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id2))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id3))
	out = cli.DockerCmd(c, "container", "prune", "--force", "--filter", "label!=bar").Combined()
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id3))
	out = cli.DockerCmd(c, "ps", "-a", "-q", "--no-trunc").Combined()
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id2))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id3))
	// With config.json label=foobar and CLI label!=foobar, CLI label!=foobar supersede
	out = cli.DockerCmd(c, "--config", d, "container", "prune", "--force", "--filter", "label!=foobar").Combined()
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id2))
	out = cli.DockerCmd(c, "ps", "-a", "-q", "--no-trunc").Combined()
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
}

func (s *DockerCLIPruneSuite) TestPruneVolumeLabel(c *testing.T) {
	out, _ := dockerCmd(c, "volume", "create", "--label", "foo")
	id1 := strings.TrimSpace(out)
	assert.Assert(c, id1 != "")

	out, _ = dockerCmd(c, "volume", "create", "--label", "bar")
	id2 := strings.TrimSpace(out)
	assert.Assert(c, id2 != "")

	out, _ = dockerCmd(c, "volume", "create")
	id3 := strings.TrimSpace(out)
	assert.Assert(c, id3 != "")

	out, _ = dockerCmd(c, "volume", "create", "--label", "foobar")
	id4 := strings.TrimSpace(out)
	assert.Assert(c, id4 != "")

	// Add a config file of label=foobar, that will have no impact if cli is label!=foobar
	config := `{"pruneFilters": ["label=foobar"]}`
	d, err := os.MkdirTemp("", "integration-cli-")
	assert.NilError(c, err)
	defer os.RemoveAll(d)
	err = os.WriteFile(filepath.Join(d, "config.json"), []byte(config), 0o644)
	assert.NilError(c, err)

	// With config.json only, prune based on label=foobar
	out, _ = dockerCmd(c, "--config", d, "volume", "prune", "--force")
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id3))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id4))
	out, _ = dockerCmd(c, "volume", "prune", "--force", "--filter", "label=foo")
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id3))
	out, _ = dockerCmd(c, "volume", "ls", "--format", "{{.Name}}")
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id2))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id3))
	out, _ = dockerCmd(c, "volume", "prune", "--force", "--filter", "label!=bar")
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id3))
	out, _ = dockerCmd(c, "volume", "ls", "--format", "{{.Name}}")
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id2))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id3))
	// With config.json label=foobar and CLI label!=foobar, CLI label!=foobar supersede
	out, _ = dockerCmd(c, "--config", d, "volume", "prune", "--force", "--filter", "label!=foobar")
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id2))
	out, _ = dockerCmd(c, "volume", "ls", "--format", "{{.Name}}")
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
}

func (s *DockerCLIPruneSuite) TestPruneNetworkLabel(c *testing.T) {
	dockerCmd(c, "network", "create", "--label", "foo", "n1")
	dockerCmd(c, "network", "create", "--label", "bar", "n2")
	dockerCmd(c, "network", "create", "n3")

	out, _ := dockerCmd(c, "network", "prune", "--force", "--filter", "label=foo")
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), "n1"))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), "n2"))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), "n3"))
	out, _ = dockerCmd(c, "network", "prune", "--force", "--filter", "label!=bar")
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), "n1"))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), "n2"))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), "n3"))
	out, _ = dockerCmd(c, "network", "prune", "--force")
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), "n1"))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), "n2"))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), "n3"))
}

func (s *DockerDaemonSuite) TestPruneImageLabel(c *testing.T) {
	s.d.StartWithBusybox(c)

	result := cli.BuildCmd(c, "test1", cli.Daemon(s.d),
		build.WithDockerfile(`FROM busybox
                 LABEL foo=bar`),
		cli.WithFlags("-q"),
	)
	result.Assert(c, icmd.Success)
	id1 := strings.TrimSpace(result.Combined())
	out, err := s.d.Cmd("images", "-q", "--no-trunc")
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id1))
	result = cli.BuildCmd(c, "test2", cli.Daemon(s.d),
		build.WithDockerfile(`FROM busybox
                 LABEL bar=foo`),
		cli.WithFlags("-q"),
	)
	result.Assert(c, icmd.Success)
	id2 := strings.TrimSpace(result.Combined())
	out, err = s.d.Cmd("images", "-q", "--no-trunc")
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id2))
	out, err = s.d.Cmd("image", "prune", "--force", "--all", "--filter", "label=foo=bar")
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
	out, err = s.d.Cmd("image", "prune", "--force", "--all", "--filter", "label!=bar=foo")
	assert.NilError(c, err)
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id2))
	out, err = s.d.Cmd("image", "prune", "--force", "--all", "--filter", "label=bar=foo")
	assert.NilError(c, err)
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), id1))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), id2))
}
