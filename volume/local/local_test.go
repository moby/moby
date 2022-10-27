package local // import "github.com/docker/docker/volume/local"

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/sys/mountinfo"
	"gotest.tools/v3/skip"
)

func TestGetAddress(t *testing.T) {
	cases := map[string]string{
		"addr=11.11.11.1":   "11.11.11.1",
		" ":                 "",
		"addr=":             "",
		"addr=2001:db8::68": "2001:db8::68",
	}
	for name, success := range cases {
		v := getAddress(name)
		if v != success {
			t.Errorf("Test case failed for %s actual: %s expected : %s", name, v, success)
		}
	}
}

func TestGetPassword(t *testing.T) {
	cases := map[string]string{
		"password=secret":                       "secret",
		" ":                                     "",
		"password=":                             "",
		"password=Tr0ub4dor&3":                  "Tr0ub4dor&3",
		"password=correcthorsebatterystaple":    "correcthorsebatterystaple",
		"username=moby,password=secret":         "secret",
		"username=moby,password=secret,addr=11": "secret",
		"username=moby,addr=11":                 "",
	}
	for optsstring, success := range cases {
		v := getPassword(optsstring)
		if v != success {
			t.Errorf("Test case failed for %s actual: %s expected : %s", optsstring, v, success)
		}
	}
}

func TestRemove(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "FIXME: investigate why this test fails on CI")
	rootDir, err := os.MkdirTemp("", "local-volume-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	r, err := New(rootDir, idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
	}

	vol, err := r.Create("testing", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Remove(vol); err != nil {
		t.Fatal(err)
	}

	vol, err = r.Create("testing2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(vol.Path()); err != nil {
		t.Fatal(err)
	}

	if err := r.Remove(vol); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(vol.Path()); err != nil && !os.IsNotExist(err) {
		t.Fatal("volume dir not removed")
	}

	if l, _ := r.List(); len(l) != 0 {
		t.Fatal("expected there to be no volumes")
	}
}

func TestInitializeWithVolumes(t *testing.T) {
	rootDir, err := os.MkdirTemp("", "local-volume-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	r, err := New(rootDir, idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
	}

	vol, err := r.Create("testing", nil)
	if err != nil {
		t.Fatal(err)
	}

	r, err = New(rootDir, idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
	}

	v, err := r.Get(vol.Name())
	if err != nil {
		t.Fatal(err)
	}

	if v.Path() != vol.Path() {
		t.Fatal("expected to re-initialize root with existing volumes")
	}
}

func TestCreate(t *testing.T) {
	rootDir, err := os.MkdirTemp("", "local-volume-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	r, err := New(rootDir, idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
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
				t.Fatal(err)
			}
			if v.Name() != name {
				t.Fatalf("Expected volume with name %s, got %s", name, v.Name())
			}
		} else {
			if err == nil {
				t.Fatalf("Expected error creating volume with name %s, got nil", name)
			}
		}
	}

	_, err = New(rootDir, idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateName(t *testing.T) {
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
			t.Fatalf("expected %s to be valid got %v", vol, err)
		}
		if !expected && err == nil {
			t.Fatalf("expected %s to be invalid", vol)
		}
	}
}

func TestCreateWithOpts(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows")
	skip.If(t, os.Getuid() != 0, "requires mounts")
	rootDir, err := os.MkdirTemp("", "local-volume-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	r, err := New(rootDir, idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.Create("test", map[string]string{"invalidopt": "notsupported"}); err == nil {
		t.Fatal("expected invalid opt to cause error")
	}

	vol, err := r.Create("test", map[string]string{"device": "tmpfs", "type": "tmpfs", "o": "size=1m,uid=1000"})
	if err != nil {
		t.Fatal(err)
	}
	v := vol.(*localVolume)

	dir, err := v.Mount("1234")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := v.Unmount("1234"); err != nil {
			t.Fatal(err)
		}
	}()

	mountInfos, err := mountinfo.GetMounts(mountinfo.SingleEntryFilter(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(mountInfos) != 1 {
		t.Fatalf("expected 1 mount, found %d: %+v", len(mountInfos), mountInfos)
	}

	info := mountInfos[0]
	t.Logf("%+v", info)
	if info.FSType != "tmpfs" {
		t.Fatalf("expected tmpfs mount, got %q", info.FSType)
	}
	if info.Source != "tmpfs" {
		t.Fatalf("expected tmpfs mount, got %q", info.Source)
	}
	if !strings.Contains(info.VFSOptions, "uid=1000") {
		t.Fatalf("expected mount info to have uid=1000: %q", info.VFSOptions)
	}
	if !strings.Contains(info.VFSOptions, "size=1024k") {
		t.Fatalf("expected mount info to have size=1024k: %q", info.VFSOptions)
	}

	if v.active.count != 1 {
		t.Fatalf("Expected active mount count to be 1, got %d", v.active.count)
	}

	// test double mount
	if _, err := v.Mount("1234"); err != nil {
		t.Fatal(err)
	}
	if v.active.count != 2 {
		t.Fatalf("Expected active mount count to be 2, got %d", v.active.count)
	}

	if err := v.Unmount("1234"); err != nil {
		t.Fatal(err)
	}
	if v.active.count != 1 {
		t.Fatalf("Expected active mount count to be 1, got %d", v.active.count)
	}

	mounted, err := mountinfo.Mounted(v.path)
	if err != nil {
		t.Fatal(err)
	}
	if !mounted {
		t.Fatal("expected mount to still be active")
	}

	r, err = New(rootDir, idtools.Identity{UID: 0, GID: 0})
	if err != nil {
		t.Fatal(err)
	}

	v2, exists := r.volumes["test"]
	if !exists {
		t.Fatal("missing volume on restart")
	}

	if !reflect.DeepEqual(v.opts, v2.opts) {
		t.Fatal("missing volume options on restart")
	}
}

func TestRelaodNoOpts(t *testing.T) {
	rootDir, err := os.MkdirTemp("", "volume-test-reload-no-opts")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	r, err := New(rootDir, idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.Create("test1", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create("test2", nil); err != nil {
		t.Fatal(err)
	}
	// make sure a file with `null` (.e.g. empty opts map from older daemon) is ok
	if err := os.WriteFile(filepath.Join(rootDir, "test2"), []byte("null"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Create("test3", nil); err != nil {
		t.Fatal(err)
	}
	// make sure an empty opts file doesn't break us too
	if err := os.WriteFile(filepath.Join(rootDir, "test3"), nil, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Create("test4", map[string]string{}); err != nil {
		t.Fatal(err)
	}

	r, err = New(rootDir, idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"test1", "test2", "test3", "test4"} {
		v, err := r.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		lv, ok := v.(*localVolume)
		if !ok {
			t.Fatalf("expected *localVolume got: %v", reflect.TypeOf(v))
		}
		if lv.opts != nil {
			t.Fatalf("expected opts to be nil, got: %v", lv.opts)
		}
		if _, err := lv.Mount("1234"); err != nil {
			t.Fatal(err)
		}
	}
}
