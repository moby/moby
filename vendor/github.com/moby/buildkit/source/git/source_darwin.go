package git

import "golang.org/x/sys/unix"

var reexecSysProcAttr = unix.SysProcAttr{
	Setpgid: true,
}
