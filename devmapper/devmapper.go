package devmapper

/*
#cgo LDFLAGS: -L. -ldevmapper
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <libdevmapper.h>
#include <linux/loop.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <sys/ioctl.h>
#include <linux/fs.h>
#include <errno.h>

#ifndef LOOP_CTL_GET_FREE
#define LOOP_CTL_GET_FREE       0x4C82
#endif

char*			attach_loop_device(const char *filename, int *loop_fd_out)
{
  struct loop_info64	loopinfo = {0};
  struct stat		st;
  char			buf[64];
  int			i, loop_fd, fd, start_index;
  char*			loopname;

  *loop_fd_out = -1;

  start_index = 0;
  fd = open("/dev/loop-control", O_RDONLY);
  if (fd >= 0) {
    start_index = ioctl(fd, LOOP_CTL_GET_FREE);
    close(fd);

    if (start_index < 0)
      start_index = 0;
  }

  fd = open(filename, O_RDWR);
  if (fd < 0) {
    perror("open");
    return NULL;
  }

  loop_fd = -1;
  for (i = start_index ; loop_fd < 0 ; i++ ) {
    if (sprintf(buf, "/dev/loop%d", i) < 0) {
      close(fd);
	perror("sprintf");
      return NULL;
    }

    if (stat(buf, &st) || !S_ISBLK(st.st_mode)) {
      close(fd);
      return NULL;
    }

    loop_fd = open(buf, O_RDWR);
    if (loop_fd < 0 && errno == ENOENT) {
      close(fd);
      fprintf (stderr, "no available loopback device!");
      return NULL;
    } else if (loop_fd < 0)
      continue;

    if (ioctl (loop_fd, LOOP_SET_FD, (void *)(size_t)fd) < 0) {
      int errsv = errno;
      close(loop_fd);
      loop_fd = -1;
      if (errsv != EBUSY) {
        close (fd);
        fprintf (stderr, "cannot set up loopback device %s: %s", buf, strerror(errsv));
        return NULL;
      }
      continue;
    }

    close (fd);

    strncpy((char*)loopinfo.lo_file_name, buf, LO_NAME_SIZE);
    loopinfo.lo_offset = 0;
    loopinfo.lo_flags = LO_FLAGS_AUTOCLEAR;

    if (ioctl(loop_fd, LOOP_SET_STATUS64, &loopinfo) < 0) {
      perror("ioctl1");
      if (ioctl(loop_fd, LOOP_CLR_FD, 0) < 0) {
        perror("ioctl2");
      }
      close(loop_fd);
      fprintf (stderr, "cannot set up loopback device info");
      return NULL;
    }

    loopname = strdup(buf);
    if (loopname == NULL) {
      close(loop_fd);
      return NULL;
    }

    *loop_fd_out = loop_fd;
    return loopname;
  }
  return NULL;
}

static int64_t
get_block_size(int fd)
{
  uint64_t size;
  if (ioctl(fd, BLKGETSIZE64, &size) == -1)
    return -1;
  return (int64_t)size;
}

extern void DevmapperLogCallback(int level, char *file, int line, int dm_errno_or_class, char *str);

static void
log_cb(int level, const char *file, int line,
       int dm_errno_or_class, const char *f, ...)
{
  char buffer[256];
  va_list ap;

  va_start(ap, f);
  vsnprintf(buffer, 256, f, ap);
  va_end(ap);

  DevmapperLogCallback(level, (char *)file, line, dm_errno_or_class, buffer);
}

static void
log_with_errno_init ()
{
  dm_log_with_errno_init(log_cb);
}

*/
import "C"

import (
	"errors"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

type DevmapperLogger interface  {
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
		unmanaged *C.struct_dm_task
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
	TaskType int
	AddNodeType int
)

func (t *Task) destroy() {
	if t != nil {
		C.dm_task_destroy(t.unmanaged)
		runtime.SetFinalizer(t, nil)
	}
}

func TaskCreate(tasktype TaskType) *Task {
	c_task := C.dm_task_create(C.int(tasktype))
	if c_task == nil {
		return nil
	}
	task := &Task{unmanaged: c_task}
	runtime.SetFinalizer(task, (*Task).destroy)
	return task
}

func (t *Task) Run() error {
	if res := C.dm_task_run(t.unmanaged); res != 1 {
		return ErrTaskRun
	}
	return nil
}

func (t *Task) SetName(name string) error {
	c_name := C.CString(name)
	defer free(c_name)

	if res := C.dm_task_set_name(t.unmanaged, c_name); res != 1 {
		if os.Getenv("DEBUG") != "" {
			C.perror(C.CString(fmt.Sprintf("[debug] Error dm_task_set_name(%s, %#v)", name, t.unmanaged)))
		}
		return ErrTaskSetName
	}
	return nil
}

func (t *Task) SetMessage(message string) error {
	c_message := C.CString(message)
	defer free(c_message)

	if res := C.dm_task_set_message(t.unmanaged, c_message); res != 1 {
		return ErrTaskSetMessage
	}
	return nil
}

func (t *Task) SetSector(sector uint64) error {
	if res := C.dm_task_set_sector(t.unmanaged, C.uint64_t(sector)); res != 1 {
		return ErrTaskSetAddNode
	}
	return nil
}

func (t *Task) SetCookie(cookie *uint32, flags uint16) error {
	c_cookie := C.uint32_t(*cookie)
	if res := C.dm_task_set_cookie(t.unmanaged, &c_cookie, C.uint16_t(flags)); res != 1 {
		return ErrTaskSetAddNode
	}
	*cookie = uint32(c_cookie)
	return nil
}

func (t *Task) SetAddNode(add_node AddNodeType) error {
	if res := C.dm_task_set_add_node(t.unmanaged, C.dm_add_node_t (add_node)); res != 1 {
		return ErrTaskSetAddNode
	}
	return nil
}

func (t *Task) SetRo() error {
	if res := C.dm_task_set_ro(t.unmanaged); res != 1 {
		return ErrTaskSetRO
	}
	return nil
}

func (t *Task) AddTarget(start uint64, size uint64, ttype string, params string) error {
	c_ttype := C.CString(ttype)
	defer free(c_ttype)

	c_params := C.CString(params)
	defer free(c_params)

	if res := C.dm_task_add_target(t.unmanaged, C.uint64_t(start), C.uint64_t(size), c_ttype, c_params); res != 1 {
		return ErrTaskAddTarget
	}
	return nil
}

func (t *Task) GetDriverVersion() (string, error) {
	buffer := C.CString(string(make([]byte, 128)))
	defer free(buffer)

	if res := C.dm_task_get_driver_version(t.unmanaged, buffer, 128); res != 1 {
		return "", ErrGetDriverVersion
	}
	return C.GoString(buffer), nil
}

func (t *Task) GetInfo() (*Info, error) {
	c_info := C.struct_dm_info{}
	if res := C.dm_task_get_info(t.unmanaged, &c_info); res != 1 {
		return nil, ErrGetDriverVersion
	}
	return &Info{
		Exists:        int(c_info.exists),
		Suspended:     int(c_info.suspended),
		LiveTable:     int(c_info.live_table),
		InactiveTable: int(c_info.inactive_table),
		OpenCount:     int32(c_info.open_count),
		EventNr:       uint32(c_info.event_nr),
		Major:         uint32(c_info.major),
		Minor:         uint32(c_info.minor),
		ReadOnly:      int(c_info.read_only),
		TargetCount:   int32(c_info.target_count),
	}, nil
}

func (t *Task) GetNextTarget(next uintptr) (uintptr, uint64, uint64, string, string) {
	var (
		c_start, c_length       C.uint64_t
		c_target_type, c_params *C.char
	)

	nextp := C.dm_get_next_target(t.unmanaged, unsafe.Pointer(next), &c_start, &c_length, &c_target_type, &c_params)
	return uintptr(nextp), uint64(c_start), uint64(c_length), C.GoString(c_target_type), C.GoString(c_params)
}

func AttachLoopDevice(filename string) (*os.File, error) {
	c_filename := C.CString(filename)
	defer free(c_filename)

	var fd C.int
	res := C.attach_loop_device(c_filename, &fd)
	if res == nil {
		if os.Getenv("DEBUG") != "" {
			C.perror(C.CString(fmt.Sprintf("[debug] Error attach_loop_device(%s, $#v)", c_filename, &fd)))
		}
		return nil, ErrAttachLoopbackDevice
	}
	defer free(res)

	return os.NewFile(uintptr(fd), C.GoString(res)), nil
}

func getBlockSize(fd uintptr) int {
	var size uint64

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, C.BLKGETSIZE64, uintptr(unsafe.Pointer(&size))); err != 0 {
		utils.Debugf("Error ioctl: %s", err)
		return -1
	}
	return int(size)
}

func GetBlockDeviceSize(file *os.File) (uint64, error) {
	if size := C.get_block_size(C.int(file.Fd())); size == -1 {
		return 0, ErrGetBlockSize
	} else {
		return uint64(size), nil
	}
}

func UdevWait(cookie uint32) error {
	if res := C.dm_udev_wait(C.uint32_t(cookie)); res != 1 {
		utils.Debugf("Failed to wait on udev cookie %d", cookie)
		return ErrUdevWait
	}
	return nil
}

func LogInitVerbose(level int) {
	C.dm_log_init_verbose(C.int(level))
}

var dmLogger DevmapperLogger = nil

func logInit(logger DevmapperLogger) {
	dmLogger = logger
	C.log_with_errno_init()
}

func SetDevDir(dir string) error {
	c_dir := C.CString(dir)
	defer free(c_dir)

	if res := C.dm_set_dev_dir(c_dir); res != 1 {
		utils.Debugf("Error dm_set_dev_dir")
		return ErrSetDevDir
	}
	return nil
}

func GetLibraryVersion() (string, error) {
	buffer := C.CString(string(make([]byte, 128)))
	defer free(buffer)

	if res := C.dm_get_library_version(buffer, 128); res != 1 {
		return "", ErrGetLibraryVersion
	}
	return C.GoString(buffer), nil
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

func free(p *C.char) {
	C.free(unsafe.Pointer(p))
}

func createPool(poolName string, dataFile *os.File, metadataFile *os.File) error {
	task, err := createTask(DeviceCreate, poolName)
	if task == nil {
		return err
	}

	size, err := GetBlockDeviceSize(dataFile)
	if err != nil {
		return fmt.Errorf("Can't get data size")
	}

	params := metadataFile.Name() + " " + dataFile.Name() + " 512 8192"
	if err := task.AddTarget(0, size/512, "thin-pool", params); err != nil {
		return fmt.Errorf("Can't add target")
	}

	var cookie uint32 = 0
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

	var cookie uint32 = 0
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

	var cookie uint32 = 0
	if err := task.SetCookie(&cookie, 0); err != nil {
		return fmt.Errorf("Can't set cookie")
	}

	if err := task.Run(); err != nil {
		return fmt.Errorf("Error running DeviceCreate")
	}

	UdevWait(cookie)

	return nil
}

func (devices *DeviceSetDM) createSnapDevice(poolName string, deviceId int, baseName string, baseDeviceId int) error {
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
