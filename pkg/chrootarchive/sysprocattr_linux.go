package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"
import (
	"syscall"

	"golang.org/x/sys/unix"
)

func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: unix.SIGTERM,
	}
}
