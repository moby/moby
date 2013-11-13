package devmapper

import (
	"errors"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"os"
	"runtime"
)

type DevmapperLogger interface {
	log(level int, file string, line int, dmError int, message string)
}

const (
	DeviceCreate TaskType = iota
	DeviceReload
	DeviceRemove
	DeviceRemoveAll
	DeviceSuspend
	DeviceResume
	DeviceInfo
	DeviceDeps
	DeviceRename
	DeviceVersion
	DeviceStatus
	DeviceTable
	DeviceWaitevent
	DeviceList
	DeviceClear
	DeviceMknodes
	DeviceListVersions
	DeviceTargetMsg
	DeviceSetGeometry
)

const (
	AddNodeOnResume AddNodeType = iota
	AddNodeOnCreate
)

var (
	ErrTaskRun              = errors.New("dm_task_run failed")
	ErrTaskSetName          = errors.New("dm_task_set_name failed")
	ErrTaskSetMessage       = errors.New("dm_task_set_message failed")
	ErrTaskSetAddNode       = errors.New("dm_task_set_add_node failed")
	ErrTaskSetRO            = errors.New("dm_task_set_ro failed")
	ErrTaskAddTarget        = errors.New("dm_task_add_target failed")
	ErrGetInfo              = errors.New("dm_task_get_info failed")
	ErrGetDriverVersion     = errors.New("dm_task_get_driver_version failed")
	ErrAttachLoopbackDevice = errors.New("loopback mounting failed")
	ErrGetBlockSize         = errors.New("Can't get block size")
	ErrUdevWait             = errors.New("wait on udev cookie failed")
	ErrSetDevDir            = errors.New("dm_set_dev_dir failed")
	ErrGetLibraryVersion    = errors.New("dm_get_library_version failed")
	ErrCreateRemoveTask     = errors.New("Can't create task of type DeviceRemove")
	ErrRunRemoveDevice      = errors.New("running removeDevice failed")
)

type (
	Task struct {
		unmanaged *CDmTask
	}
	Info struct {
		Exists        int
		Suspended     int
		LiveTable     int
		InactiveTable int
		OpenCount     int32
		EventNr       uint32
		Major         uint32
		Minor         uint32
		ReadOnly      int
		TargetCount   int32
	}
	TaskType    int
	AddNodeType int
)

func (t *Task) destroy() {
	if t != nil {
		DmTaskDestory(t.unmanaged)
		runtime.SetFinalizer(t, nil)
	}
}

func TaskCreate(tasktype TaskType) *Task {
	Ctask := DmTaskCreate(int(tasktype))
	if Ctask == nil {
		return nil
	}
	task := &Task{unmanaged: Ctask}
	runtime.SetFinalizer(task, (*Task).destroy)
	return task
}

func (t *Task) Run() error {
	if res := DmTaskRun(t.unmanaged); res != 1 {
		return ErrTaskRun
	}
	return nil
}

func (t *Task) SetName(name string) error {
	if res := DmTaskSetName(t.unmanaged, name); res != 1 {
		return ErrTaskSetName
	}
	return nil
}

func (t *Task) SetMessage(message string) error {
	if res := DmTaskSetMessage(t.unmanaged, message); res != 1 {
		return ErrTaskSetMessage
	}
	return nil
}

func (t *Task) SetSector(sector uint64) error {
	if res := DmTaskSetSector(t.unmanaged, sector); res != 1 {
		return ErrTaskSetAddNode
	}
	return nil
}

func (t *Task) SetCookie(cookie *uint, flags uint16) error {
	if res := DmTaskSetCookie(t.unmanaged, cookie, flags); res != 1 {
		return ErrTaskSetAddNode
	}
	return nil
}

func (t *Task) SetAddNode(addNode AddNodeType) error {
	if res := DmTaskSetAddNode(t.unmanaged, addNode); res != 1 {
		return ErrTaskSetAddNode
	}
	return nil
}

func (t *Task) SetRo() error {
	if res := DmTaskSetRo(t.unmanaged); res != 1 {
		return ErrTaskSetRO
	}
	return nil
}

func (t *Task) AddTarget(start, size uint64, ttype, params string) error {
	if res := DmTaskAddTarget(t.unmanaged, start, size,
		ttype, params); res != 1 {
		return ErrTaskAddTarget
	}
	return nil
}

func (t *Task) GetDriverVersion() (string, error) {
	var version string
	if res := DmTaskGetDriverVersion(t.unmanaged, &version); res != 1 {
		return "", ErrGetDriverVersion
	}
	return version, nil
}

func (t *Task) GetInfo() (*Info, error) {
	info := &Info{}
	if res := DmTaskGetInfo(t.unmanaged, info); res != 1 {
		return nil, ErrGetInfo
	}
	return info, nil
}

func (t *Task) GetNextTarget(next uintptr) (nextPtr uintptr, start uint64,
	length uint64, targetType string, params string) {

	return DmGetNextTarget(t.unmanaged, next, &start, &length,
			&targetType, &params),
		start, length, targetType, params
}

func AttachLoopDevice(filename string) (*os.File, error) {
	var fd int
	res := DmAttachLoopDevice(filename, &fd)
	if res == "" {
		return nil, ErrAttachLoopbackDevice
	}
	return os.NewFile(uintptr(fd), res), nil
}

func getBlockSize(fd uintptr) int {
	var size uint64

	if err := SysGetBlockSize(fd, &size); err != 0 {
		utils.Debugf("Error ioctl (getBlockSize: %s)", err)
		return -1
	}
	return int(size)
}

func GetBlockDeviceSize(file *os.File) (uint64, error) {
	size := DmGetBlockSize(file.Fd())
	if size == -1 {
		return 0, ErrGetBlockSize
	}
	return uint64(size), nil
}

func UdevWait(cookie uint) error {
	if res := DmUdevWait(cookie); res != 1 {
		utils.Debugf("Failed to wait on udev cookie %d", cookie)
		return ErrUdevWait
	}
	return nil
}

func LogInitVerbose(level int) {
	DmLogInitVerbose(level)
}

var dmLogger DevmapperLogger = nil

func logInit(logger DevmapperLogger) {
	dmLogger = logger
	LogWithErrnoInit()
}

func SetDevDir(dir string) error {
	if res := DmSetDevDir(dir); res != 1 {
		utils.Debugf("Error dm_set_dev_dir")
		return ErrSetDevDir
	}
	return nil
}

func GetLibraryVersion() (string, error) {
	var version string
	if res := DmGetLibraryVersion(&version); res != 1 {
		return "", ErrGetLibraryVersion
	}
	return version, nil
}

// Useful helper for cleanup
func RemoveDevice(name string) error {
	task := TaskCreate(DeviceRemove)
	if task == nil {
		return ErrCreateRemoveTask
	}
	if err := task.SetName(name); err != nil {
		utils.Debugf("Can't set task name %s", name)
		return err
	}
	if err := task.Run(); err != nil {
		return ErrRunRemoveDevice
	}
	return nil
}

// This is the programmatic example of "dmsetup create"
func createPool(poolName string, dataFile *os.File, metadataFile *os.File) error {
	task, err := createTask(DeviceCreate, poolName)
	if task == nil {
		return err
	}

	size, err := GetBlockDeviceSize(dataFile)
	if err != nil {
		return fmt.Errorf("Can't get data size")
	}

	params := metadataFile.Name() + " " + dataFile.Name() + " 128 32768"
	if err := task.AddTarget(0, size/512, "thin-pool", params); err != nil {
		return fmt.Errorf("Can't add target")
	}

	var cookie uint = 0
	if err := task.SetCookie(&cookie, 0); err != nil {
		return fmt.Errorf("Can't set cookie")
	}

	if err := task.Run(); err != nil {
		return fmt.Errorf("Error running DeviceCreate")
	}

	UdevWait(cookie)

	return nil
}

func createTask(t TaskType, name string) (*Task, error) {
	task := TaskCreate(t)
	if task == nil {
		return nil, fmt.Errorf("Can't create task of type %d", int(t))
	}
	if err := task.SetName(name); err != nil {
		return nil, fmt.Errorf("Can't set task name %s", name)
	}
	return task, nil
}

func getInfo(name string) (*Info, error) {
	task, err := createTask(DeviceInfo, name)
	if task == nil {
		return nil, err
	}
	if err := task.Run(); err != nil {
		return nil, err
	}
	return task.GetInfo()
}

func getStatus(name string) (uint64, uint64, string, string, error) {
	task, err := createTask(DeviceStatus, name)
	if task == nil {
		utils.Debugf("getStatus: Error createTask: %s", err)
		return 0, 0, "", "", err
	}
	if err := task.Run(); err != nil {
		utils.Debugf("getStatus: Error Run: %s", err)
		return 0, 0, "", "", err
	}

	devinfo, err := task.GetInfo()
	if err != nil {
		utils.Debugf("getStatus: Error GetInfo: %s", err)
		return 0, 0, "", "", err
	}
	if devinfo.Exists == 0 {
		utils.Debugf("getStatus: Non existing device %s", name)
		return 0, 0, "", "", fmt.Errorf("Non existing device %s", name)
	}

	_, start, length, target_type, params := task.GetNextTarget(0)
	return start, length, target_type, params, nil
}

func setTransactionId(poolName string, oldId uint64, newId uint64) error {
	task, err := createTask(DeviceTargetMsg, poolName)
	if task == nil {
		return err
	}

	if err := task.SetSector(0); err != nil {
		return fmt.Errorf("Can't set sector")
	}

	if err := task.SetMessage(fmt.Sprintf("set_transaction_id %d %d", oldId, newId)); err != nil {
		return fmt.Errorf("Can't set message")
	}

	if err := task.Run(); err != nil {
		return fmt.Errorf("Error running setTransactionId")
	}
	return nil
}

func suspendDevice(name string) error {
	task, err := createTask(DeviceSuspend, name)
	if task == nil {
		return err
	}
	if err := task.Run(); err != nil {
		return fmt.Errorf("Error running DeviceSuspend")
	}
	return nil
}

func resumeDevice(name string) error {
	task, err := createTask(DeviceResume, name)
	if task == nil {
		return err
	}

	var cookie uint = 0
	if err := task.SetCookie(&cookie, 0); err != nil {
		return fmt.Errorf("Can't set cookie")
	}

	if err := task.Run(); err != nil {
		return fmt.Errorf("Error running DeviceSuspend")
	}

	UdevWait(cookie)

	return nil
}

func createDevice(poolName string, deviceId int) error {
	utils.Debugf("[devmapper] createDevice(poolName=%v, deviceId=%v)", poolName, deviceId)
	task, err := createTask(DeviceTargetMsg, poolName)
	if task == nil {
		return err
	}

	if err := task.SetSector(0); err != nil {
		return fmt.Errorf("Can't set sector")
	}

	if err := task.SetMessage(fmt.Sprintf("create_thin %d", deviceId)); err != nil {
		return fmt.Errorf("Can't set message")
	}

	if err := task.Run(); err != nil {
		return fmt.Errorf("Error running createDevice")
	}
	return nil
}

func deleteDevice(poolName string, deviceId int) error {
	task, err := createTask(DeviceTargetMsg, poolName)
	if task == nil {
		return err
	}

	if err := task.SetSector(0); err != nil {
		return fmt.Errorf("Can't set sector")
	}

	if err := task.SetMessage(fmt.Sprintf("delete %d", deviceId)); err != nil {
		return fmt.Errorf("Can't set message")
	}

	if err := task.Run(); err != nil {
		return fmt.Errorf("Error running deleteDevice")
	}
	return nil
}

func removeDevice(name string) error {
	utils.Debugf("[devmapper] removeDevice START")
	defer utils.Debugf("[devmapper] removeDevice END")
	task, err := createTask(DeviceRemove, name)
	if task == nil {
		return err
	}
	if err = task.Run(); err != nil {
		return fmt.Errorf("Error running removeDevice")
	}
	return nil
}

func activateDevice(poolName string, name string, deviceId int, size uint64) error {
	task, err := createTask(DeviceCreate, name)
	if task == nil {
		return err
	}

	params := fmt.Sprintf("%s %d", poolName, deviceId)
	if err := task.AddTarget(0, size/512, "thin", params); err != nil {
		return fmt.Errorf("Can't add target")
	}
	if err := task.SetAddNode(AddNodeOnCreate); err != nil {
		return fmt.Errorf("Can't add node")
	}

	var cookie uint = 0
	if err := task.SetCookie(&cookie, 0); err != nil {
		return fmt.Errorf("Can't set cookie")
	}

	if err := task.Run(); err != nil {
		return fmt.Errorf("Error running DeviceCreate")
	}

	UdevWait(cookie)

	return nil
}

func (devices *DeviceSet) createSnapDevice(poolName string, deviceId int, baseName string, baseDeviceId int) error {
	devinfo, _ := getInfo(baseName)
	doSuspend := devinfo != nil && devinfo.Exists != 0

	if doSuspend {
		if err := suspendDevice(baseName); err != nil {
			return err
		}
	}

	task, err := createTask(DeviceTargetMsg, poolName)
	if task == nil {
		if doSuspend {
			resumeDevice(baseName)
		}
		return err
	}

	if err := task.SetSector(0); err != nil {
		if doSuspend {
			resumeDevice(baseName)
		}
		return fmt.Errorf("Can't set sector")
	}

	if err := task.SetMessage(fmt.Sprintf("create_snap %d %d", deviceId, baseDeviceId)); err != nil {
		if doSuspend {
			resumeDevice(baseName)
		}
		return fmt.Errorf("Can't set message")
	}

	if err := task.Run(); err != nil {
		if doSuspend {
			resumeDevice(baseName)
		}
		return fmt.Errorf("Error running DeviceCreate")
	}

	if doSuspend {
		if err := resumeDevice(baseName); err != nil {
			return err
		}
	}

	return nil
}
