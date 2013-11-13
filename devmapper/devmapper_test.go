package devmapper

import (
	"syscall"
	"testing"
)

func TestTaskCreate(t *testing.T) {
	// Test success
	taskCreate(t, DeviceInfo)

	// Test Failure
	DmTaskCreate = dmTaskCreateFail
	defer func() { DmTaskCreate = dmTaskCreateFct }()
	if task := TaskCreate(-1); task != nil {
		t.Fatalf("An error should have occured while creating an invalid task.")
	}
}

func TestTaskRun(t *testing.T) {
	task := taskCreate(t, DeviceInfo)

	// Test success
	// Perform the RUN
	if err := task.Run(); err != nil {
		t.Fatal(err)
	}
	// Make sure we don't have error with GetInfo
	if _, err := task.GetInfo(); err != nil {
		t.Fatal(err)
	}

	// Test failure
	DmTaskRun = dmTaskRunFail
	defer func() { DmTaskRun = dmTaskRunFct }()

	task = taskCreate(t, DeviceInfo)
	// Perform the RUN
	if err := task.Run(); err == nil {
		t.Fatalf("An error should have occured while running task.")
	}
	// Make sure GetInfo also fails
	if _, err := task.GetInfo(); err == nil {
		t.Fatalf("GetInfo should fail if task.Run() failed.")
	}
}

func TestTaskSetName(t *testing.T) {
	task := taskCreate(t, DeviceInfo)

	// Test success
	if err := task.SetName("test"); err != nil {
		t.Fatal(err)
	}

	// Test failure
	DmTaskSetName = dmTaskSetNameFail
	defer func() { DmTaskSetName = dmTaskSetNameFct }()
	if err := task.SetName("test"); err != ErrTaskSetName {
		t.Fatalf("An error should have occured while runnign SetName.")
	}
}

func TestTaskSetMessage(t *testing.T) {
	task := taskCreate(t, DeviceInfo)

	// Test success
	if err := task.SetMessage("test"); err != nil {
		t.Fatal(err)
	}

	// Test failure
	DmTaskSetMessage = dmTaskSetMessageFail
	defer func() { DmTaskSetMessage = dmTaskSetMessageFct }()
	if err := task.SetMessage("test"); err != ErrTaskSetMessage {
		t.Fatalf("An error should have occured while runnign SetMessage.")
	}
}

func TestTaskSetSector(t *testing.T) {
	task := taskCreate(t, DeviceInfo)

	// Test success
	if err := task.SetSector(128); err != nil {
		t.Fatal(err)
	}

	DmTaskSetSector = dmTaskSetSectorFail
	defer func() { DmTaskSetSector = dmTaskSetSectorFct }()

	// Test failure
	if err := task.SetSector(0); err != ErrTaskSetSector {
		t.Fatalf("An error should have occured while running SetSector.")
	}
}

func TestTaskSetCookie(t *testing.T) {
	var (
		cookie uint = 0
		task        = taskCreate(t, DeviceInfo)
	)

	// Test success
	if err := task.SetCookie(&cookie, 0); err != nil {
		t.Fatal(err)
	}

	// Test failure
	if err := task.SetCookie(nil, 0); err != ErrNilCookie {
		t.Fatalf("An error should have occured while running SetCookie with nil cookie.")
	}

	DmTaskSetCookie = dmTaskSetCookieFail
	defer func() { DmTaskSetCookie = dmTaskSetCookieFct }()

	if err := task.SetCookie(&cookie, 0); err != ErrTaskSetCookie {
		t.Fatalf("An error should have occured while running SetCookie.")
	}
}

func TestTaskSetAddNode(t *testing.T) {
	task := taskCreate(t, DeviceInfo)
	if err := task.SetAddNode(0); err != nil {
		t.Fatal(err)
	}
}

func TestTaskSetRo(t *testing.T) {
	task := taskCreate(t, DeviceInfo)
	if err := task.SetRo(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskAddTarget(t *testing.T) {
	//	task := taskCreate(t, DeviceInfo)
}

/// Utils
func taskCreate(t *testing.T, taskType TaskType) *Task {
	task := TaskCreate(taskType)
	if task == nil {
		t.Fatalf("Error creating task")
	}
	return task
}

/// Failure function replacement
func dmTaskCreateFail(t int) *CDmTask {
	return nil
}

func dmTaskRunFail(task *CDmTask) int {
	return -1
}

func dmTaskSetNameFail(task *CDmTask, name string) int {
	return -1
}

func dmTaskSetMessageFail(task *CDmTask, message string) int {
	return -1
}

func dmTaskSetSectorFail(task *CDmTask, sector uint64) int {
	return -1
}

func dmTaskSetCookieFail(task *CDmTask, cookie *uint, flags uint16) int {
	return -1
}

func dmTaskSetAddNodeFail(task *CDmTask, addNode AddNodeType) int {
	return -1
}

func dmTaskSetRoFail(task *CDmTask) int {
	return -1
}

func dmTaskAddTargetFail(task *CDmTask,
	start, size uint64, ttype, params string) int {
	return -1
}

func dmTaskGetDriverVersionFail(task *CDmTask, version *string) int {
	return -1
}

func dmTaskGetInfoFail(task *CDmTask, info *Info) int {
	return -1
}

func dmGetNextTargetFail(task *CDmTask, next uintptr, start, length *uint64,
	target, params *string) uintptr {
	return 0
}

func dmAttachLoopDeviceFail(filename string, fd *int) string {
	return ""
}

func sysGetBlockSizeFail(fd uintptr, size *uint64) syscall.Errno {
	return 1
}

func dmGetBlockSizeFail(fd uintptr) int64 {
	return -1
}

func dmUdevWaitFail(cookie uint) int {
	return -1
}

func dmSetDevDirFail(dir string) int {
	return -1
}

func dmGetLibraryVersionFail(version *string) int {
	return -1
}
