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

static char *
attach_loop_device(const char *filename, int *loop_fd_out)
{
  struct loop_info64 loopinfo = { 0 };
  struct stat st;
  char buf[64];
  int i, loop_fd, fd, start_index;
  char *loopname;

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
    return NULL;
  }

  loop_fd = -1;
  for (i = start_index ; loop_fd < 0 ; i++ ) {
    if (sprintf(buf, "/dev/loop%d", i) < 0) {
      close(fd);
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
      close(loop_fd);
      loop_fd = -1;
      if (errno != EBUSY) {
        close (fd);
        fprintf (stderr, "cannot set up loopback device %s", buf);
        return NULL;
      }
      continue;
    }

    close (fd);

    strncpy((char*)loopinfo.lo_file_name, buf, LO_NAME_SIZE);
    loopinfo.lo_offset = 0;
    loopinfo.lo_flags = LO_FLAGS_AUTOCLEAR;

    if (ioctl(loop_fd, LOOP_SET_STATUS64, &loopinfo) < 0) {
      ioctl(loop_fd, LOOP_CLR_FD, 0);
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


*/
import "C"
import "unsafe"
import "fmt"
import "runtime"
import "os"

func SetDevDir(dir string) error {
	c_dir := C.CString(dir)
	defer C.free(unsafe.Pointer(c_dir))
	res := C.dm_set_dev_dir(c_dir)
	if res != 1 {
		return fmt.Errorf("dm_set_dev_dir failed")
	}
	return nil
}

func GetLibraryVersion() (string, error) {
	buffer := (*C.char)(C.malloc(128))
	defer C.free(unsafe.Pointer(buffer))
	res := C.dm_get_library_version(buffer, 128)
	if res != 1 {
		return "", fmt.Errorf("dm_get_library_version failed")
	} else {
		return C.GoString(buffer), nil
	}
}

type TaskType int

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

type Task struct {
	unmanaged *C.struct_dm_task
}

type Info struct {
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

func (t *Task) destroy() {
	if t != nil {
		C.dm_task_destroy(t.unmanaged)
		runtime.SetFinalizer(t, nil)
	}
}

func TaskCreate(tasktype TaskType) *Task {
	c_task := C.dm_task_create(C.int(int(tasktype)))
	if c_task == nil {
		return nil
	}
	task := &Task{c_task}
	runtime.SetFinalizer(task, (*Task).destroy)
	return task
}

func (t *Task) Run() error {
	res := C.dm_task_run(t.unmanaged)
	if res != 1 {
		return fmt.Errorf("dm_task_run failed")
	}
	return nil
}

func (t *Task) SetName(name string) error {
	c_name := C.CString(name)
	defer C.free(unsafe.Pointer(c_name))

	res := C.dm_task_set_name(t.unmanaged, c_name)
	if res != 1 {
		return fmt.Errorf("dm_task_set_name failed")
	}
	return nil
}

func (t *Task) SetMessage(message string) error {
	c_message := C.CString(message)
	defer C.free(unsafe.Pointer(c_message))

	res := C.dm_task_set_message(t.unmanaged, c_message)
	if res != 1 {
		return fmt.Errorf("dm_task_set_message failed")
	}
	return nil
}

func (t *Task) SetSector(sector uint64) error {
	res := C.dm_task_set_sector(t.unmanaged, C.uint64_t(sector))
	if res != 1 {
		return fmt.Errorf("dm_task_set_add_node failed")
	}
	return nil
}

func (t *Task) SetCookie(cookie *uint32, flags uint16) error {
	var c_cookie C.uint32_t
	c_cookie = C.uint32_t(*cookie)
	res := C.dm_task_set_cookie(t.unmanaged, &c_cookie, C.uint16_t(flags))
	if res != 1 {
		return fmt.Errorf("dm_task_set_add_node failed")
	}
	*cookie = uint32(c_cookie)
	return nil
}

func (t *Task) SetRo() error {
	res := C.dm_task_set_ro(t.unmanaged)
	if res != 1 {
		return fmt.Errorf("dm_task_set_ro failed")
	}
	return nil
}

func (t *Task) AddTarget(start uint64, size uint64, ttype string, params string) error {
	c_ttype := C.CString(ttype)
	defer C.free(unsafe.Pointer(c_ttype))

	c_params := C.CString(params)
	defer C.free(unsafe.Pointer(c_params))

	res := C.dm_task_add_target(t.unmanaged, C.uint64_t(start), C.uint64_t(size), c_ttype, c_params)
	if res != 1 {
		return fmt.Errorf("dm_task_add_target failed")
	}
	return nil
}

func (t *Task) GetDriverVersion() (string, error) {
	buffer := (*C.char)(C.malloc(128))
	defer C.free(unsafe.Pointer(buffer))

	res := C.dm_task_get_driver_version(t.unmanaged, buffer, 128)
	if res != 1 {
		return "", fmt.Errorf("dm_task_get_driver_version")
	} else {
		return C.GoString(buffer), nil
	}
}

func (t *Task) GetInfo() (*Info, error) {
	c_info := C.struct_dm_info{}
	res := C.dm_task_get_info(t.unmanaged, &c_info)
	if res != 1 {
		return nil, fmt.Errorf("dm_task_get_driver_version")
	} else {
		info := &Info{}
		info.Exists = int(c_info.exists)
		info.Suspended = int(c_info.suspended)
		info.LiveTable = int(c_info.live_table)
		info.InactiveTable = int(c_info.inactive_table)
		info.OpenCount = int32(c_info.open_count)
		info.EventNr = uint32(c_info.event_nr)
		info.Major = uint32(c_info.major)
		info.Minor = uint32(c_info.minor)
		info.ReadOnly = int(c_info.read_only)
		info.TargetCount = int32(c_info.target_count)

		return info, nil
	}
}

func (t *Task) GetNextTarget(next uintptr) (uintptr, uint64, uint64, string, string) {
	nextp := unsafe.Pointer(next)
	var c_start C.uint64_t
	var c_length C.uint64_t
	var c_target_type *C.char
	var c_params *C.char

	nextp = C.dm_get_next_target(t.unmanaged, nextp, &c_start, &c_length, &c_target_type, &c_params)

	target_type := C.GoString(c_target_type)
	params := C.GoString(c_params)

	return uintptr(nextp), uint64(c_start), uint64(c_length), target_type, params
}

func AttachLoopDevice(filename string) (*os.File, error) {
	c_filename := C.CString(filename)
	defer C.free(unsafe.Pointer(c_filename))

	var fd C.int
	res := C.attach_loop_device(c_filename, &fd)
	if res == nil {
		return nil, fmt.Errorf("error loopback mounting")
	}
	file := os.NewFile(uintptr(fd), C.GoString(res))
	C.free(unsafe.Pointer(res))
	return file, nil
}

func GetBlockDeviceSize(file *os.File) (uint64, error) {
	fd := file.Fd()
	size := C.get_block_size(C.int(fd))
	if size == -1 {
		return 0, fmt.Errorf("Can't get block size")
	}
	return uint64(size), nil

}

func UdevWait(cookie uint32) error {
	res := C.dm_udev_wait(C.uint32_t(cookie))
	if res != 1 {
		return fmt.Errorf("Failed to wait on udev cookie %d", cookie)
	}
	return nil
}

func LogInitVerbose(level int) {
	C.dm_log_init_verbose(C.int(level))
}

// Useful helper for cleanup
func RemoveDevice(name string) error {
	task := TaskCreate(DeviceRemove)
	if task == nil {
		return fmt.Errorf("Can't create task of type DeviceRemove")
	}
	err := task.SetName(name)
	if err != nil {
		return fmt.Errorf("Can't set task name %s", name)
	}
	err = task.Run()
	if err != nil {
		return fmt.Errorf("Error running removeDevice")
	}
	return nil
}
