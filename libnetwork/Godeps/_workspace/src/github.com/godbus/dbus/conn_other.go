// +build !darwin

package dbus

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
)

func sessionBusPlatform() (*Conn, error) {
	cmd := exec.Command("dbus-launch")
	b, err := cmd.CombinedOutput()

	if err != nil {
		return nil, err
	}

	i := bytes.IndexByte(b, '=')
	j := bytes.IndexByte(b, '\n')

	if i == -1 || j == -1 {
		return nil, errors.New("dbus: couldn't determine address of session bus")
	}

	env, addr := string(b[0:i]), string(b[i+1:j])
	os.Setenv(env, addr)

	return Dial(addr)
}
