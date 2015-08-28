package inheritcap

import (
	"github.com/Sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
	"os"
	"os/exec"
	"path/filepath"
)

func checkInheritable() {
	c, err := capability.NewPid(os.Getpid())
	if err != nil {
		logrus.Errorf("capability.NewPid: %v", err)
		return
	}

	if c.Empty(capability.INHERITABLE) {
		for i := capability.Cap(0); i <= capability.CAP_LAST_CAP; i++ {
			if c.Get(capability.EFFECTIVE, i) {
				c.Set(capability.INHERITABLE, i)
			}
		}
		c.Apply(capability.CAPS)
	}
}

func Command(name string, arg ...string) *exec.Cmd {
	if os.Geteuid() != 0 {
		checkInheritable()
	}

	cmd := &exec.Cmd{
		Path: name,
		Args: append([]string{name}, arg...),
	}
	if filepath.Base(name) == name {
		if lp, err := exec.LookPath(name); err != nil {
			logrus.Errorf("LookPath %s error: %v", name, err)
		} else {
			cmd.Path = lp
		}
	}
	return cmd
}
