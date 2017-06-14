// +build windows

package opengcs

import (
	"fmt"
	"io"
	"os"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

// TarStreamToVHD converts a tar stream fom an io.Reader into a fixed vhd file
func TarStreamToVHD(uvm hcsshim.Container, targetVHDFile string, reader io.Reader) (int64, error) {
	logrus.Debugf("opengcs: TarStreamToVHD: %s", targetVHDFile)

	if uvm == nil {
		return 0, fmt.Errorf("opengcs: TarStreamToVHD: No utility VM was supplied")
	}

	// Write the readers contents to a temporary file
	tmpFile, fileSize, err := storeReader(reader)
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	process, err := createUtilsProcess(uvm)
	if err != nil {
		return 0, fmt.Errorf("opengcs: TarStreamToVHD: %s: failed to create utils process: %s", targetVHDFile, err)
	}

	defer func() {
		process.Process.Kill() // TODO @jhowardmsft - Add a timeout?
		process.Process.Close()
	}()

	header := &protocolCommandHeader{
		Command:     cmdImport,
		Version:     version1,
		PayloadSize: fileSize,
	}

	logrus.Debugf("opengcs: TarStreamToVHD: %s: Sending %d bytes to utility VM", targetVHDFile, fileSize)
	err = sendData(header, tmpFile, process.Stdin)
	if err != nil {
		return 0, fmt.Errorf("opengcs: TarStreamToVHD: %s: failed send to utility VM: %s", targetVHDFile, err)
	}

	logrus.Debugf("opengcs: TarStreamToVHD: %s: waiting for response", targetVHDFile)
	payloadSize, err := waitForResponse(process.Stdout)
	if err != nil {
		return 0, fmt.Errorf("opengcs: TarStreamToVHD: %s: failed waiting for response from utility VM: %s", targetVHDFile, err)
	}

	logrus.Debugf("opengcs: TarStreamToVHD: %s: response payload size %d", targetVHDFile, payloadSize)
	err = writeFileFromReader(targetVHDFile, payloadSize, process.Stdout)
	if err != nil {
		return 0, fmt.Errorf("opengcs: TarStreamToVHD: %s: failed writing VHD file: %s", targetVHDFile, err)
	}
	logrus.Debugf("opengcs: TarStreamToVHD: %s created", targetVHDFile)
	return payloadSize, err
}
