// +build linux

package overlay2

import (
	"os"
	"syscall"
	"testing"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func init() {
	// Do not sure chroot to speed run time and allow archive
	// errors or hangs to be debugged directly from the test process.
	untar = archive.UntarUncompressed
	graphdriver.ApplyUncompressedLayer = archive.ApplyUncompressedLayer

	reexec.Init()
}

func cdMountFrom(dir, device, target, mType, label string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	os.Chdir(dir)
	defer os.Chdir(wd)

	return syscall.Mount(device, target, mType, 0, label)
}

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestOverlaySetup and TestOverlayTeardown
func (s *DockerSuite) TestOverlaySetup(c *check.C) {
	graphtest.GetDriver(c, driverName)
}

func (s *DockerSuite) TestOverlayCreateEmpty(c *check.C) {
	graphtest.DriverTestCreateEmpty(c, driverName)
}

func (s *DockerSuite) TestOverlayCreateBase(c *check.C) {
	graphtest.DriverTestCreateBase(c, driverName)
}

func (s *DockerSuite) TestOverlayCreateSnap(c *check.C) {
	graphtest.DriverTestCreateSnap(c, driverName)
}

func (s *DockerSuite) TestOverlay128LayerRead(c *check.C) {
	graphtest.DriverTestDeepLayerRead(c, 128, driverName)
}

func (s *DockerSuite) TestOverlayDiffApply10Files(c *check.C) {
	graphtest.DriverTestDiffApply(c, 10, driverName)
}

func (s *DockerSuite) TestOverlayChanges(c *check.C) {
	graphtest.DriverTestChanges(c, driverName)
}

func (s *DockerSuite) TestOverlayTeardown(c *check.C) {
	graphtest.PutDriver(c)
}

// Benchmarks should always setup new driver

func (s *DockerSuite) BenchmarkExists(c *check.C) {
	graphtest.DriverBenchExists(c, driverName)
}

func (s *DockerSuite) BenchmarkGetEmpty(c *check.C) {
	graphtest.DriverBenchGetEmpty(c, driverName)
}

func (s *DockerSuite) BenchmarkDiffBase(c *check.C) {
	graphtest.DriverBenchDiffBase(c, driverName)
}

func (s *DockerSuite) BenchmarkDiffSmallUpper(c *check.C) {
	graphtest.DriverBenchDiffN(c, 10, 10, driverName)
}

func (s *DockerSuite) BenchmarkDiff10KFileUpper(c *check.C) {
	graphtest.DriverBenchDiffN(c, 10, 10000, driverName)
}

func (s *DockerSuite) BenchmarkDiff10KFilesBottom(c *check.C) {
	graphtest.DriverBenchDiffN(c, 10000, 10, driverName)
}

func (s *DockerSuite) BenchmarkDiffApply100(c *check.C) {
	graphtest.DriverBenchDiffApplyN(c, 100, driverName)
}

func (s *DockerSuite) BenchmarkDiff20Layers(c *check.C) {
	graphtest.DriverBenchDeepLayerDiff(c, 20, driverName)
}

func (s *DockerSuite) BenchmarkRead20Layers(c *check.C) {
	graphtest.DriverBenchDeepLayerRead(c, 20, driverName)
}
