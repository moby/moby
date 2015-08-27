package ploop

// A test suite, also serving as an example of how to use the package

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/dustin/go-humanize"
)

var (
	old_pwd  string
	test_dir string
	d        Ploop
	snap     string
)

const baseDelta = "root.hdd"

// abort is used when the tests after the current one
// can't be run as one of their prerequisite(s) failed
func abort(format string, args ...interface{}) {
	s := fmt.Sprintf("ABORT: "+format+"\n", args...)
	f := bufio.NewWriter(os.Stderr)
	f.Write([]byte(s))
	f.Flush()
	cleanup()
	os.Exit(1)
}

// Check for a fatal error, call abort() if it is
func chk(err error) {
	if err != nil {
		abort("%s", err)
	}
}

func prepare(dir string) {
	var err error

	old_pwd, err = os.Getwd()
	chk(err)

	test_dir, err = ioutil.TempDir(old_pwd, dir)
	chk(err)

	err = os.Chdir(test_dir)
	chk(err)

	SetVerboseLevel(NoStdout)
}

func TestPrepare(t *testing.T) {
	prepare("tmp-test")
}

func TestUUID(t *testing.T) {
	uuid, e := UUID()
	if e != nil {
		t.Errorf("UUID: %s", e)
	}

	t.Logf("Got uuid %s", uuid)
}

func create() {
	size := "384M"
	var p CreateParam

	s, e := humanize.ParseBytes(size)
	if e != nil {
		abort("humanize.ParseBytes: can't parse %s: %s", size, e)
	}
	p.Size = s / 1024
	p.File = baseDelta

	e = Create(&p)
	if e != nil {
		abort("Create: %s", e)
	}
}

func TestCreate(t *testing.T) {
	create()
}

func open() {
	var e error

	d, e = Open("DiskDescriptor.xml")
	if e != nil {
		abort("Open: %s", e)
	}
}

func TestOpen(t *testing.T) {
	open()
}

func TestMount(t *testing.T) {
	mnt := "mnt"

	e := os.Mkdir(mnt, 0755)
	chk(e)

	p := MountParam{Target: mnt}
	dev, e := d.Mount(&p)
	if e != nil {
		abort("Mount: %s", e)
	}

	t.Logf("Mounted; ploop device %s", dev)
}

func resize(t *testing.T, size string, offline bool) {
	if offline && testing.Short() {
		t.Skip("skipping offline resize test in short mode.")
	}
	s, e := humanize.ParseBytes(size)
	if e != nil {
		t.Fatalf("humanize.ParseBytes: can't parse %s: %s", size, e)
	}
	s = s / 1024

	e = d.Resize(s, offline)
	if e != nil {
		t.Fatalf("Resize to %s (%d bytes) failed: %s", size, s, e)
	}
}

func TestResizeOnlineShrink(t *testing.T) {
	resize(t, "256MB", false)
}

func TestResizeOnlineGrow(t *testing.T) {
	resize(t, "512MB", false)
}

func TestSnapshot(t *testing.T) {
	uuid, e := d.Snapshot()
	if e != nil {
		abort("Snapshot: %s", e)
	}

	t.Logf("Created online snapshot; uuid %s", uuid)
	snap = uuid
}

func copyFile(src, dst string) error {
	return exec.Command("cp", "-a", src, dst).Run()
}

func testReplace(t *testing.T) {
	var p ReplaceParam
	newDelta := baseDelta + ".new"
	e := copyFile(baseDelta, newDelta)
	if e != nil {
		t.Fatalf("copyFile: %s", e)
	}

	p.File = newDelta
	p.CurFile = baseDelta
	p.Flags = KeepName
	e = d.Replace(&p)
	if e != nil {
		t.Fatalf("Replace: %s", e)
	}
}

func TestReplaceOnline(t *testing.T) {
	testReplace(t)
}

func TestSwitchSnapshotOnline(t *testing.T) {
	e := d.SwitchSnapshot(snap)
	// should fail with E_PARAM
	if IsError(e, E_PARAM) {
		t.Logf("SwitchSnapshot: (online) good, expected error")
	} else {
		t.Fatalf("SwitchSnapshot: (should fail): %s", e)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	e := d.DeleteSnapshot(snap)
	if e != nil {
		t.Fatalf("DeleteSnapshot: %s", e)
	} else {
		t.Logf("Deleted snapshot %s", snap)
	}
}

func TestIsMounted1(t *testing.T) {
	m, e := d.IsMounted()
	if e != nil {
		t.Fatalf("IsMounted: %s", e)
	}
	if !m {
		t.Fatalf("IsMounted: unexpectedly returned false")
	}
}

func TestUmount(t *testing.T) {
	e := d.Umount()
	if e != nil {
		t.Fatalf("Umount: %s", e)
	}
}

func TestIsMounted2(t *testing.T) {
	m, e := d.IsMounted()
	if e != nil {
		t.Fatalf("IsMounted: %s", e)
	}
	if m {
		t.Fatalf("IsMounted: unexpectedly returned true")
	}
}

func TestUmountAgain(t *testing.T) {
	e := d.Umount()
	if IsNotMounted(e) {
		t.Logf("Umount: (not mounted) good, expected error")
	} else {
		t.Fatalf("Umount: %s", e)
	}
}

func TestResizeOfflineShrink(t *testing.T) {
	resize(t, "256MB", true)
}

func TestResizeOfflineGrow(t *testing.T) {
	resize(t, "512MB", true)
}

func TestResizeOfflineShrinkAgain(t *testing.T) {
	resize(t, "256MB", true)
}

func TestSnapshotOffline(t *testing.T) {
	uuid, e := d.Snapshot()
	if e != nil {
		t.Fatalf("Snapshot: %s", e)
	} else {
		t.Logf("Created offline snapshot; uuid %s", uuid)
	}

	snap = uuid
}

func TestReplaceOffline(t *testing.T) {
	testReplace(t)
}

func TestSwitchSnapshot(t *testing.T) {
	e := d.SwitchSnapshot(snap)
	if e != nil {
		t.Fatalf("SwitchSnapshot: %s", e)
	} else {
		t.Logf("Switched to snapshot %s", snap)
	}
}

func TestFSInfo(t *testing.T) {
	i, e := FSInfo("DiskDescriptor.xml")

	if e != nil {
		t.Errorf("FSInfo: %v", e)
	} else {
		bTotal := i.Blocks * i.BlockSize
		bAvail := i.BlocksFree * i.BlockSize
		bUsed := bTotal - bAvail

		iTotal := i.Inodes
		iAvail := i.InodesFree
		iUsed := iTotal - iAvail

		t.Logf("\n             Size       Used      Avail Use%%\n%7s %9s %10s %10s %3d%%\n%7s %9d %10d %10d %3d%%",
			"Blocks",
			humanize.Bytes(bTotal),
			humanize.Bytes(bUsed),
			humanize.Bytes(bAvail),
			100*bUsed/bTotal,
			"Inodes",
			iTotal,
			iUsed,
			iAvail,
			100*iUsed/iTotal)
		t.Logf("\nInode ratio: 1 inode per %s of disk space",
			humanize.Bytes(bTotal/iTotal))
	}
}

func TestImageInfo(t *testing.T) {
	i, e := d.ImageInfo()
	if e != nil {
		t.Errorf("ImageInfo: %v", e)
	} else {
		t.Logf("\n              Blocks  Blocksize       Size  Ver\n%20d %10d %10s %4d",
			i.Blocks, i.BlockSize,
			humanize.Bytes(512*i.Blocks),
			i.Version)
	}

}

func cleanup() {
	if d.dd != "" {
		if m, _ := d.IsMounted(); m {
			d.Umount()
		}
		d.Close()
	}
	if old_pwd != "" {
		os.Chdir(old_pwd)
	}
	if test_dir != "" {
		os.RemoveAll(test_dir)
	}
}

// TestCleanup is the last test, removing files created by previous tests
func TestCleanup(t *testing.T) {
	cleanup()
}

func BenchmarkMountUmount(b *testing.B) {
	b.StopTimer()
	prepare("tmp-bench")
	SetVerboseLevel(NoStdout)
	create()
	open()
	mnt := "mnt"
	e := os.Mkdir(mnt, 0755)
	chk(e)
	p := MountParam{Target: mnt, Readonly: true}

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, e := d.Mount(&p)
		if e != nil {
			b.Fatalf("Mount: %s", e)
		}
		e = d.Umount()
		if e != nil {
			b.Fatalf("Umount: %s", e)
		}
	}
	b.StopTimer()
	cleanup()
}

func BenchmarkIsMounted(b *testing.B) {
	b.StopTimer()
	prepare("tmp-bench")
	SetVerboseLevel(NoStdout)
	create()
	open()
	mnt := "mnt"
	e := os.Mkdir(mnt, 0755)
	chk(e)
	p := MountParam{Target: mnt, Readonly: true}
	_, e = d.Mount(&p)
	if e != nil {
		b.Fatalf("Mount: %s", e)
	}

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, e := d.IsMounted()
		if e != nil {
			b.Fatalf("IsMounted: %s", e)
		}
	}
	b.StopTimer()
	cleanup()
}

func BenchmarkFSInfo(b *testing.B) {
	b.StopTimer()
	prepare("tmp-bench")
	SetVerboseLevel(NoStdout)
	create()
	open()
	mnt := "mnt"
	e := os.Mkdir(mnt, 0755)
	chk(e)
	p := MountParam{Target: mnt, Readonly: true}
	_, e = d.Mount(&p)
	if e != nil {
		b.Fatalf("Mount: %s", e)
	}

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, e := FSInfo("DiskDescriptor.xml")
		if e != nil {
			b.Fatalf("FSInfo: %s", e)
		}
	}
	b.StopTimer()
	cleanup()
}

func BenchmarkImageInfo(b *testing.B) {
	b.StopTimer()
	prepare("tmp-bench")
	SetVerboseLevel(NoStdout)
	create()
	open()
	mnt := "mnt"
	e := os.Mkdir(mnt, 0755)
	chk(e)
	p := MountParam{Target: mnt, Readonly: true}
	_, e = d.Mount(&p)
	if e != nil {
		b.Fatalf("Mount: %s", e)
	}

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, e := d.ImageInfo()
		if e != nil {
			b.Fatalf("ImageInfo: %s", e)
		}
	}
	b.StopTimer()
	cleanup()
}
