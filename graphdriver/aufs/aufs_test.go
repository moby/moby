package aufs

import (
	"github.com/dotcloud/docker/archive"
	"os"
	"path"
	"testing"
)

var (
	tmp = path.Join(os.TempDir(), "aufs-tests", "aufs")
)

func newDriver(t *testing.T) *Driver {
	if err := os.MkdirAll(tmp, 0755); err != nil {
		t.Fatal(err)
	}

	d, err := Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return d.(*Driver)
}

func TestNewDriver(t *testing.T) {
	if err := os.MkdirAll(tmp, 0755); err != nil {
		t.Fatal(err)
	}

	d, err := Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	if d == nil {
		t.Fatalf("Driver should not be nil")
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

	if _, err := Init(tmp); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(tmp); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(tmp)
}

func TestCreateNewDir(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
}

func TestCreateNewDirStructure(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", ""); err != nil {
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

	if err := d.Create("1", ""); err != nil {
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
			t.Fatalf("Error should not be nil because dirs with id 1 should be delted: %s", p)
		}
	}
}

func TestGetWithoutParent(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	diffPath, err := d.Get("1")
	if err != nil {
		t.Fatal(err)
	}
	expected := path.Join(tmp, "diff", "1")
	if diffPath != expected {
		t.Fatalf("Expected path %s got %s", expected, diffPath)
	}
}

func TestCleanupWithNoDirs(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupWithDir(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	if err := d.Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestMountedFalseResponse(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	response, err := d.mounted("1")
	if err != nil {
		t.Fatal(err)
	}

	if response != false {
		t.Fatalf("Response if dir id 1 is mounted should be false")
	}
}

func TestMountedTrueReponse(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.Create("2", "1"); err != nil {
		t.Fatal(err)
	}

	_, err := d.Get("2")
	if err != nil {
		t.Fatal(err)
	}

	response, err := d.mounted("2")
	if err != nil {
		t.Fatal(err)
	}

	if response != true {
		t.Fatalf("Response if dir id 2 is mounted should be true")
	}
}

func TestMountWithParent(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.Create("2", "1"); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := d.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	mntPath, err := d.Get("2")
	if err != nil {
		t.Fatal(err)
	}
	if mntPath == "" {
		t.Fatal("mntPath should not be empty string")
	}

	expected := path.Join(tmp, "mnt", "2")
	if mntPath != expected {
		t.Fatalf("Expected %s got %s", expected, mntPath)
	}
}

func TestRemoveMountedDir(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.Create("2", "1"); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := d.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	mntPath, err := d.Get("2")
	if err != nil {
		t.Fatal(err)
	}
	if mntPath == "" {
		t.Fatal("mntPath should not be empty string")
	}

	mounted, err := d.mounted("2")
	if err != nil {
		t.Fatal(err)
	}

	if !mounted {
		t.Fatalf("Dir id 2 should be mounted")
	}

	if err := d.Remove("2"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateWithInvalidParent(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", "docker"); err == nil {
		t.Fatalf("Error should not be nil with parent does not exist")
	}
}

func TestGetDiff(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	diffPath, err := d.Get("1")
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

	a, err := d.Diff("1")
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatalf("Archive should not be nil")
	}
}

func TestChanges(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.Create("2", "1"); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := d.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	mntPoint, err := d.Get("2")
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

	changes, err := d.Changes("2")
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

	if err := d.Create("3", "2"); err != nil {
		t.Fatal(err)
	}
	mntPoint, err = d.Get("3")
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

	changes, err = d.Changes("3")
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

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	diffPath, err := d.Get("1")
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

	diffSize, err := d.DiffSize("1")
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

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	diffPath, err := d.Get("1")
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

	diffSize, err := d.DiffSize("1")
	if err != nil {
		t.Fatal(err)
	}
	if diffSize != size {
		t.Fatalf("Expected size to be %d got %d", size, diffSize)
	}

	if err := d.Create("2", "1"); err != nil {
		t.Fatal(err)
	}

	diffSize, err = d.DiffSize("2")
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

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	if d.Exists("none") {
		t.Fatal("id name should not exist in the driver")
	}

	if !d.Exists("1") {
		t.Fatal("id 1 should exist in the driver")
	}
}

func TestStatus(t *testing.T) {
	d := newDriver(t)
	defer os.RemoveAll(tmp)
	defer d.Cleanup()

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	status := d.Status()
	if status == nil || len(status) == 0 {
		t.Fatal("Status should not be nil or empty")
	}
	rootDir := status[0]
	dirs := status[1]
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

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	diffPath, err := d.Get("1")
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

	diff, err := d.Diff("1")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.Create("2", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.Create("3", "2"); err != nil {
		t.Fatal(err)
	}

	if err := d.ApplyDiff("3", diff); err != nil {
		t.Fatal(err)
	}

	// Ensure that the file is in the mount point for id 3

	mountPoint, err := d.Get("3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path.Join(mountPoint, "test_file")); err != nil {
		t.Fatal(err)
	}
}
