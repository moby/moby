// +build !windows

package main

import (
	"encoding/json"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/ulimit"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildResourceConstraintsAreUsed(c *check.C) {
	testRequires(c, cpuCfsQuota)
	name := "testbuildresourceconstraints"

	ctx, err := fakeContext(`
	FROM hello-world:frozen
	RUN ["/hello"]
	`, map[string]string{})
	c.Assert(err, checker.IsNil)

	dockerCmdInDir(c, ctx.Dir, "build", "--no-cache", "--rm=false", "--memory=64m", "--memory-swap=-1", "--cpuset-cpus=0", "--cpuset-mems=0", "--cpu-shares=100", "--cpu-quota=8000", "--ulimit", "nofile=42", "-t", name, ".")

	out, _ := dockerCmd(c, "ps", "-lq")
	cID := strings.TrimSpace(out)

	type hostConfig struct {
		Memory     int64
		MemorySwap int64
		CpusetCpus string
		CpusetMems string
		CPUShares  int64
		CPUQuota   int64
		Ulimits    []*ulimit.Ulimit
	}

	cfg, err := inspectFieldJSON(cID, "HostConfig")
	c.Assert(err, checker.IsNil)

	var c1 hostConfig
	err = json.Unmarshal([]byte(cfg), &c1)
	c.Assert(err, checker.IsNil, check.Commentf(cfg))

	c.Assert(c1.Memory, checker.Equals, int64(64*1024*1024), check.Commentf("resource constraints not set properly for Memory"))
	c.Assert(c1.MemorySwap, checker.Equals, int64(-1), check.Commentf("resource constraints not set properly for MemorySwap"))
	c.Assert(c1.CpusetCpus, checker.Equals, "0", check.Commentf("resource constraints not set properly for CpusetCpus"))
	c.Assert(c1.CpusetMems, checker.Equals, "0", check.Commentf("resource constraints not set properly for CpusetMems"))
	c.Assert(c1.CPUShares, checker.Equals, int64(100), check.Commentf("resource constraints not set properly for CPUShares"))
	c.Assert(c1.CPUQuota, checker.Equals, int64(8000), check.Commentf("resource constraints not set properly for CPUQuota"))
	c.Assert(c1.Ulimits[0].Name, checker.Equals, "nofile", check.Commentf("resource constraints not set properly for Ulimits"))
	c.Assert(c1.Ulimits[0].Hard, checker.Equals, int64(42), check.Commentf("resource constraints not set properly for Ulimits"))

	// Make sure constraints aren't saved to image
	dockerCmd(c, "run", "--name=test", name)

	cfg, err = inspectFieldJSON("test", "HostConfig")
	c.Assert(err, checker.IsNil)

	var c2 hostConfig
	err = json.Unmarshal([]byte(cfg), &c2)
	c.Assert(err, checker.IsNil, check.Commentf(cfg))

	c.Assert(c2.Memory, check.Not(checker.Equals), int64(64*1024*1024), check.Commentf("resource leaked from build for Memory"))
	c.Assert(c2.MemorySwap, check.Not(checker.Equals), int64(-1), check.Commentf("resource leaked from build for MemorySwap"))
	c.Assert(c2.CpusetCpus, check.Not(checker.Equals), "0", check.Commentf("resource leaked from build for CpusetCpus"))
	c.Assert(c2.CpusetMems, check.Not(checker.Equals), "0", check.Commentf("resource leaked from build for CpusetMems"))
	c.Assert(c2.CPUShares, check.Not(checker.Equals), int64(100), check.Commentf("resource leaked from build for CPUShares"))
	c.Assert(c2.CPUQuota, check.Not(checker.Equals), int64(8000), check.Commentf("resource leaked from build for CPUQuota"))
	c.Assert(c2.Ulimits, checker.IsNil, check.Commentf("resource leaked from build for Ulimits"))
}
