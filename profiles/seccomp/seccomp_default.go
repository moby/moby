// +build linux,seccomp

package seccomp

import (
	"syscall"

	"github.com/opencontainers/runc/libcontainer/configs"
	libseccomp "github.com/seccomp/libseccomp-golang"
)

func arches() []string {
	var native, err = libseccomp.GetNativeArch()
	if err != nil {
		return []string{}
	}
	var a = native.String()
	switch a {
	case "amd64":
		return []string{"amd64", "x86", "x32"}
	case "arm64":
		return []string{"arm64", "arm"}
	case "mips64":
		return []string{"mips64", "mips64n32", "mips"}
	case "mips64n32":
		return []string{"mips64", "mips64n32", "mips"}
	case "mipsel64":
		return []string{"mipsel64", "mipsel64n32", "mipsel"}
	case "mipsel64n32":
		return []string{"mipsel64", "mipsel64n32", "mipsel"}
	default:
		return []string{a}
	}
}

var defaultSeccompProfile = &configs.Seccomp{
	DefaultAction: configs.Errno,
	Architectures: arches(),
	Syscalls: []*configs.Syscall{
		{
			Name:   "accept",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "accept4",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "access",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "alarm",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "arch_prctl",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "bind",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "brk",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "capget",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "capset",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "chdir",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "chmod",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "chown",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "chown32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "chroot",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "clock_getres",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "clock_gettime",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "clock_nanosleep",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "clone",
			Action: configs.Allow,
			Args: []*configs.Arg{
				{
					Index:    0,
					Value:    syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID | syscall.CLONE_NEWNET,
					ValueTwo: 0,
					Op:       configs.MaskEqualTo,
				},
			},
		},
		{
			Name:   "close",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "connect",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "creat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "dup",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "dup2",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "dup3",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "epoll_create",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "epoll_create1",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "epoll_ctl",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "epoll_ctl_old",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "epoll_pwait",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "epoll_wait",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "epoll_wait_old",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "eventfd",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "eventfd2",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "execve",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "execveat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "exit",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "exit_group",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "faccessat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fadvise64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fadvise64_64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fallocate",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fanotify_init",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fanotify_mark",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fchdir",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fchmod",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fchmodat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fchown",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fchown32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fchownat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fcntl",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fcntl64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fdatasync",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fgetxattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "flistxattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "flock",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fork",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fremovexattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fsetxattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fstat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fstat64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fstatat64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fstatfs",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fstatfs64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "fsync",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "ftruncate",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "ftruncate64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "futex",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "futimesat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getcpu",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getcwd",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getdents",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getdents64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getegid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getegid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "geteuid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "geteuid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getgid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getgid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getgroups",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getgroups32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getitimer",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getpeername",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getpgid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getpgrp",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getpid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getppid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getpriority",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getrandom",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getresgid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getresgid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getresuid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getresuid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getrlimit",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "get_robust_list",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getrusage",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getsid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getsockname",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getsockopt",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "get_thread_area",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "gettid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "gettimeofday",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getuid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getuid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "getxattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "inotify_add_watch",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "inotify_init",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "inotify_init1",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "inotify_rm_watch",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "io_cancel",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "ioctl",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "io_destroy",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "io_getevents",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "ioprio_get",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "ioprio_set",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "io_setup",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "io_submit",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "kill",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "lchown",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "lchown32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "lgetxattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "link",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "linkat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "listen",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "listxattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "llistxattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "_llseek",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "lremovexattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "lseek",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "lsetxattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "lstat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "lstat64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "madvise",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "memfd_create",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mincore",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mkdir",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mkdirat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mknod",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mknodat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mlock",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mlockall",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mmap",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mmap2",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mprotect",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mq_getsetattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mq_notify",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mq_open",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mq_timedreceive",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mq_timedsend",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mq_unlink",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "mremap",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "msgctl",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "msgget",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "msgrcv",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "msgsnd",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "msync",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "munlock",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "munlockall",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "munmap",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "nanosleep",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "newfstatat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "_newselect",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "open",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "openat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "pause",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "pipe",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "pipe2",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "poll",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "ppoll",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "prctl",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "pread64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "preadv",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "prlimit64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "pselect6",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "pwrite64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "pwritev",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "read",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "readahead",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "readlink",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "readlinkat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "readv",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "recv",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "recvfrom",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "recvmmsg",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "recvmsg",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "remap_file_pages",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "removexattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "rename",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "renameat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "renameat2",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "rmdir",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "rt_sigaction",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "rt_sigpending",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "rt_sigprocmask",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "rt_sigqueueinfo",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "rt_sigreturn",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "rt_sigsuspend",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "rt_sigtimedwait",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "rt_tgsigqueueinfo",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_getaffinity",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_getattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_getparam",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_get_priority_max",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_get_priority_min",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_getscheduler",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_rr_get_interval",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_setaffinity",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_setattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_setparam",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_setscheduler",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sched_yield",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "seccomp",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "select",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "semctl",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "semget",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "semop",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "semtimedop",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "send",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sendfile",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sendfile64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sendmmsg",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sendmsg",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sendto",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setdomainname",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setfsgid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setfsgid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setfsuid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setfsuid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setgid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setgid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setgroups",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setgroups32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sethostname",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setitimer",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setpgid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setpriority",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setregid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setregid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setresgid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setresgid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setresuid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setresuid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setreuid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setreuid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setrlimit",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "set_robust_list",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setsid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setsockopt",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "set_thread_area",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "set_tid_address",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setuid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setuid32",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "setxattr",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "shmat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "shmctl",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "shmdt",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "shmget",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "shutdown",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sigaltstack",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "signalfd",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "signalfd4",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sigreturn",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "socket",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "socketpair",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "splice",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "stat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "stat64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "statfs",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "statfs64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "symlink",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "symlinkat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sync",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sync_file_range",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "syncfs",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "sysinfo",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "syslog",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "tee",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "tgkill",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "time",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "timer_create",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "timer_delete",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "timerfd_create",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "timerfd_gettime",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "timerfd_settime",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "timer_getoverrun",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "timer_gettime",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "timer_settime",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "times",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "tkill",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "truncate",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "truncate64",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "ugetrlimit",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "umask",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "uname",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "unlink",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "unlinkat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "utime",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "utimensat",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "utimes",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "vfork",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "vhangup",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "vmsplice",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "wait4",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "waitid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "waitpid",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "write",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "writev",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		// i386 specific syscalls
		{
			Name:   "modify_ldt",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		// arm specific syscalls
		{
			Name:   "breakpoint",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "cacheflush",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
		{
			Name:   "set_tls",
			Action: configs.Allow,
			Args:   []*configs.Arg{},
		},
	},
}
