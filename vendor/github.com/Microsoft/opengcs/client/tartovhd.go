// +build windows

package client

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

// TarToVhd streams a tarstream contained in an io.Reader to a fixed vhd file
func (config *Config) TarToVhd(targetVHDFile string, reader io.Reader) (int64, error) {
	logrus.Debugf("opengcs: TarToVhd: %s", targetVHDFile)

	if config.Uvm == nil {
		return 0, fmt.Errorf("cannot Tar2Vhd as no utility VM is in configuration")
	}

	defer config.DebugGCS()

	process, err := config.createUtilsProcess("tar2vhd")
	if err != nil {
		return 0, fmt.Errorf("failed to start tar2vhd for %s: %s", targetVHDFile, err)
	}
	defer process.Process.Close()

	// Send the tarstream into the `tar2vhd`s stdin
	if _, err = copyWithTimeout(process.Stdin, reader, 0, config.UvmTimeoutSeconds, fmt.Sprintf("stdin of tar2vhd for generating %s", targetVHDFile)); err != nil {
		return 0, fmt.Errorf("failed sending to tar2vhd for %s: %s", targetVHDFile, err)
	}

	// Don't need stdin now we've sent everything. This signals GCS that we are finished sending data.
	if err := process.Process.CloseStdin(); err != nil {
		return 0, fmt.Errorf("failed closing stdin handle for %s: %s", targetVHDFile, err)
	}

	// Write stdout contents of `tar2vhd` to the VHD file
	payloadSize, err := writeFileFromReader(targetVHDFile, process.Stdout, config.UvmTimeoutSeconds, fmt.Sprintf("stdout of tar2vhd to %s", targetVHDFile))
	if err != nil {
		return 0, fmt.Errorf("failed to write %s during tar2vhd: %s", targetVHDFile, err)
	}

	logrus.Debugf("opengcs: TarToVhd: %s created, %d bytes", targetVHDFile, payloadSize)
	return payloadSize, err
}
