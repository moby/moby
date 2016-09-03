// +build linux

package devmapper

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func init() {
	// Reduce the size the the base fs and loopback for the tests
	defaultDataLoopbackSize = 300 * 1024 * 1024
	defaultMetaDataLoopbackSize = 200 * 1024 * 1024
	defaultBaseFsSize = 300 * 1024 * 1024
	defaultUdevSyncOverride = true
	if err := graphtest.InitLoopbacks(); err != nil {
		panic(err)
	}
}

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestDevmapperSetup and TestDevmapperTeardown
func (s *DockerSuite) TestDevmapperSetup(c *check.C) {
	graphtest.GetDriver(c, "devicemapper")
}

func (s *DockerSuite) TestDevmapperCreateEmpty(c *check.C) {
	graphtest.DriverTestCreateEmpty(c, "devicemapper")
}

func (s *DockerSuite) TestDevmapperCreateBase(c *check.C) {
	graphtest.DriverTestCreateBase(c, "devicemapper")
}

func (s *DockerSuite) TestDevmapperCreateSnap(c *check.C) {
	graphtest.DriverTestCreateSnap(c, "devicemapper")
}

func (s *DockerSuite) TestDevmapperTeardown(c *check.C) {
	graphtest.PutDriver(c)
}

func (s *DockerSuite) TestDevmapperReduceLoopBackSize(c *check.C) {
	tenMB := int64(10 * 1024 * 1024)
	testChangeLoopBackSize(c, -tenMB, defaultDataLoopbackSize, defaultMetaDataLoopbackSize)
}

func (s *DockerSuite) TestDevmapperIncreaseLoopBackSize(c *check.C) {
	tenMB := int64(10 * 1024 * 1024)
	testChangeLoopBackSize(c, tenMB, defaultDataLoopbackSize+tenMB, defaultMetaDataLoopbackSize+tenMB)
}

func testChangeLoopBackSize(c *check.C, delta, expectDataSize, expectMetaDataSize int64) {
	driver := graphtest.GetDriver(c, "devicemapper").(*graphtest.Driver).Driver.(*graphdriver.NaiveDiffDriver).ProtoDriver.(*Driver)
	defer graphtest.PutDriver(c)
	// make sure data or metadata loopback size are the default size
	if s := driver.DeviceSet.Status(); s.Data.Total != uint64(defaultDataLoopbackSize) || s.Metadata.Total != uint64(defaultMetaDataLoopbackSize) {
		c.Fatalf("data or metadata loop back size is incorrect")
	}
	if err := driver.Cleanup(); err != nil {
		c.Fatal(err)
	}
	//Reload
	d, err := Init(driver.home, []string{
		fmt.Sprintf("dm.loopdatasize=%d", defaultDataLoopbackSize+delta),
		fmt.Sprintf("dm.loopmetadatasize=%d", defaultMetaDataLoopbackSize+delta),
	}, nil, nil)
	if err != nil {
		c.Fatalf("error creating devicemapper driver: %v", err)
	}
	driver = d.(*graphdriver.NaiveDiffDriver).ProtoDriver.(*Driver)
	if s := driver.DeviceSet.Status(); s.Data.Total != uint64(expectDataSize) || s.Metadata.Total != uint64(expectMetaDataSize) {
		c.Fatalf("data or metadata loop back size is incorrect")
	}
	if err := driver.Cleanup(); err != nil {
		c.Fatal(err)
	}
}

// Make sure devices.Lock() has been release upon return from cleanupDeletedDevices() function
func (s *DockerSuite) TestDevmapperLockReleasedDeviceDeletion(c *check.C) {
	driver := graphtest.GetDriver(c, "devicemapper").(*graphtest.Driver).Driver.(*graphdriver.NaiveDiffDriver).ProtoDriver.(*Driver)
	defer graphtest.PutDriver(c)

	// Call cleanupDeletedDevices() and after the call take and release
	// DeviceSet Lock. If lock has not been released, this will hang.
	driver.DeviceSet.cleanupDeletedDevices()

	doneChan := make(chan bool)

	go func() {
		driver.DeviceSet.Lock()
		defer driver.DeviceSet.Unlock()
		doneChan <- true
	}()

	select {
	case <-time.After(time.Second * 5):
		// Timer expired. That means lock was not released upon
		// function return and we are deadlocked. Release lock
		// here so that cleanup could succeed and fail the test.
		driver.DeviceSet.Unlock()
		c.Fatalf("Could not acquire devices lock after call to cleanupDeletedDevices()")
	case <-doneChan:
	}
}
