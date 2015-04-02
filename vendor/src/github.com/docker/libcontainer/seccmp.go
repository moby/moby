// +build linux

package libcontainer

/*
#include <linux/filter.h>
#include <linux/seccomp.h>
#include <stddef.h>
#include <string.h>
#include <sys/prctl.h>
#include <malloc.h>

struct scmp_map {
    int syscall;
    int action;
};

static int scmp_filter(struct scmp_map **filter, int num)
{
    struct sock_filter *sec_filter = malloc(sizeof(struct sock_filter) * (num * 2 + 3));
    if (sec_filter) {
		struct sock_filter scmp_head[] = {
	        BPF_STMT(BPF_LD + BPF_W + BPF_ABS, offsetof(struct seccomp_data, nr)),
	    };
	    memcpy(sec_filter, scmp_head, sizeof(scmp_head));

	    int i = 0;
	    for ( ; i < num; i++)
	    {
	        struct sock_filter node[] = {
	            BPF_JUMP(BPF_JMP + BPF_JEQ + BPF_K, (*filter)[i].syscall, 0, 1),
	            BPF_STMT(BPF_RET + BPF_K, SECCOMP_RET_ALLOW),
	        };
	        memcpy(&sec_filter[1 + i * 2], node, sizeof(node));
	    }

	    struct sock_filter scmp_end[] = {
	        BPF_STMT(BPF_RET + BPF_K, SECCOMP_RET_TRAP),
	        BPF_STMT(BPF_RET + BPF_K, SECCOMP_RET_KILL),
	    };
	    memcpy(&sec_filter[1 + num * 2], scmp_end, sizeof(scmp_end));

	    struct sock_fprog prog = {
	        .len = (unsigned short)(num * 2 + 3),
	        .filter = sec_filter,
	    };
	    if ((!prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0))
	        && (!prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, &prog))) {
	        free(sec_filter);
		    return 0;
	    }
	    free(sec_filter);
    }
    return 1;
}
*/
import "C"

import (
    "syscall"
    "errors"
    "unsafe"
)

type Action struct {
    syscall int
    action  int
    args string
}

type ScmpCtx struct {
    CallMap map[string] Action
}

var SyscallMap = map[string]int{
    "read":                     syscall.SYS_READ,                              // 0
	"write":                    syscall.SYS_WRITE,                             // 1
	"open":                     syscall.SYS_OPEN,                              // 2
	"close":                    syscall.SYS_CLOSE,                             // 3
	"stat":                     syscall.SYS_STAT,                              // 4
	"fstat":                    syscall.SYS_FSTAT,                             // 5
	"lstat":                    syscall.SYS_LSTAT,                             // 6
	"poll":                     syscall.SYS_POLL,                              // 7
	"lseek":                    syscall.SYS_LSEEK,                             // 8
	"mmap":                     syscall.SYS_MMAP,                              // 9
	"mprotect":                 syscall.SYS_MPROTECT,                          // 10
	"munmap":                   syscall.SYS_MUNMAP,                            // 11
	"brk":                      syscall.SYS_BRK,                               // 12
	"rt_sigaction":             syscall.SYS_RT_SIGACTION,                      // 13
	"rt_sigprocmask":           syscall.SYS_RT_SIGPROCMASK,                    // 14
	"rt_sigreturn":             syscall.SYS_RT_SIGRETURN,                      // 15
	"ioctl":                    syscall.SYS_IOCTL,                             // 16
	"pread64":                  syscall.SYS_PREAD64,                           // 17
	"pwrite64":                 syscall.SYS_PWRITE64,                          // 18
	"readv":                    syscall.SYS_READV,                             // 19
	"writev":                   syscall.SYS_WRITEV,                            // 20
	"access":                   syscall.SYS_ACCESS,                            // 21
	"pipe":                     syscall.SYS_PIPE,                              // 22
	"select":                   syscall.SYS_SELECT,                            // 23
	"sched_yield":              syscall.SYS_SCHED_YIELD,                       // 24
	"mremap":                   syscall.SYS_MREMAP,                            // 25
	"msync":                    syscall.SYS_MSYNC,                             // 26
	"mincore":                  syscall.SYS_MINCORE,                           // 27
	"madvise":                  syscall.SYS_MADVISE,                           // 28
	"shmget":                   syscall.SYS_SHMGET,                            // 29
	"shmat":                    syscall.SYS_SHMAT,                             // 30
	"shmctl":                   syscall.SYS_SHMCTL,                            // 31
	"dup":                      syscall.SYS_DUP,                               // 32
	"dup2":                     syscall.SYS_DUP2,                              // 33
	"pause":                    syscall.SYS_PAUSE,                             // 34
	"nanosleep":                syscall.SYS_NANOSLEEP,                         // 35
	"getitimer":                syscall.SYS_GETITIMER,                         // 36
	"alarm":                    syscall.SYS_ALARM,                             // 37
	"setitimer":                syscall.SYS_SETITIMER,                         // 38
	"getpid":                   syscall.SYS_GETPID,                            // 39
	"sendfile":                 syscall.SYS_SENDFILE,                          // 40
	"socket":                   syscall.SYS_SOCKET,                            // 41
	"connect":                  syscall.SYS_CONNECT,                           // 42
	"accept":                   syscall.SYS_ACCEPT,                            // 43
	"sendto":                   syscall.SYS_SENDTO,                            // 44
	"recvfrom":                 syscall.SYS_RECVFROM,                          // 45
	"sendmsg":                  syscall.SYS_SENDMSG,                           // 46
	"recvmsg":                  syscall.SYS_RECVMSG,                           // 47
	"shutdown":                 syscall.SYS_SHUTDOWN,                          // 48
	"bind":                     syscall.SYS_BIND,                              // 49
	"listen":                   syscall.SYS_LISTEN,                            // 50
	"getsockname":              syscall.SYS_GETSOCKNAME,                       // 51
	"getpeername":              syscall.SYS_GETPEERNAME,                       // 52
	"socketpair":               syscall.SYS_SOCKETPAIR,                        // 53
	"setsockopt":               syscall.SYS_SETSOCKOPT,                        // 54
	"getsockopt":               syscall.SYS_GETSOCKOPT,                        // 55
	"clone":                    syscall.SYS_CLONE,                             // 56
	"fork":                     syscall.SYS_FORK,                              // 57
	"vfork":                    syscall.SYS_VFORK,                             // 58
	"execve":                   syscall.SYS_EXECVE,                            // 59
	"exit":                     syscall.SYS_EXIT,                              // 60
	"wait4":                    syscall.SYS_WAIT4,                             // 61
	"kill":                     syscall.SYS_KILL,                              // 62
	"uname":                    syscall.SYS_UNAME,                             // 63
	"semget":                   syscall.SYS_SEMGET,                            // 64
	"semop":                    syscall.SYS_SEMOP,                             // 65
	"semctl":                   syscall.SYS_SEMCTL,                            // 66
	"shmdt":                    syscall.SYS_SHMDT,                             // 67
	"msgget":                   syscall.SYS_MSGGET,                            // 68
	"msgsnd":                   syscall.SYS_MSGSND,                            // 69
	"msgrcv":                   syscall.SYS_MSGRCV,                            // 70
	"msgctl":                   syscall.SYS_MSGCTL,                            // 71
	"fcntl":                    syscall.SYS_FCNTL,                             // 72
	"flock":                    syscall.SYS_FLOCK,                             // 73
	"fsync":                    syscall.SYS_FSYNC,                             // 74
	"fdatasync":                syscall.SYS_FDATASYNC,                         // 75
	"truncate":                 syscall.SYS_TRUNCATE,                          // 76
	"ftruncate":                syscall.SYS_FTRUNCATE,                         // 77
	"getdents":                 syscall.SYS_GETDENTS,                          // 78
	"getcwd":                   syscall.SYS_GETCWD,                            // 79
	"chdir":                    syscall.SYS_CHDIR,                             // 80
	"fchdir":                   syscall.SYS_FCHDIR,                            // 81
	"rename":                   syscall.SYS_RENAME,                            // 82
	"mkdir":                    syscall.SYS_MKDIR,                             // 83
	"rmdir":                    syscall.SYS_RMDIR,                             // 84
	"creat":                    syscall.SYS_CREAT,                             // 85
	"link":                     syscall.SYS_LINK,                              // 86
	"unlink":                   syscall.SYS_UNLINK,                            // 87
	"symlink":                  syscall.SYS_SYMLINK,                           // 88
	"readlink":                 syscall.SYS_READLINK,                          // 89
	"chmod":                    syscall.SYS_CHMOD,                             // 90
	"fchmod":                   syscall.SYS_FCHMOD,                            // 91
	"chown":                    syscall.SYS_CHOWN,                             // 92
	"fchown":                   syscall.SYS_FCHOWN,                            // 93
	"lchown":                   syscall.SYS_LCHOWN,                            // 94
	"umask":                    syscall.SYS_UMASK,                             // 95
	"gettimeofday":             syscall.SYS_GETTIMEOFDAY,                      // 96
	"getrlimit":                syscall.SYS_GETRLIMIT,                         // 97
	"getrusage":                syscall.SYS_GETRUSAGE,                         // 98
	"sysinfo":                  syscall.SYS_SYSINFO,                           // 99
	"times":                    syscall.SYS_TIMES,                             // 100
	"ptrace":                   syscall.SYS_PTRACE,                            // 101
	"getuid":                   syscall.SYS_GETUID,                            // 102
	"syslog":                   syscall.SYS_SYSLOG,                            // 103
	"getgid":                   syscall.SYS_GETGID,                            // 104
	"setuid":                   syscall.SYS_SETUID,                            // 105
	"setgid":                   syscall.SYS_SETGID,                            // 106
	"geteuid":                  syscall.SYS_GETEUID,                           // 107
	"getegid":                  syscall.SYS_GETEGID,                           // 108
	"setpgid":                  syscall.SYS_SETPGID,                           // 109
	"getppid":                  syscall.SYS_GETPPID,                           // 110
	"getpgrp":                  syscall.SYS_GETPGRP,                           // 111
	"setsid":                   syscall.SYS_SETSID,                            // 112
	"setreuid":                 syscall.SYS_SETREUID,                          // 113
	"setregid":                 syscall.SYS_SETREGID,                          // 114
	"getgroups":                syscall.SYS_GETGROUPS,                         // 115
	"setgroups":                syscall.SYS_SETGROUPS,                         // 116
	"setresuid":                syscall.SYS_SETRESUID,                         // 117
	"getresuid":                syscall.SYS_GETRESUID,                         // 118
	"setresgid":                syscall.SYS_SETRESGID,                         // 119
	"getresgid":                syscall.SYS_GETRESGID,                         // 120
	"getpgid":                  syscall.SYS_GETPGID,                           // 121
	"setfsuid":                 syscall.SYS_SETFSUID,                          // 122
	"setfsgid":                 syscall.SYS_SETFSGID,                          // 123
	"getsid":                   syscall.SYS_GETSID,                            // 124
	"capget":                   syscall.SYS_CAPGET,                            // 125
	"capset":                   syscall.SYS_CAPSET,                            // 126
	"rt_sigpending":            syscall.SYS_RT_SIGPENDING,                     // 127
	"rt_sigtimedwait":          syscall.SYS_RT_SIGTIMEDWAIT,                   // 128
	"rt_sigqueueinfo":          syscall.SYS_RT_SIGQUEUEINFO,                   // 129
	"rt_sigsuspend":            syscall.SYS_RT_SIGSUSPEND,                     // 130
	"sigaltstack":              syscall.SYS_SIGALTSTACK,                       // 131
	"utime":                    syscall.SYS_UTIME,                             // 132
	"mknod":                    syscall.SYS_MKNOD,                             // 133
	"uselib":                   syscall.SYS_USELIB,                            // 134
	"personality":              syscall.SYS_PERSONALITY,                       // 135
	"ustat":                    syscall.SYS_USTAT,                             // 136
	"statfs":                   syscall.SYS_STATFS,                            // 137
	"fstatfs":                  syscall.SYS_FSTATFS,                           // 138
	"sysfs":                    syscall.SYS_SYSFS,                             // 139
	"getpriority":              syscall.SYS_GETPRIORITY,                       // 140
	"setpriority":              syscall.SYS_SETPRIORITY,                       // 141
	"sched_setparam":           syscall.SYS_SCHED_SETPARAM,                    // 142
	"sched_getparam":           syscall.SYS_SCHED_GETPARAM,                    // 143
	"sched_setscheduler":       syscall.SYS_SCHED_SETSCHEDULER,                // 144
	"sched_getscheduler":       syscall.SYS_SCHED_GETSCHEDULER,                // 145
	"sched_get_priority_max":   syscall.SYS_SCHED_GET_PRIORITY_MAX,            // 146
	"sched_get_priority_min":   syscall.SYS_SCHED_GET_PRIORITY_MIN,            // 147
	"sched_rr_get_interval":    syscall.SYS_SCHED_RR_GET_INTERVAL,             // 148
	"mlock":                    syscall.SYS_MLOCK,                             // 149
	"munlock":                  syscall.SYS_MUNLOCK,                           // 150
	"mlockall":                 syscall.SYS_MLOCKALL,                          // 151
	"munlockall":               syscall.SYS_MUNLOCKALL,                        // 152
	"vhangup":                  syscall.SYS_VHANGUP,                           // 153
	"modify_ldt":               syscall.SYS_MODIFY_LDT,                        // 154
	"pivot_root":               syscall.SYS_PIVOT_ROOT,                        // 155
	"_sysctl":                  syscall.SYS__SYSCTL,                           // 156
	"prctl":                    syscall.SYS_PRCTL,                             // 157
	"arch_prctl":               syscall.SYS_ARCH_PRCTL,                        // 158
	"adjtimex":                 syscall.SYS_ADJTIMEX,                          // 159
	"setrlimit":                syscall.SYS_SETRLIMIT,                         // 160
	"chroot":                   syscall.SYS_CHROOT,                            // 161
	"sync":                     syscall.SYS_SYNC,                              // 162
	"acct":                     syscall.SYS_ACCT,                              // 163
	"settimeofday":             syscall.SYS_SETTIMEOFDAY,                      // 164
	"mount":                    syscall.SYS_MOUNT,                             // 165
	"umount2":                  syscall.SYS_UMOUNT2,                           // 166
	"swapon":                   syscall.SYS_SWAPON,                            // 167
	"swapoff":                  syscall.SYS_SWAPOFF,                           // 168
	"reboot":                   syscall.SYS_REBOOT,                            // 169
	"sethostname":              syscall.SYS_SETHOSTNAME,                       // 170
	"setdomainname":            syscall.SYS_SETDOMAINNAME,                     // 171
	"iopl":                     syscall.SYS_IOPL,                              // 172
	"ioperm":                   syscall.SYS_IOPERM,                            // 173
	"create_module":            syscall.SYS_CREATE_MODULE,                     // 174
	"init_module":              syscall.SYS_INIT_MODULE,                       // 175
	"delete_module":            syscall.SYS_DELETE_MODULE,                     // 176
	"get_kernel_syms":          syscall.SYS_GET_KERNEL_SYMS,                   // 177
	"query_module":             syscall.SYS_QUERY_MODULE,                      // 178
	"quotactl":                 syscall.SYS_QUOTACTL,                          // 179
	"nfsservctl":               syscall.SYS_NFSSERVCTL,                        // 180
	"getpmsg":                  syscall.SYS_GETPMSG,                           // 181
	"putpmsg":                  syscall.SYS_PUTPMSG,                           // 182
	"afs_syscall":              syscall.SYS_AFS_SYSCALL,                       // 183
	"tuxcall":                  syscall.SYS_TUXCALL,                           // 184
	"security":                 syscall.SYS_SECURITY,                          // 185
	"gettid":                   syscall.SYS_GETTID,                            // 186
	"readahead":                syscall.SYS_READAHEAD,                         // 187
	"setxattr":                 syscall.SYS_SETXATTR,                          // 188
	"lsetxattr":                syscall.SYS_LSETXATTR,                         // 189
	"fsetxattr":                syscall.SYS_FSETXATTR,                         // 190
	"getxattr":                 syscall.SYS_GETXATTR,                          // 191
	"lgetxattr":                syscall.SYS_LGETXATTR,                         // 192
	"fgetxattr":                syscall.SYS_FGETXATTR,                         // 193
	"listxattr":                syscall.SYS_LISTXATTR,                         // 194
	"llistxattr":               syscall.SYS_LLISTXATTR,                        // 195
	"flistxattr":               syscall.SYS_FLISTXATTR,                        // 196
	"removexattr":              syscall.SYS_REMOVEXATTR,                       // 197
	"lremovexattr":             syscall.SYS_LREMOVEXATTR,                      // 198
	"fremovexattr":             syscall.SYS_FREMOVEXATTR,                      // 199
	"tkill":                    syscall.SYS_TKILL,                             // 200
	"time":                     syscall.SYS_TIME,                              // 201
	"futex":                    syscall.SYS_FUTEX,                             // 202
	"sched_setaffinity":        syscall.SYS_SCHED_SETAFFINITY,                 // 203
	"sched_getaffinity":        syscall.SYS_SCHED_GETAFFINITY,                 // 204
	"set_thread_area":          syscall.SYS_SET_THREAD_AREA,                   // 205
	"io_setup":                 syscall.SYS_IO_SETUP,                          // 206
	"io_destroy":               syscall.SYS_IO_DESTROY,                        // 207
	"io_getevents":             syscall.SYS_IO_GETEVENTS,                      // 208
	"io_submit":                syscall.SYS_IO_SUBMIT,                         // 209
	"io_cancel":                syscall.SYS_IO_CANCEL,                         // 210
	"get_thread_area":          syscall.SYS_GET_THREAD_AREA,                   // 211
	"lookup_dcookie":           syscall.SYS_LOOKUP_DCOOKIE,                    // 212
	"epoll_create":             syscall.SYS_EPOLL_CREATE,                      // 213
	"epoll_ctl_old":            syscall.SYS_EPOLL_CTL_OLD,                     // 214
	"epoll_wait_old":           syscall.SYS_EPOLL_WAIT_OLD,                    // 215
	"remap_file_pages":         syscall.SYS_REMAP_FILE_PAGES,                  // 216
	"getdents64":               syscall.SYS_GETDENTS64,                        // 217
	"set_tid_address":          syscall.SYS_SET_TID_ADDRESS,                   // 218
	"restart_syscall":          syscall.SYS_RESTART_SYSCALL,                   // 219
	"semtimedop":               syscall.SYS_SEMTIMEDOP,                        // 220
	"fadvise64":                syscall.SYS_FADVISE64,                         // 221
	"timer_create":             syscall.SYS_TIMER_CREATE,                      // 222
	"timer_settime":            syscall.SYS_TIMER_SETTIME,                     // 223
	"timer_gettime":            syscall.SYS_TIMER_GETTIME,                     // 224
	"timer_getoverrun":         syscall.SYS_TIMER_GETOVERRUN,                  // 225
	"timer_delete":             syscall.SYS_TIMER_DELETE,                      // 226
	"clock_settime":            syscall.SYS_CLOCK_SETTIME,                     // 227
	"clock_gettime":            syscall.SYS_CLOCK_GETTIME,                     // 228
	"clock_getres":             syscall.SYS_CLOCK_GETRES,                      // 229
	"clock_nanosleep":          syscall.SYS_CLOCK_NANOSLEEP,                   // 230
	"exit_group":               syscall.SYS_EXIT_GROUP,                        // 231
	"epoll_wait":               syscall.SYS_EPOLL_WAIT,                        // 232
	"epoll_ctl":                syscall.SYS_EPOLL_CTL,                         // 233
	"tgkill":                   syscall.SYS_TGKILL,                            // 234
	"utimes":                   syscall.SYS_UTIMES,                            // 235
	"vserver":                  syscall.SYS_VSERVER,                           // 236
	"mbind":                    syscall.SYS_MBIND,                             // 237
	"set_mempolicy":            syscall.SYS_SET_MEMPOLICY,                     // 238
	"get_mempolicy":            syscall.SYS_GET_MEMPOLICY,                     // 239
	"mq_open":                  syscall.SYS_MQ_OPEN,                           // 240
	"mq_unlink":                syscall.SYS_MQ_UNLINK,                         // 241
	"mq_timedsend":             syscall.SYS_MQ_TIMEDSEND,                      // 242
	"mq_timedreceive":          syscall.SYS_MQ_TIMEDRECEIVE,                   // 243
	"mq_notify":                syscall.SYS_MQ_NOTIFY,                         // 244
	"mq_getsetattr":            syscall.SYS_MQ_GETSETATTR,                     // 245
	"kexec_load":               syscall.SYS_KEXEC_LOAD,                        // 246
	"waitid":                   syscall.SYS_WAITID,                            // 247
	"add_key":                  syscall.SYS_ADD_KEY,                           // 248
	"request_key":              syscall.SYS_REQUEST_KEY,                       // 249
	"keyctl":                   syscall.SYS_KEYCTL,                            // 250
	"ioprio_set":               syscall.SYS_IOPRIO_SET,                        // 251
	"ioprio_get":               syscall.SYS_IOPRIO_GET,                        // 252
	"inotify_init":             syscall.SYS_INOTIFY_INIT,                      // 253
	"inotify_add_watch":        syscall.SYS_INOTIFY_ADD_WATCH,                 // 254
	"inotify_rm_watch":         syscall.SYS_INOTIFY_RM_WATCH,                  // 255
	"migrate_pages":            syscall.SYS_MIGRATE_PAGES,                     // 256
	"openat":                   syscall.SYS_OPENAT,                            // 257
	"mkdirat":                  syscall.SYS_MKDIRAT,                           // 258
	"mknodat":                  syscall.SYS_MKNODAT,                           // 259
	"fchownat":                 syscall.SYS_FCHOWNAT,                          // 260
	"futimesat":                syscall.SYS_FUTIMESAT,                         // 261
	"newfstatat":               syscall.SYS_NEWFSTATAT,                        // 262
	"unlinkat":                 syscall.SYS_UNLINKAT,                          // 263
	"renameat":                 syscall.SYS_RENAMEAT,                          // 264
	"linkat":                   syscall.SYS_LINKAT,                            // 265
	"symlinkat":                syscall.SYS_SYMLINKAT,                         // 266
	"readlinkat":               syscall.SYS_READLINKAT,                        // 267
	"fchmodat":                 syscall.SYS_FCHMODAT,                          // 268
	"faccessat":                syscall.SYS_FACCESSAT,                         // 269
	"pselect6":                 syscall.SYS_PSELECT6,                          // 270
	"ppoll":                    syscall.SYS_PPOLL,                             // 271
	"unshare":                  syscall.SYS_UNSHARE,                           // 272
	"set_robust_list":          syscall.SYS_SET_ROBUST_LIST,                   // 273
	"get_robust_list":          syscall.SYS_GET_ROBUST_LIST,                   // 274
	"splice":                   syscall.SYS_SPLICE,                            // 275
	"tee":                      syscall.SYS_TEE,                               // 276
	"sync_file_range":          syscall.SYS_SYNC_FILE_RANGE,                   // 277
	"vmsplice":                 syscall.SYS_VMSPLICE,                          // 278
	"move_pages":               syscall.SYS_MOVE_PAGES,                        // 279
	"utimensat":                syscall.SYS_UTIMENSAT,                         // 280
	"epoll_pwait":              syscall.SYS_EPOLL_PWAIT,                       // 281
	"signalfd":                 syscall.SYS_SIGNALFD,                          // 282
	"timerfd_create":           syscall.SYS_TIMERFD_CREATE,                    // 283
	"eventfd":                  syscall.SYS_EVENTFD,                           // 284
	"fallocate":                syscall.SYS_FALLOCATE,                         // 285
	"timerfd_settime":          syscall.SYS_TIMERFD_SETTIME,                   // 286
	"timerfd_gettime":          syscall.SYS_TIMERFD_GETTIME,                   // 287
	"accept4":                  syscall.SYS_ACCEPT4,                           // 288
	"signalfd4":                syscall.SYS_SIGNALFD4,                         // 289
	"eventfd2":                 syscall.SYS_EVENTFD2,                          // 290
	"epoll_create1":            syscall.SYS_EPOLL_CREATE1,                     // 291
	"dup3":                     syscall.SYS_DUP3,                              // 292
	"pipe2":                    syscall.SYS_PIPE2,                             // 293
	"inotify_init1":            syscall.SYS_INOTIFY_INIT1,                     // 294
	"preadv":                   syscall.SYS_PREADV,                            // 295
	"pwritev":                  syscall.SYS_PWRITEV,                           // 296
	"rt_tgsigqueueinfo":        syscall.SYS_RT_TGSIGQUEUEINFO,                 // 297
	"perf_event_open":          syscall.SYS_PERF_EVENT_OPEN,                   // 298
	"recvmmsg":                 syscall.SYS_RECVMMSG,                          // 299
	"fanotify_init":            syscall.SYS_FANOTIFY_INIT,                     // 300
	"fanotify_mark":            syscall.SYS_FANOTIFY_MARK,                     // 301
	"prlimit64":                syscall.SYS_PRLIMIT64,                         // 302
}

var SyscallMapMin = map[string]int{
        "read":                     syscall.SYS_READ,                              // 0
	"write":                    syscall.SYS_WRITE,                             // 1
	"open":                     syscall.SYS_OPEN,                              // 2
	"close":                    syscall.SYS_CLOSE,                             // 3
	"stat":                     syscall.SYS_STAT,                              // 4
	"fstat":                    syscall.SYS_FSTAT,                             // 5
	"mmap":                     syscall.SYS_MMAP,                              // 9
	"mprotect":                 syscall.SYS_MPROTECT,                          // 10
	"munmap":                   syscall.SYS_MUNMAP,                            // 11
	"brk":                      syscall.SYS_BRK,                               // 12
	"rt_sigaction":             syscall.SYS_RT_SIGACTION,                      // 13
	"rt_sigprocmask":           syscall.SYS_RT_SIGPROCMASK,                    // 14
	"rt_sigreturn":             syscall.SYS_RT_SIGRETURN,                      // 15
	"ioctl":                    syscall.SYS_IOCTL,                             // 16
	"access":                   syscall.SYS_ACCESS,                            // 21
	"pipe":                     syscall.SYS_PIPE,                              // 22
	"dup":                      syscall.SYS_DUP,                               // 32
	"dup2":                     syscall.SYS_DUP2,                              // 33
	"getpid":                   syscall.SYS_GETPID,                            // 39
	"socket":                   syscall.SYS_SOCKET,                            // 41
	"connect":                  syscall.SYS_CONNECT,                           // 42
	"clone":                    syscall.SYS_CLONE,                             // 56
	"execve":                   syscall.SYS_EXECVE,                            // 59
	"exit":                     syscall.SYS_EXIT,                              // 60
	"wait4":                    syscall.SYS_WAIT4,                             // 61
	"kill":                     syscall.SYS_KILL,                              // 62
	"uname":                    syscall.SYS_UNAME,                             // 63
	"fcntl":                    syscall.SYS_FCNTL,                             // 72
	"getdents":                 syscall.SYS_GETDENTS,                          // 78
	"getcwd":                   syscall.SYS_GETCWD,                            // 79
	"chdir":                    syscall.SYS_CHDIR,                             // 80
 	"umask":                    syscall.SYS_UMASK,                             // 95	
        "gettimeofday":             syscall.SYS_GETTIMEOFDAY,                      // 96	
        "getrlimit":                syscall.SYS_GETRLIMIT,                         // 97
        "getuid":                   syscall.SYS_GETUID,                            // 102
	"getgid":                   syscall.SYS_GETGID,                            // 104
	"geteuid":                  syscall.SYS_GETEUID,                           // 107
	"getegid":                  syscall.SYS_GETEGID,                           // 108
	"setpgid":                  syscall.SYS_SETPGID,                           // 109
	"getppid":                  syscall.SYS_GETPPID,                           // 110	
        "getpgrp":                  syscall.SYS_GETPGRP,                           // 111	
        "statfs":                   syscall.SYS_STATFS,                            // 137
	"sysfs":                    syscall.SYS_SYSFS,                             // 139	
        "arch_prctl":               syscall.SYS_ARCH_PRCTL,                        // 158
	"gettid":                   syscall.SYS_GETTID,                            // 186
	"futex":                    syscall.SYS_FUTEX,                             // 202
	"set_tid_address":          syscall.SYS_SET_TID_ADDRESS,                   // 218
	"clock_gettime":            syscall.SYS_CLOCK_GETTIME,                     // 228
	"exit_group":               syscall.SYS_EXIT_GROUP,                        // 231
	"openat":                   syscall.SYS_OPENAT,                            // 257
	"faccessat":                syscall.SYS_FACCESSAT,                         // 269
	"set_robust_list":          syscall.SYS_SET_ROBUST_LIST,                   // 273
}

var scmpActAllow = 0

func ScmpInit(action int) (*ScmpCtx, error) {
    ctx := ScmpCtx{ CallMap : make (map[string]Action) }
    return &ctx, nil
}

func ScmpAdd(ctx *ScmpCtx, call string, action int) error {
    _, exists := ctx.CallMap[call]
    if exists {
        return errors.New("syscall exist")
    }
    sysCall, sysExists := SyscallMap[call]
	if sysExists {	    
	    ctx.CallMap[call] = Action{sysCall, action, ""}
	}

    return errors.New("syscall not surport")
}

func ScmpDel(ctx *ScmpCtx, call string) error {
    _, exists := ctx.CallMap[call]
    if exists {
        delete(ctx.CallMap, call)
        return  nil
    }

    return errors.New("syscall not exist")
}

func ScmpLoad(ctx *ScmpCtx) error {
    for key, _ := range SyscallMapMin {
        ScmpAdd(ctx, key, scmpActAllow);
    }
	
    num := len(ctx.CallMap)
    filter := make([]C.struct_scmp_map, num)

    index := 0
    for _, value := range ctx.CallMap {
        filter[index].syscall = C.int(value.syscall)
        filter[index].action = C.int(value.action)
        index++
    }

    res := C.scmp_filter((**C.struct_scmp_map)(unsafe.Pointer(&filter)), C.int(num))
    if 0 != res {
        return errors.New("SeccompLoad error") 
    }
    return nil 
}

func finalizeSeccomp(config *initConfig) error {
    scmpCtx, _ := ScmpInit(scmpActAllow)
	
    for _, call := range config.Config.SysCalls {
        ScmpAdd(scmpCtx, call, scmpActAllow);
    }
	
    return ScmpLoad(scmpCtx)
}

