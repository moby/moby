// Package operatingsystem provides helper function to get the operating system
// name for different platforms.
package operatingsystem

import (
	"bytes"
	"errors"
	"io/ioutil"
)

var (
	// file to use to detect if the daemon is running in a container
	proc1Cgroup = "/proc/1/cgroup"

	// file to check to determine Operating System
	etcOsRelease = "/etc/os-release"

	// used by stateless systems like Clear Linux
	altOsRelease = "/usr/lib/os-release"
)

// GetOperatingSystem gets the name of the current operating system.
func GetOperatingSystem() (string, error) {
	var err error
	for _, file := range []string{etcOsRelease, altOsRelease} {
		var b []byte
		b, err = ioutil.ReadFile(file)
		if err != nil {
			// try next file
			continue
		}
		if i := bytes.Index(b, []byte("PRETTY_NAME")); i >= 0 {
			b = b[i+13:]
			return string(b[:bytes.IndexByte(b, '"')]), nil
		}
		return "", errors.New("PRETTY_NAME not found")
	}
	return "", err
}

// IsContainerized returns true if we are running inside a container.
func IsContainerized() (bool, error) {
	b, err := ioutil.ReadFile(proc1Cgroup)
	if err != nil {
		return false, err
	}
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if len(line) > 0 && !bytes.HasSuffix(line, []byte{'/'}) {
			return true, nil
		}
	}
	return false, nil
}
