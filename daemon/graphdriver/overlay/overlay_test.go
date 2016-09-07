// +build linux

package overlay

import (
	"testing"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"github.com/docker/docker/pkg/archive"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func init() {
	// Do not sure chroot to speed run time and allow archive
	// errors or hangs to be debugged directly from the test process.
	graphdriver.ApplyUncompressedLayer = archive.ApplyUncompressedLayer
}

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestOverlaySetup and TestOverlayTeardown
func (s *DockerSuite) TestOverlaySetup(c *check.C) {
	graphtest.GetDriver(c, "overlay")
}

func (s *DockerSuite) TestOverlayCreateEmpty(c *check.C) {
	graphtest.DriverTestCreateEmpty(c, "overlay")
}

func (s *DockerSuite) TestOverlayCreateBase(c *check.C) {
	graphtest.DriverTestCreateBase(c, "overlay")
}

func (s *DockerSuite) TestOverlayCreateSnap(c *check.C) {
	graphtest.DriverTestCreateSnap(c, "overlay")
}

func (s *DockerSuite) TestOverlay50LayerRead(c *check.C) {
	graphtest.DriverTestDeepLayerRead(c, 50, "overlay")
}

// Fails due to bug in calculating changes after apply
// likely related to https://github.com/docker/docker/issues/21555
func (s *DockerSuite) TestOverlayDiffApply10Files(c *check.C) {
	c.Skip("Fails to compute changes after apply intermittently")
	graphtest.DriverTestDiffApply(c, 10, "overlay")
}

func (s *DockerSuite) TestOverlayChanges(c *check.C) {
	c.Skip("Fails to compute changes intermittently")
	graphtest.DriverTestChanges(c, "overlay")
}

func (s *DockerSuite) TestOverlayTeardown(c *check.C) {
	graphtest.PutDriver(c)
}

// Benchmarks should always setup new driver

func (s *DockerSuite) BenchmarkExists(c *check.C) {
	graphtest.DriverBenchExists(c, "overlay")
}

func (s *DockerSuite) BenchmarkGetEmpty(c *check.C) {
	graphtest.DriverBenchGetEmpty(c, "overlay")
}

func (s *DockerSuite) BenchmarkDiffBase(c *check.C) {
	graphtest.DriverBenchDiffBase(c, "overlay")
}

func (s *DockerSuite) BenchmarkDiffSmallUpper(c *check.C) {
	graphtest.DriverBenchDiffN(c, 10, 10, "overlay")
}

func (s *DockerSuite) BenchmarkDiff10KFileUpper(c *check.C) {
	graphtest.DriverBenchDiffN(c, 10, 10000, "overlay")
}

func (s *DockerSuite) BenchmarkDiff10KFilesBottom(c *check.C) {
	graphtest.DriverBenchDiffN(c, 10000, 10, "overlay")
}

func (s *DockerSuite) BenchmarkDiffApply100(c *check.C) {
	graphtest.DriverBenchDiffApplyN(c, 100, "overlay")
}

func (s *DockerSuite) BenchmarkDiff20Layers(c *check.C) {
	graphtest.DriverBenchDeepLayerDiff(c, 20, "overlay")
}

func (s *DockerSuite) BenchmarkRead20Layers(c *check.C) {
	graphtest.DriverBenchDeepLayerRead(c, 20, "overlay")
}
