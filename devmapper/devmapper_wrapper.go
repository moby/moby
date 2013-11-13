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

// FIXME: this could easily be rewritten in go
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
	return NULL;
    }

    if (stat(buf, &st)) {
      if (!S_ISBLK(st.st_mode)) {
	 fprintf(stderr, "[error] Loopback device %s is not a block device.\n", buf);
      } else if (errno == ENOENT) {
	fprintf(stderr, "[error] There are no more loopback device available.\n");
      } else {
	fprintf(stderr, "[error] Unkown error trying to stat the loopback device %s (errno: %d).\n", buf, errno);
      }
      close(fd);
      return NULL;
    }

    loop_fd = open(buf, O_RDWR);
    if (loop_fd < 0 && errno == ENOENT) {
      fprintf(stderr, "[error] The loopback device %s does not exists.\n", buf);
      close(fd);
      return NULL;
    } else if (loop_fd < 0) {
	fprintf(stderr, "[error] Unkown error openning the loopback device %s. (errno: %d)\n", buf, errno);
	continue;
    }

    if (ioctl(loop_fd, LOOP_SET_FD, (void *)(size_t)fd) < 0) {
      int errsv = errno;
      close(loop_fd);
      loop_fd = -1;
      if (errsv != EBUSY) {
        close(fd);
        fprintf(stderr, "cannot set up loopback device %s: %s", buf, strerror(errsv));
        return NULL;
      }
      continue;
    }

    close(fd);

    strncpy((char*)loopinfo.lo_file_name, buf, LO_NAME_SIZE);
    loopinfo.lo_offset = 0;
    loopinfo.lo_flags = LO_FLAGS_AUTOCLEAR;

    if (ioctl(loop_fd, LOOP_SET_STATUS64, &loopinfo) < 0) {
      perror("ioctl LOOP_SET_STATUS64");
      if (ioctl(loop_fd, LOOP_CLR_FD, 0) < 0) {
        perror("ioctl LOOP_CLR_FD");
      }
      close(loop_fd);
      fprintf (stderr, "cannot set up loopback device info");
      return (NULL);
    }

    loopname = strdup(buf);
    if (loopname == NULL) {
      close(loop_fd);
      return (NULL);
    }

    *loop_fd_out = loop_fd;
    return (loopname);
  }

  return (NULL);
}

static int64_t	get_block_size(int fd)
{
  uint64_t	size;

  if (ioctl(fd, BLKGETSIZE64, &size) == -1)
    return -1;
  return ((int64_t)size);
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
	"syscall"
	"unsafe"
)

type (
	CDmTask C.struct_dm_task
)

var (
	DmTaskDestory          = dmTaskDestroyFct
	DmTaskCreate           = dmTaskCreateFct
	DmTaskRun              = dmTaskRunFct
	DmTaskSetName          = dmTaskSetNameFct
	DmTaskSetMessage       = dmTaskSetMessageFct
	DmTaskSetSector        = dmTaskSetSectorFct
	DmTaskSetCookie        = dmTaskSetCookieFct
	DmTaskSetAddNode       = dmTaskSetAddNodeFct
	DmTaskSetRo            = dmTaskSetRoFct
	DmTaskAddTarget        = dmTaskAddTargetFct
	DmTaskGetDriverVersion = dmTaskGetDriverVersionFct
	DmTaskGetInfo          = dmTaskGetInfoFct
	DmGetNextTarget        = dmGetNextTargetFct
	DmAttachLoopDevice     = dmAttachLoopDeviceFct
	SysGetBlockSize        = sysGetBlockSizeFct
	DmGetBlockSize         = dmGetBlockSizeFct
	DmUdevWait             = dmUdevWaitFct
	DmLogInitVerbose       = dmLogInitVerboseFct
	LogWithErrnoInit       = logWithErrnoInitFct
	DmSetDevDir            = dmSetDevDirFct
	DmGetLibraryVersion    = dmGetLibraryVersionFct
)

func free(p *C.char) {
	C.free(unsafe.Pointer(p))
}

func dmTaskDestroyFct(task *CDmTask) {
	C.dm_task_destroy((*C.struct_dm_task)(task))
}

func dmTaskCreateFct(taskType int) *CDmTask {
	return (*CDmTask)(C.dm_task_create(C.int(taskType)))
}

func dmTaskRunFct(task *CDmTask) int {
	return int(C.dm_task_run((*C.struct_dm_task)(task)))
}

func dmTaskSetNameFct(task *CDmTask, name string) int {
	Cname := C.CString(name)
	defer free(Cname)

	return int(C.dm_task_set_name((*C.struct_dm_task)(task),
		Cname))
}

func dmTaskSetMessageFct(task *CDmTask, message string) int {
	Cmessage := C.CString(message)
	defer free(Cmessage)

	return int(C.dm_task_set_message((*C.struct_dm_task)(task),
		Cmessage))
}

func dmTaskSetSectorFct(task *CDmTask, sector uint64) int {
	return int(C.dm_task_set_sector((*C.struct_dm_task)(task),
		C.uint64_t(sector)))
}

func dmTaskSetCookieFct(task *CDmTask, cookie *uint, flags uint16) int {
	cCookie := C.uint32_t(*cookie)
	defer func() {
		*cookie = uint(cCookie)
	}()
	return int(C.dm_task_set_cookie((*C.struct_dm_task)(task), &cCookie,
		C.uint16_t(flags)))
}

func dmTaskSetAddNodeFct(task *CDmTask, addNode AddNodeType) int {
	return int(C.dm_task_set_add_node((*C.struct_dm_task)(task),
		C.dm_add_node_t(addNode)))
}

func dmTaskSetRoFct(task *CDmTask) int {
	return int(C.dm_task_set_ro((*C.struct_dm_task)(task)))
}

func dmTaskAddTargetFct(task *CDmTask,
	start, size uint64, ttype, params string) int {

	Cttype := C.CString(ttype)
	defer free(Cttype)

	Cparams := C.CString(params)
	defer free(Cparams)

	return int(C.dm_task_add_target((*C.struct_dm_task)(task),
		C.uint64_t(start), C.uint64_t(size), Cttype, Cparams))
}

func dmTaskGetDriverVersionFct(task *CDmTask, version *string) int {
	buffer := C.CString(string(make([]byte, 128)))
	defer free(buffer)
	defer func() {
		*version = C.GoString(buffer)
	}()
	return int(C.dm_task_get_driver_version((*C.struct_dm_task)(task),
		buffer, 128))
}

func dmTaskGetInfoFct(task *CDmTask, info *Info) int {
	Cinfo := C.struct_dm_info{}
	defer func() {
		info.Exists = int(Cinfo.exists)
		info.Suspended = int(Cinfo.suspended)
		info.LiveTable = int(Cinfo.live_table)
		info.InactiveTable = int(Cinfo.inactive_table)
		info.OpenCount = int32(Cinfo.open_count)
		info.EventNr = uint32(Cinfo.event_nr)
		info.Major = uint32(Cinfo.major)
		info.Minor = uint32(Cinfo.minor)
		info.ReadOnly = int(Cinfo.read_only)
		info.TargetCount = int32(Cinfo.target_count)
	}()
	return int(C.dm_task_get_info((*C.struct_dm_task)(task), &Cinfo))
}

func dmGetNextTargetFct(task *CDmTask, next uintptr, start, length *uint64,
	target, params *string) uintptr {

	var (
		Cstart, Clength      C.uint64_t
		CtargetType, Cparams *C.char
	)
	defer func() {
		*start = uint64(Cstart)
		*length = uint64(Clength)
		*target = C.GoString(CtargetType)
		*params = C.GoString(Cparams)
	}()
	nextp := C.dm_get_next_target((*C.struct_dm_task)(task),
		unsafe.Pointer(next), &Cstart, &Clength, &CtargetType, &Cparams)
	return uintptr(nextp)
}

func dmAttachLoopDeviceFct(filename string, fd *int) string {
	cFilename := C.CString(filename)
	defer free(cFilename)

	var cFd C.int
	defer func() {
		*fd = int(cFd)
	}()

	ret := C.attach_loop_device(cFilename, &cFd)
	defer free(ret)
	return C.GoString(ret)
}

// sysGetBlockSizeFct retrieves the block size from IOCTL
func sysGetBlockSizeFct(fd uintptr, size *uint64) syscall.Errno {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, C.BLKGETSIZE64,
		uintptr(unsafe.Pointer(&size)))
	return err
}

// dmGetBlockSizeFct retrieves the block size from library call
func dmGetBlockSizeFct(fd uintptr) int64 {
	return int64(C.get_block_size(C.int(fd)))
}

func dmUdevWaitFct(cookie uint) int {
	return int(C.dm_udev_wait(C.uint32_t(cookie)))
}

func dmLogInitVerboseFct(level int) {
	C.dm_log_init_verbose(C.int(level))
}

func logWithErrnoInitFct() {
	C.log_with_errno_init()
}

func dmSetDevDirFct(dir string) int {
	Cdir := C.CString(dir)
	defer free(Cdir)

	return int(C.dm_set_dev_dir(Cdir))
}

func dmGetLibraryVersionFct(version *string) int {
	buffer := C.CString(string(make([]byte, 128)))
	defer free(buffer)
	defer func() {
		*version = C.GoString(buffer)
	}()
	return int(C.dm_get_library_version(buffer, 128))
}
