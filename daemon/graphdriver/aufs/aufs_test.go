//go:build linux
// +build linux

package aufs // import "github.com/docker/docker/daemon/graphdriver/aufs"

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/stringid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

var (
	tmpOuter = path.Join(os.TempDir(), "aufs-tests")
	tmp      = path.Join(tmpOuter, "aufs")
)

func init() {
	reexec.Init()
}

func testInit(dir string, t testing.TB) graphdriver.Driver {
	d, err := Init(dir, nil, nil, nil)
	if err != nil {
		if err == graphdriver.ErrNotSupported {
			t.Skip(err)
		} else {
			t.Fatal(err)
		}
	}
	return d
}

func driverGet(d *Driver, id string, mntLabel string) (string, error) {
	mnt, err := d.Get(id, mntLabel)
	if err != nil {
		return "", err
	}
	return mnt.Path(), nil
}

func newDriver(t testing.TB) *Driver {
	if err := os.MkdirAll(tmp, 0755); err != nil {
		t.Fatal(err)
	}

	d := testInit(tmp, t)
	return d.(*Driver)
}

func TestNewDriver(t *testing.T) {
	if err := os.MkdirAll(tmp, 0755); err != nil {
		t.Fatal(err)
	}

	d := testInit(tmp, t)
	defer os.RemoveAll(tmp)
	if d == nil {
		t.Fatal("Driver should not be nil")
	}
}

func TestAufsString(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if d.String() != "aufs" {
		t.Fatalf("Expected aufs got %s", d.String())
	}
}

func TestCreateDirStructure(t *testing.T) {
	newDriver(t)
	defer os.RemoveAll(tmp)

	paths := []string{
		"mnt",
		"layers",
		"diff",
	}

	for _, p := range paths {
		if _, err := os.Stat(path.Join(tmp, p)); err != nil {
			t.Fatal(err)
		}
	}
}

// We should be able to create two drivers with the same dir structure
func TestNewDriverFromExistingDir(t *testing.T) {
	if err := os.MkdirAll(tmp, 0755); err != nil {
		t.Fatal(err)
	}

	testInit(tmp, t)
	testInit(tmp, t)
	os.RemoveAll(tmp)
}

func TestCreateNewDir(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", nil); err != nil {
		t.Fatal(err)
	}
}

func TestCreateNewDirStructure(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", nil); err != nil {
		t.Fatal(err)
	}

	paths := []string{
		"mnt",
		"diff",
		"layers",
	}

	for _, p := range paths {
		if _, err := os.Stat(path.Join(tmp, p, "1")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRemoveImage(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", nil); err != nil {
		t.Fatal(err)
	}

	if err := d.Remove("1"); err != nil {
		t.Fatal(err)
	}

	paths := []string{
		"mnt",
		"diff",
		"layers",
	}

	for _, p := range paths {
		if _, err := os.Stat(path.Join(tmp, p, "1")); err == nil {
			t.Fatalf("Error should not be nil because dirs with id 1 should be deleted: %s", p)
		}
		if _, err := os.Stat(path.Join(tmp, p, "1-removing")); err == nil {
			t.Fatalf("Error should not be nil because dirs with id 1-removing should be deleted: %s", p)
		}
	}
}

func TestGetWithoutParent(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", nil); err != nil {
		t.Fatal(err)
	}

	diffPath, err := d.Get("1", "")
	if err != nil {
		t.Fatal(err)
	}
	expected := path.Join(tmp, "diff", "1")
	if diffPath.Path() != expected {
		t.Fatalf("Expected path %s got %s", expected, diffPath)
	}
}

func TestCleanupWithNoDirs(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	err := d.Cleanup()
	assert.Check(t, err)
}

func TestCleanupWithDir(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", nil); err != nil {
		t.Fatal(err)
	}

	if err := d.Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestMountedFalseResponse(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	err := d.Create("1", "", nil)
	assert.NilError(t, err)

	response, err := d.mounted(d.getDiffPath("1"))
	assert.NilError(t, err)
	assert.Check(t, !response)
}

func TestMountedTrueResponse(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	err := d.Create("1", "", nil)
	assert.NilError(t, err)
	err = d.Create("2", "1", nil)
	assert.NilError(t, err)

	_, err = d.Get("2", "")
	assert.NilError(t, err)

	response, err := d.mounted(d.pathCache["2"])
	assert.NilError(t, err)
	assert.Check(t, response)
}

func TestMountWithParent(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.Create("2", "1", nil); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := d.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	mntPath, err := d.Get("2", "")
	if err != nil {
		t.Fatal(err)
	}
	if mntPath == nil {
		t.Fatal("mntPath should not be nil")
	}

	expected := path.Join(tmp, "mnt", "2")
	if mntPath.Path() != expected {
		t.Fatalf("Expected %s got %s", expected, mntPath.Path())
	}
}

func TestRemoveMountedDir(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.Create("2", "1", nil); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := d.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	mntPath, err := d.Get("2", "")
	if err != nil {
		t.Fatal(err)
	}
	if mntPath == nil {
		t.Fatal("mntPath should not be nil")
	}

	mounted, err := d.mounted(d.pathCache["2"])
	if err != nil {
		t.Fatal(err)
	}

	if !mounted {
		t.Fatal("Dir id 2 should be mounted")
	}

	if err := d.Remove("2"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateWithInvalidParent(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "docker", nil); err == nil {
		t.Fatal("Error should not be nil with parent does not exist")
	}
}

func TestGetDiff(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.CreateReadWrite("1", "", nil); err != nil {
		t.Fatal(err)
	}

	diffPath, err := driverGet(d, "1", "")
	if err != nil {
		t.Fatal(err)
	}

	// Add a file to the diff path with a fixed size
	size := int64(1024)

	f, err := os.Create(path.Join(diffPath, "test_file"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	f.Close()

	a, err := d.Diff("1", "")
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("Archive should not be nil")
	}
}

func TestChanges(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "", nil); err != nil {
		t.Fatal(err)
	}

	if err := d.CreateReadWrite("2", "1", nil); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := d.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	mntPoint, err := driverGet(d, "2", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create a file to save in the mountpoint
	f, err := os.Create(path.Join(mntPoint, "test.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.WriteString("testline"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	changes, err := d.Changes("2", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("Dir 2 should have one change from parent got %d", len(changes))
	}
	change := changes[0]

	expectedPath := "/test.txt"
	if change.Path != expectedPath {
		t.Fatalf("Expected path %s got %s", expectedPath, change.Path)
	}

	if change.Kind != archive.ChangeAdd {
		t.Fatalf("Change kind should be ChangeAdd got %s", change.Kind)
	}

	if err := d.CreateReadWrite("3", "2", nil); err != nil {
		t.Fatal(err)
	}
	mntPoint, err = driverGet(d, "3", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create a file to save in the mountpoint
	f, err = os.Create(path.Join(mntPoint, "test2.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.WriteString("testline"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	changes, err = d.Changes("3", "2")
	if err != nil {
		t.Fatal(err)
	}

	if len(changes) != 1 {
		t.Fatalf("Dir 2 should have one change from parent got %d", len(changes))
	}
	change = changes[0]

	expectedPath = "/test2.txt"
	if change.Path != expectedPath {
		t.Fatalf("Expected path %s got %s", expectedPath, change.Path)
	}

	if change.Kind != archive.ChangeAdd {
		t.Fatalf("Change kind should be ChangeAdd got %s", change.Kind)
	}
}

func TestDiffSize(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.CreateReadWrite("1", "", nil); err != nil {
		t.Fatal(err)
	}

	diffPath, err := driverGet(d, "1", "")
	if err != nil {
		t.Fatal(err)
	}

	// Add a file to the diff path with a fixed size
	size := int64(1024)

	f, err := os.Create(path.Join(diffPath, "test_file"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	s, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	size = s.Size()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	diffSize, err := d.DiffSize("1", "")
	if err != nil {
		t.Fatal(err)
	}
	if diffSize != size {
		t.Fatalf("Expected size to be %d got %d", size, diffSize)
	}
}

func TestChildDiffSize(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.CreateReadWrite("1", "", nil); err != nil {
		t.Fatal(err)
	}

	diffPath, err := driverGet(d, "1", "")
	if err != nil {
		t.Fatal(err)
	}

	// Add a file to the diff path with a fixed size
	size := int64(1024)

	f, err := os.Create(path.Join(diffPath, "test_file"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	s, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	size = s.Size()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	diffSize, err := d.DiffSize("1", "")
	if err != nil {
		t.Fatal(err)
	}
	if diffSize != size {
		t.Fatalf("Expected size to be %d got %d", size, diffSize)
	}

	if err := d.Create("2", "1", nil); err != nil {
		t.Fatal(err)
	}

	diffSize, err = d.DiffSize("2", "1")
	if err != nil {
		t.Fatal(err)
	}
	// The diff size for the child should be zero
	if diffSize != 0 {
		t.Fatalf("Expected size to be %d got %d", 0, diffSize)
	}
}

func TestExists(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.Create("1", "", nil); err != nil {
		t.Fatal(err)
	}

	if d.Exists("none") {
		t.Fatal("id none should not exist in the driver")
	}

	if !d.Exists("1") {
		t.Fatal("id 1 should exist in the driver")
	}
}

func TestStatus(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.Create("1", "", nil); err != nil {
		t.Fatal(err)
	}

	status := d.Status()
	assert.Check(t, is.Len(status, 4))

	rootDir := status[0]
	dirs := status[2]
	if rootDir[0] != "Root Dir" {
		t.Fatalf("Expected Root Dir got %s", rootDir[0])
	}
	if rootDir[1] != d.rootPath() {
		t.Fatalf("Expected %s got %s", d.rootPath(), rootDir[1])
	}
	if dirs[0] != "Dirs" {
		t.Fatalf("Expected Dirs got %s", dirs[0])
	}
	if dirs[1] != "1" {
		t.Fatalf("Expected 1 got %s", dirs[1])
	}
}

func TestApplyDiff(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.CreateReadWrite("1", "", nil); err != nil {
		t.Fatal(err)
	}

	diffPath, err := driverGet(d, "1", "")
	if err != nil {
		t.Fatal(err)
	}

	// Add a file to the diff path with a fixed size
	size := int64(1024)

	f, err := os.Create(path.Join(diffPath, "test_file"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	f.Close()

	diff, err := d.Diff("1", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.Create("2", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.Create("3", "2", nil); err != nil {
		t.Fatal(err)
	}

	if err := d.applyDiff("3", diff); err != nil {
		t.Fatal(err)
	}

	// Ensure that the file is in the mount point for id 3

	mountPoint, err := driverGet(d, "3", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path.Join(mountPoint, "test_file")); err != nil {
		t.Fatal(err)
	}
}

func hash(c string) string {
	h := sha256.New()
	fmt.Fprint(h, c)
	return hex.EncodeToString(h.Sum(nil))
}

func testMountMoreThan42Layers(t *testing.T, mountPath string) {
	if err := os.MkdirAll(mountPath, 0755); err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(mountPath)
	d := testInit(mountPath, t).(*Driver)
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

		err := d.CreateReadWrite(current, parent, nil)
		assert.NilError(t, err, "current layer %d", i)

		point, err := driverGet(d, current, "")
		assert.NilError(t, err, "current layer %d", i)

		f, err := os.Create(path.Join(point, current))
		assert.NilError(t, err, "current layer %d", i)
		f.Close()

		if i%10 == 0 {
			err := os.Remove(path.Join(point, parent))
			assert.NilError(t, err, "current layer %d", i)
			expected--
		}
		last = current
	}

	// Perform the actual mount for the top most image
	point, err := driverGet(d, last, "")
	assert.NilError(t, err)
	files, err := os.ReadDir(point)
	assert.NilError(t, err)
	assert.Check(t, is.Len(files, expected))
}

func TestMountMoreThan42Layers(t *testing.T) {
	defer os.RemoveAll(tmpOuter)
	testMountMoreThan42Layers(t, tmp)
}

func TestMountMoreThan42LayersMatchingPathLength(t *testing.T) {
	defer os.RemoveAll(tmpOuter)
	zeroes := "0"
	for {
		// This finds a mount path so that when combined into aufs mount options
		// 4096 byte boundary would be in between the paths or in permission
		// section. For '/tmp' it will use '/tmp/aufs-tests/00000000/aufs'
		mountPath := path.Join(tmpOuter, zeroes, "aufs")
		pathLength := 77 + len(mountPath)

		if mod := 4095 % pathLength; mod == 0 || mod > pathLength-2 {
			t.Logf("Using path: %s", mountPath)
			testMountMoreThan42Layers(t, mountPath)
			return
		}
		zeroes += "0"
	}
}

func BenchmarkConcurrentAccess(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()

	d := newDriver(b)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	numConcurrent := 256
	// create a bunch of ids
	ids := make([]string, numConcurrent)
	for i := 0; i < numConcurrent; i++ {
		ids[i] = stringid.GenerateRandomID()
	}

	if err := d.Create(ids[0], "", nil); err != nil {
		b.Fatal(err)
	}

	if err := d.Create(ids[1], ids[0], nil); err != nil {
		b.Fatal(err)
	}

	parent := ids[1]
	ids = ids[2:]

	chErr := make(chan error, numConcurrent)
	var outerGroup sync.WaitGroup
	outerGroup.Add(len(ids))
	b.StartTimer()

	// here's the actual bench
	for _, id := range ids {
		go func(id string) {
			defer outerGroup.Done()
			if err := d.Create(id, parent, nil); err != nil {
				b.Logf("Create %s failed", id)
				chErr <- err
				return
			}
			var innerGroup sync.WaitGroup
			for i := 0; i < b.N; i++ {
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
	b.StopTimer()
	close(chErr)
	for err := range chErr {
		if err != nil {
			b.Log(err)
			b.Fail()
		}
	}
}

func TestInitStaleCleanup(t *testing.T) {
	if err := os.MkdirAll(tmp, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	for _, d := range []string{"diff", "mnt"} {
		if err := os.MkdirAll(filepath.Join(tmp, d, "123-removing"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	testInit(tmp, t)
	for _, d := range []string{"diff", "mnt"} {
		if _, err := os.Stat(filepath.Join(tmp, d, "123-removing")); err == nil {
			t.Fatal("cleanup failed")
		}
	}
}
