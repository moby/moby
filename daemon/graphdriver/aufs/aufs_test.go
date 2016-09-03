// +build linux

package aufs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"testing"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

var (
	tmpOuter = path.Join(os.TempDir(), "aufs-tests")
	tmp      = path.Join(tmpOuter, "aufs")
)

func init() {
	reexec.Init()
}

func testInit(dir string, c *check.C) graphdriver.Driver {
	d, err := Init(dir, nil, nil, nil)
	if err != nil {
		if err == graphdriver.ErrNotSupported {
			c.Skip(err.Error())
		} else {
			c.Fatal(err)
		}
	}
	return d
}

func newDriver(c *check.C) *Driver {
	if err := os.MkdirAll(tmp, 0755); err != nil {
		c.Fatal(err)
	}

	d := testInit(tmp, c)
	return d.(*Driver)
}

func (s *DockerSuite) TestNewDriver(c *check.C) {
	if err := os.MkdirAll(tmp, 0755); err != nil {
		c.Fatal(err)
	}

	d := testInit(tmp, c)
	defer os.RemoveAll(tmp)
	if d == nil {
		c.Fatalf("Driver should not be nil")
	}
}

func (s *DockerSuite) TestAufsString(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if d.String() != "aufs" {
		c.Fatalf("Expected aufs got %s", d.String())
	}
}

func (s *DockerSuite) TestCreateDirStructure(c *check.C) {
	newDriver(c)
	defer os.RemoveAll(tmp)

	paths := []string{
		"mnt",
		"layers",
		"diff",
	}

	for _, p := range paths {
		if _, err := os.Stat(path.Join(tmp, p)); err != nil {
			c.Fatal(err)
		}
	}
}

// We should be able to create two drivers with the same dir structure
func (s *DockerSuite) TestNewDriverFromExistingDir(c *check.C) {
	if err := os.MkdirAll(tmp, 0755); err != nil {
		c.Fatal(err)
	}

	testInit(tmp, c)
	testInit(tmp, c)
	os.RemoveAll(tmp)
}

func (s *DockerSuite) TestCreateNewDir(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestCreateNewDirStructure(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	paths := []string{
		"mnt",
		"diff",
		"layers",
	}

	for _, p := range paths {
		if _, err := os.Stat(path.Join(tmp, p, "1")); err != nil {
			c.Fatal(err)
		}
	}
}

func (s *DockerSuite) TestRemoveImage(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	if err := d.Remove("1"); err != nil {
		c.Fatal(err)
	}

	paths := []string{
		"mnt",
		"diff",
		"layers",
	}

	for _, p := range paths {
		if _, err := os.Stat(path.Join(tmp, p, "1")); err == nil {
			c.Fatalf("Error should not be nil because dirs with id 1 should be delted: %s", p)
		}
	}
}

func (s *DockerSuite) TestGetWithoutParent(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	diffPath, err := d.Get("1", "")
	if err != nil {
		c.Fatal(err)
	}
	expected := path.Join(tmp, "diff", "1")
	if diffPath != expected {
		c.Fatalf("Expected path %s got %s", expected, diffPath)
	}
}

func (s *DockerSuite) TestCleanupWithNoDirs(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Cleanup(); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestCleanupWithDir(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	if err := d.Cleanup(); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestMountedFalseResponse(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	response, err := d.mounted(d.getDiffPath("1"))
	if err != nil {
		c.Fatal(err)
	}

	if response != false {
		c.Fatalf("Response if dir id 1 is mounted should be false")
	}
}

func (s *DockerSuite) TestMountedTrueReponse(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}
	if err := d.Create("2", "1", "", nil); err != nil {
		c.Fatal(err)
	}

	_, err := d.Get("2", "")
	if err != nil {
		c.Fatal(err)
	}

	response, err := d.mounted(d.pathCache["2"])
	if err != nil {
		c.Fatal(err)
	}

	if response != true {
		c.Fatalf("Response if dir id 2 is mounted should be true")
	}
}

func (s *DockerSuite) TestMountWithParent(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}
	if err := d.Create("2", "1", "", nil); err != nil {
		c.Fatal(err)
	}

	defer func() {
		if err := d.Cleanup(); err != nil {
			c.Fatal(err)
		}
	}()

	mntPath, err := d.Get("2", "")
	if err != nil {
		c.Fatal(err)
	}
	if mntPath == "" {
		c.Fatal("mntPath should not be empty string")
	}

	expected := path.Join(tmp, "mnt", "2")
	if mntPath != expected {
		c.Fatalf("Expected %s got %s", expected, mntPath)
	}
}

func (s *DockerSuite) TestRemoveMountedDir(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}
	if err := d.Create("2", "1", "", nil); err != nil {
		c.Fatal(err)
	}

	defer func() {
		if err := d.Cleanup(); err != nil {
			c.Fatal(err)
		}
	}()

	mntPath, err := d.Get("2", "")
	if err != nil {
		c.Fatal(err)
	}
	if mntPath == "" {
		c.Fatal("mntPath should not be empty string")
	}

	mounted, err := d.mounted(d.pathCache["2"])
	if err != nil {
		c.Fatal(err)
	}

	if !mounted {
		c.Fatalf("Dir id 2 should be mounted")
	}

	if err := d.Remove("2"); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestCreateWithInvalidParent(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "docker", "", nil); err == nil {
		c.Fatalf("Error should not be nil with parent does not exist")
	}
}

func (s *DockerSuite) TestGetDiff(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.CreateReadWrite("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	diffPath, err := d.Get("1", "")
	if err != nil {
		c.Fatal(err)
	}

	// Add a file to the diff path with a fixed size
	size := int64(1024)

	f, err := os.Create(path.Join(diffPath, "test_file"))
	if err != nil {
		c.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		c.Fatal(err)
	}
	f.Close()

	a, err := d.Diff("1", "")
	if err != nil {
		c.Fatal(err)
	}
	if a == nil {
		c.Fatalf("Archive should not be nil")
	}
}

func (s *DockerSuite) TestChanges(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}
	if err := d.CreateReadWrite("2", "1", "", nil); err != nil {
		c.Fatal(err)
	}

	defer func() {
		if err := d.Cleanup(); err != nil {
			c.Fatal(err)
		}
	}()

	mntPoint, err := d.Get("2", "")
	if err != nil {
		c.Fatal(err)
	}

	// Create a file to save in the mountpoint
	f, err := os.Create(path.Join(mntPoint, "test.txt"))
	if err != nil {
		c.Fatal(err)
	}

	if _, err := f.WriteString("testline"); err != nil {
		c.Fatal(err)
	}
	if err := f.Close(); err != nil {
		c.Fatal(err)
	}

	changes, err := d.Changes("2", "")
	if err != nil {
		c.Fatal(err)
	}
	if len(changes) != 1 {
		c.Fatalf("Dir 2 should have one change from parent got %d", len(changes))
	}
	change := changes[0]

	expectedPath := "/test.txt"
	if change.Path != expectedPath {
		c.Fatalf("Expected path %s got %s", expectedPath, change.Path)
	}

	if change.Kind != archive.ChangeAdd {
		c.Fatalf("Change kind should be ChangeAdd got %s", change.Kind)
	}

	if err := d.CreateReadWrite("3", "2", "", nil); err != nil {
		c.Fatal(err)
	}
	mntPoint, err = d.Get("3", "")
	if err != nil {
		c.Fatal(err)
	}

	// Create a file to save in the mountpoint
	f, err = os.Create(path.Join(mntPoint, "test2.txt"))
	if err != nil {
		c.Fatal(err)
	}

	if _, err := f.WriteString("testline"); err != nil {
		c.Fatal(err)
	}
	if err := f.Close(); err != nil {
		c.Fatal(err)
	}

	changes, err = d.Changes("3", "")
	if err != nil {
		c.Fatal(err)
	}

	if len(changes) != 1 {
		c.Fatalf("Dir 2 should have one change from parent got %d", len(changes))
	}
	change = changes[0]

	expectedPath = "/test2.txt"
	if change.Path != expectedPath {
		c.Fatalf("Expected path %s got %s", expectedPath, change.Path)
	}

	if change.Kind != archive.ChangeAdd {
		c.Fatalf("Change kind should be ChangeAdd got %s", change.Kind)
	}
}

func (s *DockerSuite) TestDiffSize(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)

	if err := d.CreateReadWrite("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	diffPath, err := d.Get("1", "")
	if err != nil {
		c.Fatal(err)
	}

	// Add a file to the diff path with a fixed size
	size := int64(1024)

	f, err := os.Create(path.Join(diffPath, "test_file"))
	if err != nil {
		c.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		c.Fatal(err)
	}
	fs, err := f.Stat()
	if err != nil {
		c.Fatal(err)
	}
	size = fs.Size()
	if err := f.Close(); err != nil {
		c.Fatal(err)
	}

	diffSize, err := d.DiffSize("1", "")
	if err != nil {
		c.Fatal(err)
	}
	if diffSize != size {
		c.Fatalf("Expected size to be %d got %d", size, diffSize)
	}
}

func (s *DockerSuite) TestChildDiffSize(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.CreateReadWrite("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	diffPath, err := d.Get("1", "")
	if err != nil {
		c.Fatal(err)
	}

	// Add a file to the diff path with a fixed size
	size := int64(1024)

	f, err := os.Create(path.Join(diffPath, "test_file"))
	if err != nil {
		c.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		c.Fatal(err)
	}
	fs, err := f.Stat()
	if err != nil {
		c.Fatal(err)
	}
	size = fs.Size()
	if err := f.Close(); err != nil {
		c.Fatal(err)
	}

	diffSize, err := d.DiffSize("1", "")
	if err != nil {
		c.Fatal(err)
	}
	if diffSize != size {
		c.Fatalf("Expected size to be %d got %d", size, diffSize)
	}

	if err := d.Create("2", "1", "", nil); err != nil {
		c.Fatal(err)
	}

	diffSize, err = d.DiffSize("2", "")
	if err != nil {
		c.Fatal(err)
	}
	// The diff size for the child should be zero
	if diffSize != 0 {
		c.Fatalf("Expected size to be %d got %d", 0, diffSize)
	}
}

func (s *DockerSuite) TestExists(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	if d.Exists("none") {
		c.Fatal("id none should not exist in the driver")
	}

	if !d.Exists("1") {
		c.Fatal("id 1 should exist in the driver")
	}
}

func (s *DockerSuite) TestStatus(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.Create("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	status := d.Status()
	if status == nil || len(status) == 0 {
		c.Fatal("Status should not be nil or empty")
	}
	rootDir := status[0]
	dirs := status[2]
	if rootDir[0] != "Root Dir" {
		c.Fatalf("Expected Root Dir got %s", rootDir[0])
	}
	if rootDir[1] != d.rootPath() {
		c.Fatalf("Expected %s got %s", d.rootPath(), rootDir[1])
	}
	if dirs[0] != "Dirs" {
		c.Fatalf("Expected Dirs got %s", dirs[0])
	}
	if dirs[1] != "1" {
		c.Fatalf("Expected 1 got %s", dirs[1])
	}
}

func (s *DockerSuite) TestApplyDiff(c *check.C) {
	d := newDriver(c)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.CreateReadWrite("1", "", "", nil); err != nil {
		c.Fatal(err)
	}

	diffPath, err := d.Get("1", "")
	if err != nil {
		c.Fatal(err)
	}

	// Add a file to the diff path with a fixed size
	size := int64(1024)

	f, err := os.Create(path.Join(diffPath, "test_file"))
	if err != nil {
		c.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		c.Fatal(err)
	}
	f.Close()

	diff, err := d.Diff("1", "")
	if err != nil {
		c.Fatal(err)
	}

	if err := d.Create("2", "", "", nil); err != nil {
		c.Fatal(err)
	}
	if err := d.Create("3", "2", "", nil); err != nil {
		c.Fatal(err)
	}

	if err := d.applyDiff("3", diff); err != nil {
		c.Fatal(err)
	}

	// Ensure that the file is in the mount point for id 3

	mountPoint, err := d.Get("3", "")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := os.Stat(path.Join(mountPoint, "test_file")); err != nil {
		c.Fatal(err)
	}
}

func hash(c string) string {
	h := sha256.New()
	fmt.Fprint(h, c)
	return hex.EncodeToString(h.Sum(nil))
}

func testMountMoreThan42Layers(c *check.C, mountPath string) {
	if err := os.MkdirAll(mountPath, 0755); err != nil {
		c.Fatal(err)
	}

	defer os.RemoveAll(mountPath)
	d := testInit(mountPath, c).(*Driver)
	defer d.Cleanup()
	var last string
	var expected int

	for i := 1; i < 127; i++ {
		expected++
		var (
			parent  = fmt.Sprintf("%d", i-1)
			current = fmt.Sprintf("%d", i)
		)

		if parent == "0" {
			parent = ""
		} else {
			parent = hash(parent)
		}
		current = hash(current)

		if err := d.CreateReadWrite(current, parent, "", nil); err != nil {
			c.Logf("Current layer %d", i)
			c.Error(err)
		}
		point, err := d.Get(current, "")
		if err != nil {
			c.Logf("Current layer %d", i)
			c.Error(err)
		}
		f, err := os.Create(path.Join(point, current))
		if err != nil {
			c.Logf("Current layer %d", i)
			c.Error(err)
		}
		f.Close()

		if i%10 == 0 {
			if err := os.Remove(path.Join(point, parent)); err != nil {
				c.Logf("Current layer %d", i)
				c.Error(err)
			}
			expected--
		}
		last = current
	}

	// Perform the actual mount for the top most image
	point, err := d.Get(last, "")
	if err != nil {
		c.Error(err)
	}
	files, err := ioutil.ReadDir(point)
	if err != nil {
		c.Error(err)
	}
	if len(files) != expected {
		c.Errorf("Expected %d got %d", expected, len(files))
	}
}

func (s *DockerSuite) TestMountMoreThan42Layers(c *check.C) {
	os.RemoveAll(tmpOuter)
	testMountMoreThan42Layers(c, tmp)
}

func (s *DockerSuite) TestMountMoreThan42LayersMatchingPathLength(c *check.C) {
	defer os.RemoveAll(tmpOuter)
	zeroes := "0"
	for {
		// This finds a mount path so that when combined into aufs mount options
		// 4096 byte boundary would be in between the paths or in permission
		// section. For '/tmp' it will use '/tmp/aufs-tests/00000000/aufs'
		mountPath := path.Join(tmpOuter, zeroes, "aufs")
		pathLength := 77 + len(mountPath)

		if mod := 4095 % pathLength; mod == 0 || mod > pathLength-2 {
			c.Logf("Using path: %s", mountPath)
			testMountMoreThan42Layers(c, mountPath)
			return
		}
		zeroes += "0"
	}
}

func (s *DockerSuite) BenchmarkConcurrentAccess(c *check.C) {
	c.StopTimer()
	c.ResetTimer()

	d := newDriver(c)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	numConcurent := 256
	// create a bunch of ids
	var ids []string
	for i := 0; i < numConcurent; i++ {
		ids = append(ids, stringid.GenerateNonCryptoID())
	}

	if err := d.Create(ids[0], "", "", nil); err != nil {
		c.Fatal(err)
	}

	if err := d.Create(ids[1], ids[0], "", nil); err != nil {
		c.Fatal(err)
	}

	parent := ids[1]
	ids = append(ids[2:])

	chErr := make(chan error, numConcurent)
	var outerGroup sync.WaitGroup
	outerGroup.Add(len(ids))
	c.StartTimer()

	// here's the actual bench
	for _, id := range ids {
		go func(id string) {
			defer outerGroup.Done()
			if err := d.Create(id, parent, "", nil); err != nil {
				c.Logf("Create %s failed", id)
				chErr <- err
				return
			}
			var innerGroup sync.WaitGroup
			for i := 0; i < c.N; i++ {
				innerGroup.Add(1)
				go func() {
					d.Get(id, "")
					d.Put(id)
					innerGroup.Done()
				}()
			}
			innerGroup.Wait()
			d.Remove(id)
		}(id)
	}

	outerGroup.Wait()
	c.StopTimer()
	close(chErr)
	for err := range chErr {
		if err != nil {
			c.Log(err)
			c.Fail()
		}
	}
}
