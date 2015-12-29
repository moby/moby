// +build linux,seccomp

package native

import "github.com/opencontainers/runc/libcontainer/configs"

var defaultSeccompProfile = &configs.Seccomp{
	DefaultAction: configs.Allow,
	Syscalls: []*configs.Syscall{
		{
			// Quota and Accounting syscalls which could let containers
			// disable their own resource limits or process accounting
			Name:   "acct",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Prevent containers from using the kernel keyring,
			// which is not namespaced
			Name:   "add_key",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Similar to clock_settime and settimeofday
			// Time/Date is not namespaced
			Name:   "adjtimex",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny loading potentially persistent bpf programs into kernel
			// already gated by CAP_SYS_ADMIN
			Name:   "bpf",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Time/Date is not namespaced
			Name:   "clock_adjtime",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Time/Date is not namespaced
			Name:   "clock_settime",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny cloning new namespaces
			Name:   "clone",
			Action: configs.Errno,
			Args: []*configs.Arg{
				{
					// flags from sched.h
					// CLONE_NEWUTS		0x04000000
					// CLONE_NEWIPC		0x08000000
					// CLONE_NEWUSER	0x10000000
					// CLONE_NEWPID		0x20000000
					// CLONE_NEWNET		0x40000000
					Index: 0,
					Value: uint64(0x04000000),
					Op:    configs.GreaterThanOrEqualTo,
				},
				{
					// flags from sched.h
					// CLONE_NEWNS		0x00020000
					Index: 0,
					Value: uint64(0x00020000),
					Op:    configs.EqualTo,
				},
			},
		},
		{
			// Deny manipulation and functions on kernel modules.
			Name:   "create_module",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny manipulation and functions on kernel modules.
			Name:   "delete_module",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny manipulation and functions on kernel modules.
			Name:   "finit_module",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny retrieval of exported kernel and module symbols
			Name:   "get_kernel_syms",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Terrifying syscalls that modify kernel memory and NUMA settings.
			// They're gated by CAP_SYS_NICE,
			// which we do not retain by default in containers.
			Name:   "get_mempolicy",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny manipulation and functions on kernel modules.
			Name:   "init_module",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Prevent containers from modifying kernel I/O privilege levels.
			// Already restricted as containers drop CAP_SYS_RAWIO by default.
			Name:   "ioperm",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Prevent containers from modifying kernel I/O privilege levels.
			// Already restricted as containers drop CAP_SYS_RAWIO by default.
			Name:   "iopl",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Restrict process inspection capabilities
			// Already blocked by dropping CAP_PTRACE
			Name:   "kcmp",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Sister syscall of kexec_load that does the same thing,
			// slightly different arguments
			Name:   "kexec_file_load",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny loading a new kernel for later execution
			Name:   "kexec_load",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Prevent containers from using the kernel keyring,
			// which is not namespaced
			Name:   "keyctl",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Tracing/profiling syscalls,
			// which could leak a lot of information on the host
			Name:   "lookup_dcookie",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Terrifying syscalls that modify kernel memory and NUMA settings.
			// They're gated by CAP_SYS_NICE,
			// which we do not retain by default in containers.
			Name:   "mbind",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Terrifying syscalls that modify kernel memory and NUMA settings.
			// They're gated by CAP_SYS_NICE,
			// which we do not retain by default in containers.
			Name:   "migrate_pages",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Old syscall only used in 16-bit code,
			// and a potential information leak
			Name:   "modify_ldt",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny mount
			Name:   "mount",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Terrifying syscalls that modify kernel memory and NUMA settings.
			// They're gated by CAP_SYS_NICE,
			// which we do not retain by default in containers.
			Name:   "move_pages",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny interaction with the kernel nfs daemon
			Name:   "nfsservctl",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Cause of an old container breakout,
			// might as well restrict it to be on the safe side
			Name:   "open_by_handle_at",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Tracing/profiling syscalls,
			// which could leak a lot of information on the host
			Name:   "perf_event_open",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Prevent container from enabling BSD emulation.
			// Not inherently dangerous, but poorly tested,
			// potential for a lot of kernel vulns in this.
			Name:   "personality",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny pivot_root
			Name:   "pivot_root",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Restrict process inspection capabilities
			// Already blocked by dropping CAP_PTRACE
			Name:   "process_vm_readv",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Restrict process modification capabilities
			// Already blocked by dropping CAP_PTRACE
			Name:   "process_vm_writev",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Already blocked by dropping CAP_PTRACE
			Name:   "ptrace",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny manipulation and functions on kernel modules.
			Name:   "query_module",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Quota and Accounting syscalls which could let containers
			// disable their own resource limits or process accounting
			Name:   "quotactl",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Probably a bad idea to let containers reboot the host
			Name:   "reboot",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Probably a bad idea to let containers restart a syscall.
			// Possible seccomp bypass, see: https://code.google.com/p/chromium/issues/detail?id=408827.
			Name:   "restart_syscall",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Prevent containers from using the kernel keyring,
			// which is not namespaced
			Name:   "request_key",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Terrifying syscalls that modify kernel memory and NUMA settings.
			// They're gated by CAP_SYS_NICE,
			// which we do not retain by default in containers.
			Name:   "set_mempolicy",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// deny associating a thread with a namespace
			Name:   "setns",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Time/Date is not namespaced
			Name:   "settimeofday",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Time/Date is not namespaced
			Name:   "stime",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny start/stop swapping to file/device
			Name:   "swapon",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny start/stop swapping to file/device
			Name:   "swapoff",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny read/write system parameters
			Name:   "_sysctl",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny umount
			Name:   "umount",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Deny umount
			Name:   "umount2",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Same as clone
			Name:   "unshare",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// Older syscall related to shared libraries, unused for a long time
			Name:   "uselib",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// In kernel x86 real mode virtual machine
			Name:   "vm86",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
		{
			// In kernel x86 real mode virtual machine
			Name:   "vm86old",
			Action: configs.Errno,
			Args:   []*configs.Arg{},
		},
	},
}
