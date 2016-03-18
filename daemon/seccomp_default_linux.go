// +build linux,seccomp

package daemon

import (
	"syscall"

	"github.com/opencontainers/specs/specs-go"
	libseccomp "github.com/seccomp/libseccomp-golang"
)

func arches() []specs.Arch {
	var native, err = libseccomp.GetNativeArch()
	if err != nil {
		return []specs.Arch{}
	}
	var a = native.String()
	switch a {
	case "amd64":
		return []specs.Arch{specs.ArchX86_64, specs.ArchX86, specs.ArchX32}
	case "arm64":
		return []specs.Arch{specs.ArchAARCH64, specs.ArchARM}
	case "mips64":
		return []specs.Arch{specs.ArchMIPS, specs.ArchMIPS64, specs.ArchMIPS64N32}
	case "mips64n32":
		return []specs.Arch{specs.ArchMIPS, specs.ArchMIPS64, specs.ArchMIPS64N32}
	case "mipsel64":
		return []specs.Arch{specs.ArchMIPSEL, specs.ArchMIPSEL64, specs.ArchMIPSEL64N32}
	case "mipsel64n32":
		return []specs.Arch{specs.ArchMIPSEL, specs.ArchMIPSEL64, specs.ArchMIPSEL64N32}
	default:
		return []specs.Arch{}
	}
}

var defaultSeccompProfile = specs.Seccomp{
	DefaultAction: specs.ActErrno,
	Architectures: arches(),
	Syscalls: []specs.Syscall{
		{
			Name:   "accept",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "accept4",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "access",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "alarm",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "arch_prctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "bind",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "brk",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "capget",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "capset",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "chdir",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "chmod",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "chown",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "chown32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "chroot",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "clock_getres",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "clock_gettime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "clock_nanosleep",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "clone",
			Action: specs.ActAllow,
			Args: []specs.Arg{
				{
					Index:    0,
					Value:    syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID | syscall.CLONE_NEWNET,
					ValueTwo: 0,
					Op:       specs.OpMaskedEqual,
				},
			},
		},
		{
			Name:   "close",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "connect",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "creat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "dup",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "dup2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "dup3",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "epoll_create",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "epoll_create1",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "epoll_ctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "epoll_ctl_old",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "epoll_pwait",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "epoll_wait",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "epoll_wait_old",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "eventfd",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "eventfd2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "execve",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "execveat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "exit",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "exit_group",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "faccessat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fadvise64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fadvise64_64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fallocate",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fanotify_init",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fanotify_mark",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fchdir",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fchmod",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fchmodat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fchown",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fchown32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fchownat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fcntl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fcntl64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fdatasync",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fgetxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "flistxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "flock",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fork",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fremovexattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fsetxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fstat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fstat64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fstatat64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fstatfs",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fstatfs64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "fsync",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "ftruncate",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "ftruncate64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "futex",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "futimesat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getcpu",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getcwd",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getdents",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getdents64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getegid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getegid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "geteuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "geteuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getgid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getgroups",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getgroups32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getitimer",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getpeername",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getpgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getpgrp",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getpid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getppid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getpriority",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getrandom",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getresgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getresgid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getresuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getresuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getrlimit",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "get_robust_list",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getrusage",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getsid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getsockname",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getsockopt",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "get_thread_area",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "gettid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "gettimeofday",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "getxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "inotify_add_watch",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "inotify_init",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "inotify_init1",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "inotify_rm_watch",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "io_cancel",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "ioctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "io_destroy",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "io_getevents",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "ioprio_get",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "ioprio_set",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "io_setup",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "io_submit",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "kill",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "lchown",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "lchown32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "lgetxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "link",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "linkat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "listen",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "listxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "llistxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "_llseek",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "lremovexattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "lseek",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "lsetxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "lstat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "lstat64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "madvise",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "memfd_create",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mincore",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mkdir",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mkdirat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mknod",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mknodat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mlock",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mlockall",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mmap",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mmap2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mprotect",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mq_getsetattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mq_notify",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mq_open",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mq_timedreceive",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mq_timedsend",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mq_unlink",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "mremap",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "msgctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "msgget",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "msgrcv",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "msgsnd",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "msync",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "munlock",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "munlockall",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "munmap",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "nanosleep",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "newfstatat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "_newselect",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "open",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "openat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "pause",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "pipe",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "pipe2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "poll",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "ppoll",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "prctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "pread64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "preadv",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "prlimit64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "pselect6",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "pwrite64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "pwritev",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "read",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "readahead",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "readlink",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "readlinkat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "readv",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "recv",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "recvfrom",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "recvmmsg",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "recvmsg",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "remap_file_pages",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "removexattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "rename",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "renameat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "renameat2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "rmdir",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "rt_sigaction",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "rt_sigpending",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "rt_sigprocmask",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "rt_sigqueueinfo",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "rt_sigreturn",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "rt_sigsuspend",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "rt_sigtimedwait",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "rt_tgsigqueueinfo",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_getaffinity",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_getattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_getparam",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_get_priority_max",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_get_priority_min",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_getscheduler",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_rr_get_interval",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_setaffinity",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_setattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_setparam",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_setscheduler",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sched_yield",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "seccomp",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "select",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "semctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "semget",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "semop",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "semtimedop",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "send",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sendfile",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sendfile64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sendmmsg",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sendmsg",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sendto",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setdomainname",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setfsgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setfsgid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setfsuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setfsuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setgid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setgroups",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setgroups32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sethostname",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setitimer",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setpgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setpriority",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setregid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setregid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setresgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setresgid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setresuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setresuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setreuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setreuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setrlimit",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "set_robust_list",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setsid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setsockopt",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "set_thread_area",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "set_tid_address",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "setxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "shmat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "shmctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "shmdt",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "shmget",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "shutdown",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sigaltstack",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "signalfd",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "signalfd4",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sigreturn",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "socket",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "socketpair",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "splice",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "stat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "stat64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "statfs",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "statfs64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "symlink",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "symlinkat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sync",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sync_file_range",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "syncfs",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "sysinfo",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "syslog",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "tee",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "tgkill",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "time",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "timer_create",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "timer_delete",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "timerfd_create",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "timerfd_gettime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "timerfd_settime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "timer_getoverrun",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "timer_gettime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "timer_settime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "times",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "tkill",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "truncate",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "truncate64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "ugetrlimit",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "umask",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "uname",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "unlink",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "unlinkat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "utime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "utimensat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "utimes",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "vfork",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "vhangup",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "vmsplice",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "wait4",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "waitid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "waitpid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "write",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "writev",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		// i386 specific syscalls
		{
			Name:   "modify_ldt",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		// arm specific syscalls
		{
			Name:   "breakpoint",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "cacheflush",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		{
			Name:   "set_tls",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
	},
}
