package local

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/mount"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestRemove(c *check.C) {
	// TODO Windows: Investigate why this test fails on Windows under CI
	//               but passes locally.
	if runtime.GOOS == "windows" {
		c.Skip("Test failing on Windows CI")
	}
	rootDir, err := ioutil.TempDir("", "local-volume-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	r, err := New(rootDir, 0, 0)
	if err != nil {
		c.Fatal(err)
	}

	vol, err := r.Create("testing", nil)
	if err != nil {
		c.Fatal(err)
	}

	if err := r.Remove(vol); err != nil {
		c.Fatal(err)
	}

	vol, err = r.Create("testing2", nil)
	if err != nil {
		c.Fatal(err)
	}
	if err := os.RemoveAll(vol.Path()); err != nil {
		c.Fatal(err)
	}

	if err := r.Remove(vol); err != nil {
		c.Fatal(err)
	}

	if _, err := os.Stat(vol.Path()); err != nil && !os.IsNotExist(err) {
		c.Fatal("volume dir not removed")
	}

	if l, _ := r.List(); len(l) != 0 {
		c.Fatal("expected there to be no volumes")
	}
}

func (s *DockerSuite) TestInitializeWithVolumes(c *check.C) {
	rootDir, err := ioutil.TempDir("", "local-volume-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	r, err := New(rootDir, 0, 0)
	if err != nil {
		c.Fatal(err)
	}

	vol, err := r.Create("testing", nil)
	if err != nil {
		c.Fatal(err)
	}

	r, err = New(rootDir, 0, 0)
	if err != nil {
		c.Fatal(err)
	}

	v, err := r.Get(vol.Name())
	if err != nil {
		c.Fatal(err)
	}

	if v.Path() != vol.Path() {
		c.Fatal("expected to re-initialize root with existing volumes")
	}
}

func (s *DockerSuite) TestCreate(c *check.C) {
	rootDir, err := ioutil.TempDir("", "local-volume-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	r, err := New(rootDir, 0, 0)
	if err != nil {
		c.Fatal(err)
	}

	cases := map[string]bool{
		"name":                  true,
		"name-with-dash":        true,
		"name_with_underscore":  true,
		"name/with/slash":       false,
		"name/with/../../slash": false,
		"./name":                false,
		"../name":               false,
		"./":                    false,
		"../":                   false,
		"~":                     false,
		".":                     false,
		"..":                    false,
		"...":                   false,
	}

	for name, success := range cases {
		v, err := r.Create(name, nil)
		if success {
			if err != nil {
				c.Fatal(err)
			}
			if v.Name() != name {
				c.Fatalf("Expected volume with name %s, got %s", name, v.Name())
			}
		} else {
			if err == nil {
				c.Fatalf("Expected error creating volume with name %s, got nil", name)
			}
		}
	}

	r, err = New(rootDir, 0, 0)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestValidateName(c *check.C) {
	r := &Root{}
	names := map[string]bool{
		"x":           false,
		"/testvol":    false,
		"thing.d":     true,
		"hello-world": true,
		"./hello":     false,
		".hello":      false,
	}

	for vol, expected := range names {
		err := r.validateName(vol)
		if expected && err != nil {
			c.Fatalf("expected %s to be valid got %v", vol, err)
		}
		if !expected && err == nil {
			c.Fatalf("expected %s to be invalid", vol)
		}
	}
}

func (s *DockerSuite) TestCreateWithOpts(c *check.C) {
	if runtime.GOOS == "windows" {
		c.Skip("Invalid test on Windows")
	}

	rootDir, err := ioutil.TempDir("", "local-volume-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	r, err := New(rootDir, 0, 0)
	if err != nil {
		c.Fatal(err)
	}

	if _, err := r.Create("test", map[string]string{"invalidopt": "notsupported"}); err == nil {
		c.Fatal("expected invalid opt to cause error")
	}

	vol, err := r.Create("test", map[string]string{"device": "tmpfs", "type": "tmpfs", "o": "size=1m,uid=1000"})
	if err != nil {
		c.Fatal(err)
	}
	v := vol.(*localVolume)

	dir, err := v.Mount("1234")
	if err != nil {
		c.Fatal(err)
	}
	defer func() {
		if err := v.Unmount("1234"); err != nil {
			c.Fatal(err)
		}
	}()

	mountInfos, err := mount.GetMounts()
	if err != nil {
		c.Fatal(err)
	}

	var found bool
	for _, info := range mountInfos {
		if info.Mountpoint == dir {
			found = true
			if info.Fstype != "tmpfs" {
				c.Fatalf("expected tmpfs mount, got %q", info.Fstype)
			}
			if info.Source != "tmpfs" {
				c.Fatalf("expected tmpfs mount, got %q", info.Source)
			}
			if !strings.Contains(info.VfsOpts, "uid=1000") {
				c.Fatalf("expected mount info to have uid=1000: %q", info.VfsOpts)
			}
			if !strings.Contains(info.VfsOpts, "size=1024k") {
				c.Fatalf("expected mount info to have size=1024k: %q", info.VfsOpts)
			}
			break
		}
	}

	if !found {
		c.Fatal("mount not found")
	}

	if v.active.count != 1 {
		c.Fatalf("Expected active mount count to be 1, got %d", v.active.count)
	}

	// test double mount
	if _, err := v.Mount("1234"); err != nil {
		c.Fatal(err)
	}
	if v.active.count != 2 {
		c.Fatalf("Expected active mount count to be 2, got %d", v.active.count)
	}

	if err := v.Unmount("1234"); err != nil {
		c.Fatal(err)
	}
	if v.active.count != 1 {
		c.Fatalf("Expected active mount count to be 1, got %d", v.active.count)
	}

	mounted, err := mount.Mounted(v.path)
	if err != nil {
		c.Fatal(err)
	}
	if !mounted {
		c.Fatal("expected mount to still be active")
	}

	r, err = New(rootDir, 0, 0)
	if err != nil {
		c.Fatal(err)
	}

	v2, exists := r.volumes["test"]
	if !exists {
		c.Fatal("missing volume on restart")
	}

	if !reflect.DeepEqual(v.opts, v2.opts) {
		c.Fatal("missing volume options on restart")
	}
}

func (s *DockerSuite) TestRealodNoOpts(c *check.C) {
	rootDir, err := ioutil.TempDir("", "volume-test-reload-no-opts")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	r, err := New(rootDir, 0, 0)
	if err != nil {
		c.Fatal(err)
	}

	if _, err := r.Create("test1", nil); err != nil {
		c.Fatal(err)
	}
	if _, err := r.Create("test2", nil); err != nil {
		c.Fatal(err)
	}
	// make sure a file with `null` (.e.g. empty opts map from older daemon) is ok
	if err := ioutil.WriteFile(filepath.Join(rootDir, "test2"), []byte("null"), 600); err != nil {
		c.Fatal(err)
	}

	if _, err := r.Create("test3", nil); err != nil {
		c.Fatal(err)
	}
	// make sure an empty opts file doesn't break us too
	if err := ioutil.WriteFile(filepath.Join(rootDir, "test3"), nil, 600); err != nil {
		c.Fatal(err)
	}

	if _, err := r.Create("test4", map[string]string{}); err != nil {
		c.Fatal(err)
	}

	r, err = New(rootDir, 0, 0)
	if err != nil {
		c.Fatal(err)
	}

	for _, name := range []string{"test1", "test2", "test3", "test4"} {
		v, err := r.Get(name)
		if err != nil {
			c.Fatal(err)
		}
		lv, ok := v.(*localVolume)
		if !ok {
			c.Fatalf("expected *localVolume got: %v", reflect.TypeOf(v))
		}
		if lv.opts != nil {
			c.Fatalf("expected opts to be nil, got: %v", lv.opts)
		}
		if _, err := lv.Mount("1234"); err != nil {
			c.Fatal(err)
		}
	}
}
