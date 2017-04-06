// +build linux freebsd

package graphtest

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"syscall"
	"testing"

	"github.com/docker/docker/daemon/graphdriver"
)

// InitLoopbacks ensures that the loopback devices are properly created within
// the system running the device mapper tests.
func InitLoopbacks() error {
	statT, err := getBaseLoopStats()
	if err != nil {
		return err
	}
	// create at least 8 loopback files, ya, that is a good number
	for i := 0; i < 8; i++ {
		loopPath := fmt.Sprintf("/dev/loop%d", i)
		// only create new loopback files if they don't exist
		if _, err := os.Stat(loopPath); err != nil {
			if mkerr := syscall.Mknod(loopPath,
				uint32(statT.Mode|syscall.S_IFBLK), int((7<<8)|(i&0xff)|((i&0xfff00)<<12))); mkerr != nil {
				return mkerr
			}
			os.Chown(loopPath, int(statT.Uid), int(statT.Gid))
		}
	}
	return nil
}

// getBaseLoopStats inspects /dev/loop0 to collect uid,gid, and mode for the
// loop0 device on the system.  If it does not exist we assume 0,0,0660 for the
// stat data
func getBaseLoopStats() (*syscall.Stat_t, error) {
	loop0, err := os.Stat("/dev/loop0")
	if err != nil {
		if os.IsNotExist(err) {
			return &syscall.Stat_t{
				Uid:  0,
				Gid:  0,
				Mode: 0660,
			}, nil
		}
		return nil, err
	}
	return loop0.Sys().(*syscall.Stat_t), nil
}

func verifyFile(t testing.TB, path string, mode os.FileMode, uid, gid uint32) {
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if fi.Mode()&os.ModeType != mode&os.ModeType {
		t.Fatalf("Expected %s type 0x%x, got 0x%x", path, mode&os.ModeType, fi.Mode()&os.ModeType)
	}

	if fi.Mode()&os.ModePerm != mode&os.ModePerm {
		t.Fatalf("Expected %s mode %o, got %o", path, mode&os.ModePerm, fi.Mode()&os.ModePerm)
	}

	if fi.Mode()&os.ModeSticky != mode&os.ModeSticky {
		t.Fatalf("Expected %s sticky 0x%x, got 0x%x", path, mode&os.ModeSticky, fi.Mode()&os.ModeSticky)
	}

	if fi.Mode()&os.ModeSetuid != mode&os.ModeSetuid {
		t.Fatalf("Expected %s setuid 0x%x, got 0x%x", path, mode&os.ModeSetuid, fi.Mode()&os.ModeSetuid)
	}

	if fi.Mode()&os.ModeSetgid != mode&os.ModeSetgid {
		t.Fatalf("Expected %s setgid 0x%x, got 0x%x", path, mode&os.ModeSetgid, fi.Mode()&os.ModeSetgid)
	}

	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		if stat.Uid != uid {
			t.Fatalf("%s no owned by uid %d", path, uid)
		}
		if stat.Gid != gid {
			t.Fatalf("%s not owned by gid %d", path, gid)
		}
	}
}

func createBase(t testing.TB, driver graphdriver.Driver, name string) {
	// We need to be able to set any perms
	oldmask := syscall.Umask(0)
	defer syscall.Umask(oldmask)

	if err := driver.CreateReadWrite(name, "", nil); err != nil {
		t.Fatal(err)
	}

	dir, err := driver.Get(name, "")
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Put(name)

	subdir := path.Join(dir, "a subdir")
	if err := os.Mkdir(subdir, 0705|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(subdir, 1, 2); err != nil {
		t.Fatal(err)
	}

	file := path.Join(dir, "a file")
	if err := ioutil.WriteFile(file, []byte("Some data"), 0222|os.ModeSetuid); err != nil {
		t.Fatal(err)
	}
}

func verifyBase(t testing.TB, driver graphdriver.Driver, name string) {
	dir, err := driver.Get(name, "")
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Put(name)

	subdir := path.Join(dir, "a subdir")
	verifyFile(t, subdir, 0705|os.ModeDir|os.ModeSticky, 1, 2)

	file := path.Join(dir, "a file")
	verifyFile(t, file, 0222|os.ModeSetuid, 0, 0)

	fis, err := readDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(fis) != 2 {
		t.Fatal("Unexpected files in base image")
	}

}
