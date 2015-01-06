package systemd

import (
	"encoding/hex"

	"github.com/coreos/go-systemd/machine1"
)

// RegisterMachine with systemd on the host system
func RegisterMachine(name string, id string, pid int, root_directory string) error {
	var av []byte
	if !SdBooted() {
		return nil
	}

	conn, err := machine1.New()
	if err != nil {
		return err
	}
	av, err = hex.DecodeString(id[0:32])
	if err != nil {
		return err
	}

	return conn.RegisterMachine(name, av, "docker", "container", pid, root_directory)
}
