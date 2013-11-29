package devmapper

import (
	"fmt"
	"github.com/dotcloud/docker/graphdriver"
	"io/ioutil"
	"path"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func init() {
	// Reduce the size the the base fs and loopback for the tests
	DefaultDataLoopbackSize = 300 * 1024 * 1024
	DefaultMetaDataLoopbackSize = 200 * 1024 * 1024
	DefaultBaseFsSize = 300 * 1024 * 1024
}

// denyAllDevmapper mocks all calls to libdevmapper in the unit tests, and denies them by default
func denyAllDevmapper() {
	// Hijack all calls to libdevmapper with default panics.
	// Authorized calls are selectively hijacked in each tests.
	DmTaskCreate = func(t int) *CDmTask {
		panic("DmTaskCreate: this method should not be called here")
	}
	DmTaskRun = func(task *CDmTask) int {
		panic("DmTaskRun: this method should not be called here")
	}
	DmTaskSetName = func(task *CDmTask, name string) int {
		panic("DmTaskSetName: this method should not be called here")
	}
	DmTaskSetMessage = func(task *CDmTask, message string) int {
		panic("DmTaskSetMessage: this method should not be called here")
	}
	DmTaskSetSector = func(task *CDmTask, sector uint64) int {
		panic("DmTaskSetSector: this method should not be called here")
	}
	DmTaskSetCookie = func(task *CDmTask, cookie *uint, flags uint16) int {
		panic("DmTaskSetCookie: this method should not be called here")
	}
	DmTaskSetAddNode = func(task *CDmTask, addNode AddNodeType) int {
		panic("DmTaskSetAddNode: this method should not be called here")
	}
	DmTaskSetRo = func(task *CDmTask) int {
		panic("DmTaskSetRo: this method should not be called here")
	}
	DmTaskAddTarget = func(task *CDmTask, start, size uint64, ttype, params string) int {
		panic("DmTaskAddTarget: this method should not be called here")
	}
	DmTaskGetInfo = func(task *CDmTask, info *Info) int {
		panic("DmTaskGetInfo: this method should not be called here")
	}
	DmGetNextTarget = func(task *CDmTask, next uintptr, start, length *uint64, target, params *string) uintptr {
		panic("DmGetNextTarget: this method should not be called here")
	}
	DmUdevWait = func(cookie uint) int {
		panic("DmUdevWait: this method should not be called here")
	}
	DmSetDevDir = func(dir string) int {
		panic("DmSetDevDir: this method should not be called here")
	}
	DmGetLibraryVersion = func(version *string) int {
		panic("DmGetLibraryVersion: this method should not be called here")
	}
	DmLogInitVerbose = func(level int) {
		panic("DmLogInitVerbose: this method should not be called here")
	}
	DmTaskDestroy = func(task *CDmTask) {
		panic("DmTaskDestroy: this method should not be called here")
	}
	LogWithErrnoInit = func() {
		panic("LogWithErrnoInit: this method should not be called here")
	}
}

func denyAllSyscall() {
	sysMount = func(source, target, fstype string, flags uintptr, data string) (err error) {
		panic("sysMount: this method should not be called here")
	}
	sysUnmount = func(target string, flags int) (err error) {
		panic("sysUnmount: this method should not be called here")
	}
	sysCloseOnExec = func(fd int) {
		panic("sysCloseOnExec: this method should not be called here")
	}
	sysSyscall = func(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno) {
		panic("sysSyscall: this method should not be called here")
	}
	// Not a syscall, but forbidding it here anyway
	Mounted = func(mnt string) (bool, error) {
		panic("devmapper.Mounted: this method should not be called here")
	}
	// osOpenFile = os.OpenFile
	// osNewFile = os.NewFile
	// osCreate = os.Create
	// osStat = os.Stat
	// osIsNotExist = os.IsNotExist
	// osIsExist = os.IsExist
	// osMkdirAll = os.MkdirAll
	// osRemoveAll = os.RemoveAll
	// osRename = os.Rename
	// osReadlink = os.Readlink

	// execRun = func(name string, args ...string) error {
	// 	return exec.Command(name, args...).Run()
	// }
}

func mkTestDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir("", "docker-test-devmapper-")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func newDriver(t *testing.T) *Driver {
	home := mkTestDirectory(t)
	d, err := Init(home)
	if err != nil {
		t.Fatal(err)
	}
	return d.(*Driver)
}

func cleanup(d *Driver) {
	d.Cleanup()
	osRemoveAll(d.home)
}

type Set map[string]bool

func (r Set) Assert(t *testing.T, names ...string) {
	for _, key := range names {
		if _, exists := r[key]; !exists {
			t.Fatalf("Key not set: %s", key)
		}
		delete(r, key)
	}
	if len(r) != 0 {
		t.Fatalf("Unexpected keys: %v", r)
	}
}

func TestInit(t *testing.T) {
	var (
		calls        = make(Set)
		taskMessages = make(Set)
		taskTypes    = make(Set)
		home         = mkTestDirectory(t)
	)
	defer osRemoveAll(home)

	func() {
		denyAllDevmapper()
		DmSetDevDir = func(dir string) int {
			calls["DmSetDevDir"] = true
			expectedDir := "/dev"
			if dir != expectedDir {
				t.Fatalf("Wrong libdevmapper call\nExpected: DmSetDevDir(%v)\nReceived: DmSetDevDir(%v)\n", expectedDir, dir)
			}
			return 0
		}
		LogWithErrnoInit = func() {
			calls["DmLogWithErrnoInit"] = true
		}
		var task1 CDmTask
		DmTaskCreate = func(taskType int) *CDmTask {
			calls["DmTaskCreate"] = true
			taskTypes[fmt.Sprintf("%d", taskType)] = true
			return &task1
		}
		DmTaskSetName = func(task *CDmTask, name string) int {
			calls["DmTaskSetName"] = true
			expectedTask := &task1
			if task != expectedTask {
				t.Fatalf("Wrong libdevmapper call\nExpected: DmTaskSetName(%v)\nReceived: DmTaskSetName(%v)\n", expectedTask, task)
			}
			// FIXME: use Set.AssertRegexp()
			if !strings.HasPrefix(name, "docker-") && !strings.HasPrefix(name, "/dev/mapper/docker-") ||
				!strings.HasSuffix(name, "-pool") && !strings.HasSuffix(name, "-base") {
				t.Fatalf("Wrong libdevmapper call\nExpected: DmTaskSetName(%v)\nReceived: DmTaskSetName(%v)\n", "docker-...-pool", name)
			}
			return 1
		}
		DmTaskRun = func(task *CDmTask) int {
			calls["DmTaskRun"] = true
			expectedTask := &task1
			if task != expectedTask {
				t.Fatalf("Wrong libdevmapper call\nExpected: DmTaskRun(%v)\nReceived: DmTaskRun(%v)\n", expectedTask, task)
			}
			return 1
		}
		DmTaskGetInfo = func(task *CDmTask, info *Info) int {
			calls["DmTaskGetInfo"] = true
			expectedTask := &task1
			if task != expectedTask {
				t.Fatalf("Wrong libdevmapper call\nExpected: DmTaskGetInfo(%v)\nReceived: DmTaskGetInfo(%v)\n", expectedTask, task)
			}
			// This will crash if info is not dereferenceable
			info.Exists = 0
			return 1
		}
		DmTaskSetSector = func(task *CDmTask, sector uint64) int {
			calls["DmTaskSetSector"] = true
			expectedTask := &task1
			if task != expectedTask {
				t.Fatalf("Wrong libdevmapper call\nExpected: DmTaskSetSector(%v)\nReceived: DmTaskSetSector(%v)\n", expectedTask, task)
			}
			if expectedSector := uint64(0); sector != expectedSector {
				t.Fatalf("Wrong libdevmapper call to DmTaskSetSector\nExpected: %v\nReceived: %v\n", expectedSector, sector)
			}
			return 1
		}
		DmTaskSetMessage = func(task *CDmTask, message string) int {
			calls["DmTaskSetMessage"] = true
			expectedTask := &task1
			if task != expectedTask {
				t.Fatalf("Wrong libdevmapper call\nExpected: DmTaskSetSector(%v)\nReceived: DmTaskSetSector(%v)\n", expectedTask, task)
			}
			taskMessages[message] = true
			return 1
		}
		DmTaskDestroy = func(task *CDmTask) {
			calls["DmTaskDestroy"] = true
			expectedTask := &task1
			if task != expectedTask {
				t.Fatalf("Wrong libdevmapper call\nExpected: DmTaskDestroy(%v)\nReceived: DmTaskDestroy(%v)\n", expectedTask, task)
			}
		}
		DmTaskAddTarget = func(task *CDmTask, start, size uint64, ttype, params string) int {
			calls["DmTaskSetTarget"] = true
			expectedTask := &task1
			if task != expectedTask {
				t.Fatalf("Wrong libdevmapper call\nExpected: DmTaskDestroy(%v)\nReceived: DmTaskDestroy(%v)\n", expectedTask, task)
			}
			if start != 0 {
				t.Fatalf("Wrong start: %d != %d", start, 0)
			}
			if ttype != "thin" && ttype != "thin-pool" {
				t.Fatalf("Wrong ttype: %s", ttype)
			}
			// Quick smoke test
			if params == "" {
				t.Fatalf("Params should not be empty")
			}
			return 1
		}
		fakeCookie := uint(4321)
		DmTaskSetCookie = func(task *CDmTask, cookie *uint, flags uint16) int {
			calls["DmTaskSetCookie"] = true
			expectedTask := &task1
			if task != expectedTask {
				t.Fatalf("Wrong libdevmapper call\nExpected: DmTaskDestroy(%v)\nReceived: DmTaskDestroy(%v)\n", expectedTask, task)
			}
			if flags != 0 {
				t.Fatalf("Cookie flags should be 0 (not %x)", flags)
			}
			*cookie = fakeCookie
			return 1
		}
		DmUdevWait = func(cookie uint) int {
			calls["DmUdevWait"] = true
			if cookie != fakeCookie {
				t.Fatalf("Wrong cookie: %d != %d", cookie, fakeCookie)
			}
			return 1
		}
		DmTaskSetAddNode = func(task *CDmTask, addNode AddNodeType) int {
			if addNode != AddNodeOnCreate {
				t.Fatalf("Wrong AddNoteType: %v (expected %v)", addNode, AddNodeOnCreate)
			}
			calls["DmTaskSetAddNode"] = true
			return 1
		}
		execRun = func(name string, args ...string) error {
			calls["execRun"] = true
			if name != "mkfs.ext4" {
				t.Fatalf("Expected %s to be executed, not %s", "mkfs.ext4", name)
			}
			return nil
		}
		driver, err := Init(home)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := driver.Cleanup(); err != nil {
				t.Fatal(err)
			}
		}()
	}()
	// Put all tests in a funciton to make sure the garbage collection will
	// occur.

	// Call GC to cleanup runtime.Finalizers
	runtime.GC()

	calls.Assert(t,
		"DmSetDevDir",
		"DmLogWithErrnoInit",
		"DmTaskSetName",
		"DmTaskRun",
		"DmTaskGetInfo",
		"DmTaskDestroy",
		"execRun",
		"DmTaskCreate",
		"DmTaskSetTarget",
		"DmTaskSetCookie",
		"DmUdevWait",
		"DmTaskSetSector",
		"DmTaskSetMessage",
		"DmTaskSetAddNode",
	)
	taskTypes.Assert(t, "0", "6", "17")
	taskMessages.Assert(t, "create_thin 0", "set_transaction_id 0 1")
}

func fakeInit() func(home string) (graphdriver.Driver, error) {
	oldInit := Init
	Init = func(home string) (graphdriver.Driver, error) {
		return &Driver{
			home: home,
		}, nil
	}
	return oldInit
}

func restoreInit(init func(home string) (graphdriver.Driver, error)) {
	Init = init
}

func mockAllDevmapper(calls Set) {
	DmSetDevDir = func(dir string) int {
		calls["DmSetDevDir"] = true
		return 0
	}
	LogWithErrnoInit = func() {
		calls["DmLogWithErrnoInit"] = true
	}
	DmTaskCreate = func(taskType int) *CDmTask {
		calls["DmTaskCreate"] = true
		return &CDmTask{}
	}
	DmTaskSetName = func(task *CDmTask, name string) int {
		calls["DmTaskSetName"] = true
		return 1
	}
	DmTaskRun = func(task *CDmTask) int {
		calls["DmTaskRun"] = true
		return 1
	}
	DmTaskGetInfo = func(task *CDmTask, info *Info) int {
		calls["DmTaskGetInfo"] = true
		return 1
	}
	DmTaskSetSector = func(task *CDmTask, sector uint64) int {
		calls["DmTaskSetSector"] = true
		return 1
	}
	DmTaskSetMessage = func(task *CDmTask, message string) int {
		calls["DmTaskSetMessage"] = true
		return 1
	}
	DmTaskDestroy = func(task *CDmTask) {
		calls["DmTaskDestroy"] = true
	}
	DmTaskAddTarget = func(task *CDmTask, start, size uint64, ttype, params string) int {
		calls["DmTaskSetTarget"] = true
		return 1
	}
	DmTaskSetCookie = func(task *CDmTask, cookie *uint, flags uint16) int {
		calls["DmTaskSetCookie"] = true
		return 1
	}
	DmUdevWait = func(cookie uint) int {
		calls["DmUdevWait"] = true
		return 1
	}
	DmTaskSetAddNode = func(task *CDmTask, addNode AddNodeType) int {
		calls["DmTaskSetAddNode"] = true
		return 1
	}
	execRun = func(name string, args ...string) error {
		calls["execRun"] = true
		return nil
	}
}

func TestDriverName(t *testing.T) {
	denyAllDevmapper()
	defer denyAllDevmapper()

	oldInit := fakeInit()
	defer restoreInit(oldInit)

	d := newDriver(t)
	if d.String() != "devicemapper" {
		t.Fatalf("Expected driver name to be devicemapper got %s", d.String())
	}
}

func TestDriverCreate(t *testing.T) {
	denyAllDevmapper()
	denyAllSyscall()
	defer denyAllSyscall()
	defer denyAllDevmapper()

	calls := make(Set)
	mockAllDevmapper(calls)

	sysMount = func(source, target, fstype string, flags uintptr, data string) (err error) {
		calls["sysMount"] = true
		// FIXME: compare the exact source and target strings (inodes + devname)
		if expectedSource := "/dev/mapper/docker-"; !strings.HasPrefix(source, expectedSource) {
			t.Fatalf("Wrong syscall call\nExpected: Mount(%v)\nReceived: Mount(%v)\n", expectedSource, source)
		}
		if expectedTarget := "/tmp/docker-test-devmapper-"; !strings.HasPrefix(target, expectedTarget) {
			t.Fatalf("Wrong syscall call\nExpected: Mount(%v)\nReceived: Mount(%v)\n", expectedTarget, target)
		}
		if expectedFstype := "ext4"; fstype != expectedFstype {
			t.Fatalf("Wrong syscall call\nExpected: Mount(%v)\nReceived: Mount(%v)\n", expectedFstype, fstype)
		}
		if expectedFlags := uintptr(3236757504); flags != expectedFlags {
			t.Fatalf("Wrong syscall call\nExpected: Mount(%v)\nReceived: Mount(%v)\n", expectedFlags, flags)
		}
		return nil
	}

	Mounted = func(mnt string) (bool, error) {
		calls["Mounted"] = true
		if !strings.HasPrefix(mnt, "/tmp/docker-test-devmapper-") || !strings.HasSuffix(mnt, "/mnt/1") {
			t.Fatalf("Wrong mounted call\nExpected: Mounted(%v)\nReceived: Mounted(%v)\n", "/tmp/docker-test-devmapper-.../mnt/1", mnt)
		}
		return false, nil
	}

	sysSyscall = func(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno) {
		calls["sysSyscall"] = true
		if trap != sysSysIoctl {
			t.Fatalf("Unexpected syscall. Expecting SYS_IOCTL, received: %d", trap)
		}
		switch a2 {
		case LoopSetFd:
			calls["ioctl.loopsetfd"] = true
		case LoopCtlGetFree:
			calls["ioctl.loopctlgetfree"] = true
		case LoopGetStatus64:
			calls["ioctl.loopgetstatus"] = true
		case LoopSetStatus64:
			calls["ioctl.loopsetstatus"] = true
		case LoopClrFd:
			calls["ioctl.loopclrfd"] = true
		case LoopSetCapacity:
			calls["ioctl.loopsetcapacity"] = true
		case BlkGetSize64:
			calls["ioctl.blkgetsize"] = true
		default:
			t.Fatalf("Unexpected IOCTL. Received %d", a2)
		}
		return 0, 0, 0
	}

	func() {
		d := newDriver(t)

		calls.Assert(t,
			"DmSetDevDir",
			"DmLogWithErrnoInit",
			"DmTaskSetName",
			"DmTaskRun",
			"DmTaskGetInfo",
			"execRun",
			"DmTaskCreate",
			"DmTaskSetTarget",
			"DmTaskSetCookie",
			"DmUdevWait",
			"DmTaskSetSector",
			"DmTaskSetMessage",
			"DmTaskSetAddNode",
			"sysSyscall",
			"ioctl.blkgetsize",
			"ioctl.loopsetfd",
			"ioctl.loopsetstatus",
		)

		if err := d.Create("1", ""); err != nil {
			t.Fatal(err)
		}
		calls.Assert(t,
			"DmTaskCreate",
			"DmTaskGetInfo",
			"sysMount",
			"Mounted",
			"DmTaskRun",
			"DmTaskSetTarget",
			"DmTaskSetSector",
			"DmTaskSetCookie",
			"DmUdevWait",
			"DmTaskSetName",
			"DmTaskSetMessage",
			"DmTaskSetAddNode",
		)

	}()

	runtime.GC()

	calls.Assert(t,
		"DmTaskDestroy",
	)
}

func TestDriverRemove(t *testing.T) {
	denyAllDevmapper()
	denyAllSyscall()
	defer denyAllSyscall()
	defer denyAllDevmapper()

	calls := make(Set)
	mockAllDevmapper(calls)

	sysMount = func(source, target, fstype string, flags uintptr, data string) (err error) {
		calls["sysMount"] = true
		// FIXME: compare the exact source and target strings (inodes + devname)
		if expectedSource := "/dev/mapper/docker-"; !strings.HasPrefix(source, expectedSource) {
			t.Fatalf("Wrong syscall call\nExpected: Mount(%v)\nReceived: Mount(%v)\n", expectedSource, source)
		}
		if expectedTarget := "/tmp/docker-test-devmapper-"; !strings.HasPrefix(target, expectedTarget) {
			t.Fatalf("Wrong syscall call\nExpected: Mount(%v)\nReceived: Mount(%v)\n", expectedTarget, target)
		}
		if expectedFstype := "ext4"; fstype != expectedFstype {
			t.Fatalf("Wrong syscall call\nExpected: Mount(%v)\nReceived: Mount(%v)\n", expectedFstype, fstype)
		}
		if expectedFlags := uintptr(3236757504); flags != expectedFlags {
			t.Fatalf("Wrong syscall call\nExpected: Mount(%v)\nReceived: Mount(%v)\n", expectedFlags, flags)
		}
		return nil
	}
	sysUnmount = func(target string, flags int) (err error) {
		calls["sysUnmount"] = true
		// FIXME: compare the exact source and target strings (inodes + devname)
		if expectedTarget := "/tmp/docker-test-devmapper-"; !strings.HasPrefix(target, expectedTarget) {
			t.Fatalf("Wrong syscall call\nExpected: Mount(%v)\nReceived: Mount(%v)\n", expectedTarget, target)
		}
		if expectedFlags := 0; flags != expectedFlags {
			t.Fatalf("Wrong syscall call\nExpected: Mount(%v)\nReceived: Mount(%v)\n", expectedFlags, flags)
		}
		return nil
	}
	Mounted = func(mnt string) (bool, error) {
		calls["Mounted"] = true
		return false, nil
	}

	sysSyscall = func(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno) {
		calls["sysSyscall"] = true
		if trap != sysSysIoctl {
			t.Fatalf("Unexpected syscall. Expecting SYS_IOCTL, received: %d", trap)
		}
		switch a2 {
		case LoopSetFd:
			calls["ioctl.loopsetfd"] = true
		case LoopCtlGetFree:
			calls["ioctl.loopctlgetfree"] = true
		case LoopGetStatus64:
			calls["ioctl.loopgetstatus"] = true
		case LoopSetStatus64:
			calls["ioctl.loopsetstatus"] = true
		case LoopClrFd:
			calls["ioctl.loopclrfd"] = true
		case LoopSetCapacity:
			calls["ioctl.loopsetcapacity"] = true
		case BlkGetSize64:
			calls["ioctl.blkgetsize"] = true
		default:
			t.Fatalf("Unexpected IOCTL. Received %d", a2)
		}
		return 0, 0, 0
	}

	func() {
		d := newDriver(t)

		calls.Assert(t,
			"DmSetDevDir",
			"DmLogWithErrnoInit",
			"DmTaskSetName",
			"DmTaskRun",
			"DmTaskGetInfo",
			"execRun",
			"DmTaskCreate",
			"DmTaskSetTarget",
			"DmTaskSetCookie",
			"DmUdevWait",
			"DmTaskSetSector",
			"DmTaskSetMessage",
			"DmTaskSetAddNode",
			"sysSyscall",
			"ioctl.blkgetsize",
			"ioctl.loopsetfd",
			"ioctl.loopsetstatus",
		)

		if err := d.Create("1", ""); err != nil {
			t.Fatal(err)
		}

		calls.Assert(t,
			"DmTaskCreate",
			"DmTaskGetInfo",
			"sysMount",
			"Mounted",
			"DmTaskRun",
			"DmTaskSetTarget",
			"DmTaskSetSector",
			"DmTaskSetCookie",
			"DmUdevWait",
			"DmTaskSetName",
			"DmTaskSetMessage",
			"DmTaskSetAddNode",
		)

		Mounted = func(mnt string) (bool, error) {
			calls["Mounted"] = true
			return true, nil
		}

		if err := d.Remove("1"); err != nil {
			t.Fatal(err)
		}

		calls.Assert(t,
			"DmTaskRun",
			"DmTaskSetSector",
			"DmTaskSetName",
			"DmTaskSetMessage",
			"DmTaskCreate",
			"DmTaskGetInfo",
			"Mounted",
			"sysUnmount",
		)
	}()
	runtime.GC()

	calls.Assert(t,
		"DmTaskDestroy",
	)
}

func TestCleanup(t *testing.T) {
	t.Skip("FIXME: not a unit test")
	t.Skip("Unimplemented")
	d := newDriver(t)
	defer osRemoveAll(d.home)

	mountPoints := make([]string, 2)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
	// Mount the id
	p, err := d.Get("1")
	if err != nil {
		t.Fatal(err)
	}
	mountPoints[0] = p

	if err := d.Create("2", "1"); err != nil {
		t.Fatal(err)
	}

	p, err = d.Get("2")
	if err != nil {
		t.Fatal(err)
	}
	mountPoints[1] = p

	// Ensure that all the mount points are currently mounted
	for _, p := range mountPoints {
		if mounted, err := Mounted(p); err != nil {
			t.Fatal(err)
		} else if !mounted {
			t.Fatalf("Expected %s to be mounted", p)
		}
	}

	// Ensure that devices are active
	for _, p := range []string{"1", "2"} {
		if !d.HasActivatedDevice(p) {
			t.Fatalf("Expected %s to have an active device", p)
		}
	}

	if err := d.Cleanup(); err != nil {
		t.Fatal(err)
	}

	// Ensure that all the mount points are no longer mounted
	for _, p := range mountPoints {
		if mounted, err := Mounted(p); err != nil {
			t.Fatal(err)
		} else if mounted {
			t.Fatalf("Expected %s to not be mounted", p)
		}
	}

	// Ensure that devices are no longer activated
	for _, p := range []string{"1", "2"} {
		if d.HasActivatedDevice(p) {
			t.Fatalf("Expected %s not be an active device", p)
		}
	}
}

func TestNotMounted(t *testing.T) {
	t.Skip("FIXME: not a unit test")
	t.Skip("Not implemented")
	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	mounted, err := Mounted(path.Join(d.home, "mnt", "1"))
	if err != nil {
		t.Fatal(err)
	}
	if mounted {
		t.Fatal("Id 1 should not be mounted")
	}
}

func TestMounted(t *testing.T) {
	t.Skip("FIXME: not a unit test")
	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}

	mounted, err := Mounted(path.Join(d.home, "mnt", "1"))
	if err != nil {
		t.Fatal(err)
	}
	if !mounted {
		t.Fatal("Id 1 should be mounted")
	}
}

func TestInitCleanedDriver(t *testing.T) {
	t.Skip("FIXME: not a unit test")
	d := newDriver(t)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}

	if err := d.Cleanup(); err != nil {
		t.Fatal(err)
	}

	driver, err := Init(d.home)
	if err != nil {
		t.Fatal(err)
	}
	d = driver.(*Driver)
	defer cleanup(d)

	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}
}

func TestMountMountedDriver(t *testing.T) {
	t.Skip("FIXME: not a unit test")
	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	// Perform get on same id to ensure that it will
	// not be mounted twice
	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}
}

func TestGetReturnsValidDevice(t *testing.T) {
	t.Skip("FIXME: not a unit test")
	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	if !d.HasDevice("1") {
		t.Fatalf("Expected id 1 to be in device set")
	}

	if _, err := d.Get("1"); err != nil {
		t.Fatal(err)
	}

	if !d.HasActivatedDevice("1") {
		t.Fatalf("Expected id 1 to be activated")
	}

	if !d.HasInitializedDevice("1") {
		t.Fatalf("Expected id 1 to be initialized")
	}
}

func TestDriverGetSize(t *testing.T) {
	t.Skip("FIXME: not a unit test")
	t.Skipf("Size is currently not implemented")

	d := newDriver(t)
	defer cleanup(d)

	if err := d.Create("1", ""); err != nil {
		t.Fatal(err)
	}

	mountPoint, err := d.Get("1")
	if err != nil {
		t.Fatal(err)
	}

	size := int64(1024)

	f, err := osCreate(path.Join(mountPoint, "test_file"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// diffSize, err := d.DiffSize("1")
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// if diffSize != size {
	// 	t.Fatalf("Expected size %d got %d", size, diffSize)
	// }
}

func assertMap(t *testing.T, m map[string]bool, keys ...string) {
	for _, key := range keys {
		if _, exists := m[key]; !exists {
			t.Fatalf("Key not set: %s", key)
		}
		delete(m, key)
	}
	if len(m) != 0 {
		t.Fatalf("Unexpected keys: %v", m)
	}
}
