//go:build linux

package unix

import (
	"syscall"
	"unsafe"

	linux "golang.org/x/sys/unix"
)

const (
	BPF_F_NO_PREALLOC          = linux.BPF_F_NO_PREALLOC
	BPF_F_NUMA_NODE            = linux.BPF_F_NUMA_NODE
	BPF_F_RDONLY               = linux.BPF_F_RDONLY
	BPF_F_WRONLY               = linux.BPF_F_WRONLY
	BPF_F_RDONLY_PROG          = linux.BPF_F_RDONLY_PROG
	BPF_F_WRONLY_PROG          = linux.BPF_F_WRONLY_PROG
	BPF_F_SLEEPABLE            = linux.BPF_F_SLEEPABLE
	BPF_F_XDP_HAS_FRAGS        = linux.BPF_F_XDP_HAS_FRAGS
	BPF_F_MMAPABLE             = linux.BPF_F_MMAPABLE
	BPF_F_INNER_MAP            = linux.BPF_F_INNER_MAP
	BPF_F_KPROBE_MULTI_RETURN  = linux.BPF_F_KPROBE_MULTI_RETURN
	BPF_F_UPROBE_MULTI_RETURN  = linux.BPF_F_UPROBE_MULTI_RETURN
	BPF_F_LOCK                 = linux.BPF_F_LOCK
	BPF_OBJ_NAME_LEN           = linux.BPF_OBJ_NAME_LEN
	BPF_TAG_SIZE               = linux.BPF_TAG_SIZE
	BPF_RINGBUF_BUSY_BIT       = linux.BPF_RINGBUF_BUSY_BIT
	BPF_RINGBUF_DISCARD_BIT    = linux.BPF_RINGBUF_DISCARD_BIT
	BPF_RINGBUF_HDR_SZ         = linux.BPF_RINGBUF_HDR_SZ
	SYS_BPF                    = linux.SYS_BPF
	F_DUPFD_CLOEXEC            = linux.F_DUPFD_CLOEXEC
	EPOLL_CTL_ADD              = linux.EPOLL_CTL_ADD
	EPOLL_CLOEXEC              = linux.EPOLL_CLOEXEC
	O_RDONLY                   = linux.O_RDONLY
	O_DIRECTORY                = linux.O_DIRECTORY
	O_CLOEXEC                  = linux.O_CLOEXEC
	O_NONBLOCK                 = linux.O_NONBLOCK
	PROT_NONE                  = linux.PROT_NONE
	PROT_READ                  = linux.PROT_READ
	PROT_WRITE                 = linux.PROT_WRITE
	MAP_ANON                   = linux.MAP_ANON
	MAP_SHARED                 = linux.MAP_SHARED
	MAP_FIXED                  = linux.MAP_FIXED
	MAP_PRIVATE                = linux.MAP_PRIVATE
	PERF_ATTR_SIZE_VER1        = linux.PERF_ATTR_SIZE_VER1
	PERF_TYPE_SOFTWARE         = linux.PERF_TYPE_SOFTWARE
	PERF_TYPE_TRACEPOINT       = linux.PERF_TYPE_TRACEPOINT
	PERF_COUNT_SW_BPF_OUTPUT   = linux.PERF_COUNT_SW_BPF_OUTPUT
	PERF_EVENT_IOC_DISABLE     = linux.PERF_EVENT_IOC_DISABLE
	PERF_EVENT_IOC_ENABLE      = linux.PERF_EVENT_IOC_ENABLE
	PERF_EVENT_IOC_SET_BPF     = linux.PERF_EVENT_IOC_SET_BPF
	PerfBitWatermark           = linux.PerfBitWatermark
	PerfBitWriteBackward       = linux.PerfBitWriteBackward
	PERF_SAMPLE_RAW            = linux.PERF_SAMPLE_RAW
	PERF_FLAG_FD_CLOEXEC       = linux.PERF_FLAG_FD_CLOEXEC
	RLIM_INFINITY              = linux.RLIM_INFINITY
	RLIMIT_MEMLOCK             = linux.RLIMIT_MEMLOCK
	BPF_STATS_RUN_TIME         = linux.BPF_STATS_RUN_TIME
	PERF_RECORD_LOST           = linux.PERF_RECORD_LOST
	PERF_RECORD_SAMPLE         = linux.PERF_RECORD_SAMPLE
	AT_FDCWD                   = linux.AT_FDCWD
	RENAME_NOREPLACE           = linux.RENAME_NOREPLACE
	SO_ATTACH_BPF              = linux.SO_ATTACH_BPF
	SO_DETACH_BPF              = linux.SO_DETACH_BPF
	SOL_SOCKET                 = linux.SOL_SOCKET
	SIGPROF                    = linux.SIGPROF
	SIGUSR1                    = linux.SIGUSR1
	SIG_BLOCK                  = linux.SIG_BLOCK
	SIG_UNBLOCK                = linux.SIG_UNBLOCK
	BPF_FS_MAGIC               = linux.BPF_FS_MAGIC
	TRACEFS_MAGIC              = linux.TRACEFS_MAGIC
	DEBUGFS_MAGIC              = linux.DEBUGFS_MAGIC
	BPF_RB_NO_WAKEUP           = linux.BPF_RB_NO_WAKEUP
	BPF_RB_FORCE_WAKEUP        = linux.BPF_RB_FORCE_WAKEUP
	AF_UNSPEC                  = linux.AF_UNSPEC
	IFF_UP                     = linux.IFF_UP
	LINUX_CAPABILITY_VERSION_3 = linux.LINUX_CAPABILITY_VERSION_3
	CLONE_NEWNET               = linux.CLONE_NEWNET
	CLONE_NEWUSER              = linux.CLONE_NEWUSER
	CLONE_NEWNS                = linux.CLONE_NEWNS
	MOVE_MOUNT_F_EMPTY_PATH    = linux.MOVE_MOUNT_F_EMPTY_PATH
	AF_UNIX                    = linux.AF_UNIX
	SOCK_STREAM                = linux.SOCK_STREAM
	SOCK_CLOEXEC               = linux.SOCK_CLOEXEC
	FSOPEN_CLOEXEC             = linux.FSOPEN_CLOEXEC
	FSMOUNT_CLOEXEC            = linux.FSMOUNT_CLOEXEC
	MSG_CMSG_CLOEXEC           = linux.MSG_CMSG_CLOEXEC
	SizeofInt                  = linux.SizeofInt
)

type Statfs_t = linux.Statfs_t
type Stat_t = linux.Stat_t
type Rlimit = linux.Rlimit
type Signal = linux.Signal
type Sigset_t = linux.Sigset_t
type PerfEventMmapPage = linux.PerfEventMmapPage
type EpollEvent = linux.EpollEvent
type PerfEventAttr = linux.PerfEventAttr
type Utsname = linux.Utsname
type CPUSet = linux.CPUSet
type CapUserData = linux.CapUserData
type CapUserHeader = linux.CapUserHeader
type SysProcAttr = linux.SysProcAttr

func Syscall(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return linux.Syscall(trap, a1, a2, a3)
}

func PthreadSigmask(how int, set, oldset *Sigset_t) error {
	return linux.PthreadSigmask(how, set, oldset)
}

func FcntlInt(fd uintptr, cmd, arg int) (int, error) {
	return linux.FcntlInt(fd, cmd, arg)
}

func IoctlSetInt(fd int, req uint, value int) error {
	return linux.IoctlSetInt(fd, req, value)
}

func Statfs(path string, buf *Statfs_t) (err error) {
	return linux.Statfs(path, buf)
}

func Close(fd int) (err error) {
	return linux.Close(fd)
}

func EpollWait(epfd int, events []EpollEvent, msec int) (n int, err error) {
	return linux.EpollWait(epfd, events, msec)
}

func EpollCtl(epfd int, op int, fd int, event *EpollEvent) (err error) {
	return linux.EpollCtl(epfd, op, fd, event)
}

func Eventfd(initval uint, flags int) (fd int, err error) {
	return linux.Eventfd(initval, flags)
}

func Write(fd int, p []byte) (n int, err error) {
	return linux.Write(fd, p)
}

func EpollCreate1(flag int) (fd int, err error) {
	return linux.EpollCreate1(flag)
}

func SetNonblock(fd int, nonblocking bool) (err error) {
	return linux.SetNonblock(fd, nonblocking)
}

func Mmap(fd int, offset int64, length int, prot int, flags int) (data []byte, err error) {
	return linux.Mmap(fd, offset, length, prot, flags)
}

//go:nocheckptr
func MmapPtr(fd int, offset int64, addr unsafe.Pointer, length uintptr, prot int, flags int) (ret unsafe.Pointer, err error) {
	return linux.MmapPtr(fd, offset, addr, length, prot, flags)
}

func Munmap(b []byte) (err error) {
	return linux.Munmap(b)
}

func PerfEventOpen(attr *PerfEventAttr, pid int, cpu int, groupFd int, flags int) (fd int, err error) {
	return linux.PerfEventOpen(attr, pid, cpu, groupFd, flags)
}

func Uname(buf *Utsname) (err error) {
	return linux.Uname(buf)
}

func Getpid() int {
	return linux.Getpid()
}

func Gettid() int {
	return linux.Gettid()
}

func Tgkill(tgid int, tid int, sig syscall.Signal) (err error) {
	return linux.Tgkill(tgid, tid, sig)
}

func BytePtrFromString(s string) (*byte, error) {
	return linux.BytePtrFromString(s)
}

func ByteSliceToString(s []byte) string {
	return linux.ByteSliceToString(s)
}

func ByteSliceFromString(s string) ([]byte, error) {
	return linux.ByteSliceFromString(s)
}

func Renameat2(olddirfd int, oldpath string, newdirfd int, newpath string, flags uint) error {
	return linux.Renameat2(olddirfd, oldpath, newdirfd, newpath, flags)
}

func Prlimit(pid, resource int, new, old *Rlimit) error {
	return linux.Prlimit(pid, resource, new, old)
}

func Open(path string, mode int, perm uint32) (int, error) {
	return linux.Open(path, mode, perm)
}

func Fstat(fd int, stat *Stat_t) error {
	return linux.Fstat(fd, stat)
}

func SetsockoptInt(fd, level, opt, value int) error {
	return linux.SetsockoptInt(fd, level, opt, value)
}

func SchedSetaffinity(pid int, set *CPUSet) error {
	return linux.SchedSetaffinity(pid, set)
}

func SchedGetaffinity(pid int, set *CPUSet) error {
	return linux.SchedGetaffinity(pid, set)
}

func Auxv() ([][2]uintptr, error) {
	return linux.Auxv()
}

func Unshare(flag int) error {
	return linux.Unshare(flag)
}

func Setns(fd int, nstype int) error {
	return linux.Setns(fd, nstype)
}

func Capget(hdr *CapUserHeader, data *CapUserData) (err error) {
	return linux.Capget(hdr, data)
}

func Capset(hdr *CapUserHeader, data *CapUserData) (err error) {
	return linux.Capset(hdr, data)
}

func Sendmsg(fd int, p []byte, oob []byte, to linux.Sockaddr, flags int) (err error) {
	return linux.Sendmsg(fd, p, oob, to, flags)
}

func Fsopen(fsname string, flags int) (fd int, err error) {
	return linux.Fsopen(fsname, flags)
}

func FsconfigSetString(fd int, key string, value string) error {
	return linux.FsconfigSetString(fd, key, value)
}

func FsconfigCreate(fd int) (err error) {
	return linux.FsconfigCreate(fd)
}

func Fsmount(fd int, flags int, mountAttrs int) (fsfd int, err error) {
	return linux.Fsmount(fd, flags, mountAttrs)
}

func MoveMount(fromDirfd int, fromPathName string, toDirfd int, toPathName string, flags int) (err error) {
	return linux.MoveMount(fromDirfd, fromPathName, toDirfd, toPathName, flags)
}

func UnixRights(fds ...int) []byte {
	return linux.UnixRights(fds...)
}

func Recvmsg(fd int, p []byte, oob []byte, flags int) (n int, oobn int, recvflags int, from linux.Sockaddr, err error) {
	return linux.Recvmsg(fd, p, oob, flags)
}

func Socketpair(domain int, typ int, proto int) (fd [2]int, err error) {
	return linux.Socketpair(domain, typ, proto)
}

func CmsgSpace(datalen int) int {
	return linux.CmsgSpace(datalen)
}

func ParseSocketControlMessage(b []byte) ([]linux.SocketControlMessage, error) {
	return linux.ParseSocketControlMessage(b)
}

func ParseUnixRights(m *linux.SocketControlMessage) ([]int, error) {
	return linux.ParseUnixRights(m)
}
