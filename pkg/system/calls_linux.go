package system

import (
	"os/exec"
	"syscall"
)

func Chroot(dir string) error {
	return syscall.Chroot(dir)
}

func Chdir(dir string) error {
	return syscall.Chdir(dir)
}

func Exec(cmd string, args []string, env []string) error {
	return syscall.Exec(cmd, args, env)
}

func Execv(cmd string, args []string, env []string) error {
	name, err := exec.LookPath(cmd)
	if err != nil {
		return err
	}
	return Exec(name, args, env)
}

func Fork() (int, error) {
	syscall.ForkLock.Lock()
	pid, _, err := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	syscall.ForkLock.Unlock()
	if err != 0 {
		return -1, err
	}
	return int(pid), nil
}

func Mount(source, target, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}

func Unmount(target string, flags int) error {
	return syscall.Unmount(target, flags)
}

func Pivotroot(newroot, putold string) error {
	return syscall.PivotRoot(newroot, putold)
}

func Unshare(flags int) error {
	return syscall.Unshare(flags)
}

func Clone(flags uintptr) (int, error) {
	syscall.ForkLock.Lock()
	pid, _, err := syscall.RawSyscall(syscall.SYS_CLONE, flags, 0, 0)
	syscall.ForkLock.Unlock()
	if err != 0 {
		return -1, err
	}
	return int(pid), nil
}

func UsetCloseOnExec(fd uintptr) error {
	if _, _, err := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFD, 0); err != 0 {
		return err
	}
	return nil
}

func Setgroups(gids []int) error {
	return syscall.Setgroups(gids)
}

func Setresgid(rgid, egid, sgid int) error {
	return syscall.Setresgid(rgid, egid, sgid)
}

func Setresuid(ruid, euid, suid int) error {
	return syscall.Setresuid(ruid, euid, suid)
}

func Setgid(gid int) error {
	return syscall.Setgid(gid)
}

func Setuid(uid int) error {
	return syscall.Setuid(uid)
}

func Sethostname(name string) error {
	return syscall.Sethostname([]byte(name))
}

func Setsid() (int, error) {
	return syscall.Setsid()
}

func Ioctl(fd uintptr, flag, data uintptr) error {
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, flag, data); err != 0 {
		return err
	}
	return nil
}

func Closefd(fd uintptr) error {
	return syscall.Close(int(fd))
}

func Dup2(fd1, fd2 uintptr) error {
	return syscall.Dup2(int(fd1), int(fd2))
}

func Mknod(path string, mode uint32, dev int) error {
	return syscall.Mknod(path, mode, dev)
}

func ParentDeathSignal(sig uintptr) error {
	if _, _, err := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_PDEATHSIG, sig, 0); err != 0 {
		return err
	}
	return nil
}

func Setctty() error {
	if _, _, err := syscall.RawSyscall(syscall.SYS_IOCTL, 0, uintptr(syscall.TIOCSCTTY), 0); err != 0 {
		return err
	}
	return nil
}

func Mkfifo(name string, mode uint32) error {
	return syscall.Mkfifo(name, mode)
}

func Umask(mask int) int {
	return syscall.Umask(mask)
}

func SetCloneFlags(cmd *exec.Cmd, flag uintptr) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Cloneflags = flag
}

func Gettid() int {
	return syscall.Gettid()
}
