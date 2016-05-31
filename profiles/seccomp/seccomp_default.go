// +build linux,seccomp

package seccomp

import (
	"syscall"

	"github.com/docker/engine-api/types"
	"github.com/opencontainers/specs/specs-go"
	libseccomp "github.com/seccomp/libseccomp-golang"
)

func arches() []types.Arch {
	var native, err = libseccomp.GetNativeArch()
	if err != nil {
		return []types.Arch{}
	}
	var a = native.String()
	switch a {
	case "amd64":
		return []types.Arch{types.ArchX86_64, types.ArchX86, types.ArchX32}
	case "arm64":
		return []types.Arch{types.ArchARM, types.ArchAARCH64}
	case "mips64":
		return []types.Arch{types.ArchMIPS, types.ArchMIPS64, types.ArchMIPS64N32}
	case "mips64n32":
		return []types.Arch{types.ArchMIPS, types.ArchMIPS64, types.ArchMIPS64N32}
	case "mipsel64":
		return []types.Arch{types.ArchMIPSEL, types.ArchMIPSEL64, types.ArchMIPSEL64N32}
	case "mipsel64n32":
		return []types.Arch{types.ArchMIPSEL, types.ArchMIPSEL64, types.ArchMIPSEL64N32}
	case "s390x":
		return []types.Arch{types.ArchS390, types.ArchS390X}
	default:
		return []types.Arch{}
	}
}

// DefaultProfile defines the whitelist for the default seccomp profile.
func DefaultProfile(rs *specs.Spec) *types.Seccomp {

	syscalls := []*types.Syscall{
		{
			Name:   "accept",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "accept4",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "access",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "alarm",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "bind",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "brk",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "capget",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "capset",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "chdir",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "chmod",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "chown",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "chown32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},

		{
			Name:   "clock_getres",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "clock_gettime",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "clock_nanosleep",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "close",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "connect",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "copy_file_range",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "creat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "dup",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "dup2",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "dup3",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "epoll_create",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "epoll_create1",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "epoll_ctl",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "epoll_ctl_old",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "epoll_pwait",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "epoll_wait",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "epoll_wait_old",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "eventfd",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "eventfd2",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "execve",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "execveat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "exit",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "exit_group",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "faccessat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fadvise64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fadvise64_64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fallocate",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fanotify_mark",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fchdir",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fchmod",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fchmodat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fchown",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fchown32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fchownat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fcntl",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fcntl64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fdatasync",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fgetxattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "flistxattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "flock",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fork",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fremovexattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fsetxattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fstat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fstat64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fstatat64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fstatfs",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fstatfs64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "fsync",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "ftruncate",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "ftruncate64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "futex",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "futimesat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getcpu",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getcwd",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getdents",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getdents64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getegid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getegid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "geteuid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "geteuid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getgid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getgid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getgroups",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getgroups32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getitimer",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getpeername",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getpgid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getpgrp",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getpid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getppid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getpriority",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getrandom",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getresgid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getresgid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getresuid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getresuid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getrlimit",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "get_robust_list",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getrusage",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getsid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getsockname",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getsockopt",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "get_thread_area",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "gettid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "gettimeofday",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getuid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getuid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "getxattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "inotify_add_watch",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "inotify_init",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "inotify_init1",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "inotify_rm_watch",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "io_cancel",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "ioctl",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "io_destroy",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "io_getevents",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "ioprio_get",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "ioprio_set",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "io_setup",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "io_submit",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "ipc",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "kill",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "lchown",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "lchown32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "lgetxattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "link",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "linkat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "listen",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "listxattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "llistxattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "_llseek",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "lremovexattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "lseek",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "lsetxattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "lstat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "lstat64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "madvise",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "memfd_create",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mincore",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mkdir",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mkdirat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mknod",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mknodat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mmap",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mmap2",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mprotect",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mq_getsetattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mq_notify",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mq_open",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mq_timedreceive",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mq_timedsend",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mq_unlink",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "mremap",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "msgctl",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "msgget",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "msgrcv",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "msgsnd",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "msync",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "munlock",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "munlockall",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "munmap",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "nanosleep",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "newfstatat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "_newselect",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "open",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "openat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "pause",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "personality",
			Action: types.ActAllow,
			Args: []*types.Arg{
				{
					Index: 0,
					Value: 0x0,
					Op:    types.OpEqualTo,
				},
			},
		},
		{
			Name:   "personality",
			Action: types.ActAllow,
			Args: []*types.Arg{
				{
					Index: 0,
					Value: 0x0008,
					Op:    types.OpEqualTo,
				},
			},
		},
		{
			Name:   "personality",
			Action: types.ActAllow,
			Args: []*types.Arg{
				{
					Index: 0,
					Value: 0xffffffff,
					Op:    types.OpEqualTo,
				},
			},
		},
		{
			Name:   "pipe",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "pipe2",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "poll",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "ppoll",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "prctl",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "pread64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "preadv",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "prlimit64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "pselect6",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "pwrite64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "pwritev",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "read",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "readahead",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "readlink",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "readlinkat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "readv",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "recv",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "recvfrom",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "recvmmsg",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "recvmsg",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "remap_file_pages",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "removexattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "rename",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "renameat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "renameat2",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "restart_syscall",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "rmdir",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "rt_sigaction",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "rt_sigpending",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "rt_sigprocmask",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "rt_sigqueueinfo",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "rt_sigreturn",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "rt_sigsuspend",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "rt_sigtimedwait",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "rt_tgsigqueueinfo",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_getaffinity",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_getattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_getparam",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_get_priority_max",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_get_priority_min",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_getscheduler",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_rr_get_interval",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_setaffinity",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_setattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_setparam",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_setscheduler",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sched_yield",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "seccomp",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "select",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "semctl",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "semget",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "semop",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "semtimedop",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "send",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sendfile",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sendfile64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sendmmsg",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sendmsg",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sendto",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setfsgid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setfsgid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setfsuid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setfsuid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setgid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setgid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setgroups",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setgroups32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setitimer",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setpgid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setpriority",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setregid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setregid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setresgid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setresgid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setresuid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setresuid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setreuid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setreuid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setrlimit",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "set_robust_list",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setsid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setsockopt",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "set_thread_area",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "set_tid_address",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setuid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setuid32",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "setxattr",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "shmat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "shmctl",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "shmdt",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "shmget",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "shutdown",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sigaltstack",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "signalfd",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "signalfd4",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sigreturn",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "socket",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "socketcall",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "socketpair",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "splice",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "stat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "stat64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "statfs",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "statfs64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "symlink",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "symlinkat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sync",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sync_file_range",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "syncfs",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "sysinfo",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "syslog",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "tee",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "tgkill",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "time",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "timer_create",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "timer_delete",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "timerfd_create",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "timerfd_gettime",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "timerfd_settime",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "timer_getoverrun",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "timer_gettime",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "timer_settime",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "times",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "tkill",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "truncate",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "truncate64",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "ugetrlimit",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "umask",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "uname",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "unlink",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "unlinkat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "utime",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "utimensat",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "utimes",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "vfork",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "vmsplice",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "wait4",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "waitid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "waitpid",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "write",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
		{
			Name:   "writev",
			Action: types.ActAllow,
			Args:   []*types.Arg{},
		},
	}

	var sysCloneFlagsIndex uint
	var arch string
	var native, err = libseccomp.GetNativeArch()
	if err == nil {
		arch = native.String()
	}
	switch arch {
	case "arm", "arm64":
		syscalls = append(syscalls, []*types.Syscall{
			{
				Name:   "breakpoint",
				Action: types.ActAllow,
				Args:   []*types.Arg{},
			},
			{
				Name:   "cacheflush",
				Action: types.ActAllow,
				Args:   []*types.Arg{},
			},
			{
				Name:   "set_tls",
				Action: types.ActAllow,
				Args:   []*types.Arg{},
			},
		}...)
	case "amd64", "x32":
		syscalls = append(syscalls, []*types.Syscall{
			{
				Name:   "arch_prctl",
				Action: types.ActAllow,
				Args:   []*types.Arg{},
			},
		}...)
		fallthrough
	case "x86":
		syscalls = append(syscalls, []*types.Syscall{
			{
				Name:   "modify_ldt",
				Action: types.ActAllow,
				Args:   []*types.Arg{},
			},
		}...)
	case "s390", "s390x":
		syscalls = append(syscalls, []*types.Syscall{
			{
				Name:   "s390_pci_mmio_read",
				Action: types.ActAllow,
				Args:   []*types.Arg{},
			},
			{
				Name:   "s390_pci_mmio_write",
				Action: types.ActAllow,
				Args:   []*types.Arg{},
			},
			{
				Name:   "s390_runtime_instr",
				Action: types.ActAllow,
				Args:   []*types.Arg{},
			},
		}...)
		/* Flags parameter of the clone syscall is the 2nd on s390 */
		sysCloneFlagsIndex = 1
	}

	capSysAdmin := false

	var cap string
	for _, cap = range rs.Process.Capabilities {
		switch cap {
		case "CAP_DAC_READ_SEARCH":
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "name_to_handle_at",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "open_by_handle_at",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		case "CAP_IPC_LOCK":
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "mlock",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "mlock2",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "mlockall",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		case "CAP_SYS_ADMIN":
			capSysAdmin = true
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "bpf",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "clone",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "fanotify_init",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "lookup_dcookie",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "mount",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "perf_event_open",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "setdomainname",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "sethostname",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "setns",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "umount",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "umount2",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "unshare",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		case "CAP_SYS_BOOT":
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "reboot",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		case "CAP_SYS_CHROOT":
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "chroot",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		case "CAP_SYS_MODULE":
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "delete_module",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "init_module",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "finit_module",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "query_module",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		case "CAP_SYS_PACCT":
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "acct",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		case "CAP_SYS_PTRACE":
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "kcmp",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "process_vm_readv",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "process_vm_writev",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "ptrace",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		case "CAP_SYS_RAWIO":
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "iopl",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "ioperm",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		case "CAP_SYS_TIME":
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "settimeofday",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "stime",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
				{
					Name:   "adjtimex",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		case "CAP_SYS_TTY_CONFIG":
			syscalls = append(syscalls, []*types.Syscall{
				{
					Name:   "vhangup",
					Action: types.ActAllow,
					Args:   []*types.Arg{},
				},
			}...)
		}
	}

	if !capSysAdmin {
		syscalls = append(syscalls, []*types.Syscall{
			{
				Name:   "clone",
				Action: types.ActAllow,
				Args: []*types.Arg{
					{
						Index:    sysCloneFlagsIndex,
						Value:    syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID | syscall.CLONE_NEWNET,
						ValueTwo: 0,
						Op:       types.OpMaskedEqual,
					},
				},
			},
		}...)
	}

	return &types.Seccomp{
		DefaultAction: types.ActErrno,
		Architectures: arches(),
		Syscalls:      syscalls,
	}
}
