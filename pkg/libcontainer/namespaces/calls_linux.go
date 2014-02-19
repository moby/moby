package namespaces

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	TIOCGPTN   = 0x80045430
	TIOCSPTLCK = 0x40045431
)

func chroot(dir string) error {
	return syscall.Chroot(dir)
}

func chdir(dir string) error {
	return syscall.Chdir(dir)
}

func exec(cmd string, args []string, env []string) error {
	return syscall.Exec(cmd, args, env)
}

func fork() (int, error) {
	syscall.ForkLock.Lock()
	pid, _, err := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	syscall.ForkLock.Unlock()
	if err != 0 {
		return -1, err
	}
	return int(pid), nil
}

func vfork() (int, error) {
	syscall.ForkLock.Lock()
	pid, _, err := syscall.Syscall(syscall.SYS_VFORK, 0, 0, 0)
	syscall.ForkLock.Unlock()
	if err != 0 {
		return -1, err
	}
	return int(pid), nil
}

func mount(source, target, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}

func unmount(target string, flags int) error {
	return syscall.Unmount(target, flags)
}

func pivotroot(newroot, putold string) error {
	return syscall.PivotRoot(newroot, putold)
}

func unshare(flags int) error {
	return syscall.Unshare(flags)
}

func clone(flags uintptr) (int, error) {
	syscall.ForkLock.Lock()
	pid, _, err := syscall.RawSyscall(syscall.SYS_CLONE, flags, 0, 0)
	syscall.ForkLock.Unlock()
	if err != 0 {
		return -1, err
	}
	return int(pid), nil
}

func setns(fd uintptr, flags uintptr) error {
	_, _, err := syscall.RawSyscall(SYS_SETNS, fd, flags, 0)
	if err != 0 {
		return err
	}
	return nil
}

func usetCloseOnExec(fd uintptr) error {
	if _, _, err := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFD, 0); err != 0 {
		return err
	}
	return nil
}

func setgroups(gids []int) error {
	return syscall.Setgroups(gids)
}

func setresgid(rgid, egid, sgid int) error {
	return syscall.Setresgid(rgid, egid, sgid)
}

func setresuid(ruid, euid, suid int) error {
	return syscall.Setresuid(ruid, euid, suid)
}

func sethostname(name string) error {
	return syscall.Sethostname([]byte(name))
}

func setsid() (int, error) {
	return syscall.Setsid()
}

func ioctl(fd uintptr, flag, data uintptr) error {
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, flag, data); err != 0 {
		return err
	}
	return nil
}

func openpmtx() (*os.File, error) {
	return os.OpenFile("/dev/ptmx", syscall.O_RDONLY|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
}

func unlockpt(f *os.File) error {
	var u int
	return ioctl(f.Fd(), TIOCSPTLCK, uintptr(unsafe.Pointer(&u)))
}

func ptsname(f *os.File) (string, error) {
	var n int
	if err := ioctl(f.Fd(), TIOCGPTN, uintptr(unsafe.Pointer(&n))); err != nil {
		return "", err
	}
	return fmt.Sprintf("/dev/pts/%d", n), nil
}

func closefd(fd uintptr) error {
	return syscall.Close(int(fd))
}

func dup2(fd1, fd2 uintptr) error {
	return syscall.Dup2(int(fd1), int(fd2))
}

func mknod(path string, mode uint32, dev int) error {
	return syscall.Mknod(path, mode, dev)
}

func parentDeathSignal() error {
	if _, _, err := syscall.RawSyscall6(syscall.SYS_PRCTL, syscall.PR_SET_PDEATHSIG, uintptr(syscall.SIGKILL), 0, 0, 0, 0); err != 0 {
		return err
	}
	return nil
}

func setctty() error {
	if _, _, err := syscall.RawSyscall(syscall.SYS_IOCTL, 0, uintptr(syscall.TIOCSCTTY), 0); err != 0 {
		return err
	}
	return nil
}

func mkfifo(name string, mode uint32) error {
	return syscall.Mkfifo(name, mode)
}

func umask(mask int) int {
	return syscall.Umask(mask)
}
