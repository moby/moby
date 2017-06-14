// +build windows

package opengcs

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/rneugeba/virtsock/pkg/hvsock"
)

var (
	serviceVMId          hvsock.GUID
	serviceVMSocketID, _ = hvsock.GUIDFromString(socketID)
)

func init() {
	// TODO @jhowardmsft. Will require revisiting.  Get ID for hvsock. For now,
	// assume that it is always up. So, ignore the err for now.
	cmd := fmt.Sprintf("$(Get-ComputeProcess %s).Id", serviceVMName)
	result, _ := exec.Command("powershell", cmd).Output()
	serviceVMId, _ = hvsock.GUIDFromString(strings.TrimSpace(string(result)))
	logrus.Debugf("LCOW graphdriver: serviceVMID %s", serviceVMId)
}

//// exportLayer exports a sandbox layer
//func exportLayer(vhdPath string) (io.ReadCloser, error) {
//	logrus.Debugf("exportLayer vhdPath %s", vhdPath)
//	// Check if sandbox
//	if _, err := os.Stat(filepath.Join(vhdPath, layerSandboxName)); err == nil {
//		logrus.Debugf("exportLayer is a sandbox")
//		return exportSandbox(vhdPath)
//	}

//	// Otherwise, it's a normal vhd file.
//	logrus.Debugf("exportLayer is a normal VHD ")
//	vhdFile, err := os.Open(path.Join(vhdPath, layerVHDName))
//	if err != nil {
//		return nil, err
//	}
//	defer vhdFile.Close()

//	fileInfo, err := vhdFile.Stat()
//	if err != nil {
//		return nil, err
//	}

//	// Execute tar_to_vhd as a external process in the ServiceVM for
//	// converting a tar into a fixed VHD file
//	process, err := launchProcessInServiceVM("./svm_utils")
//	if err != nil {
//		logrus.Errorf("launchProcessInServiceVM failed with %s", err)
//		return nil, err
//	}

//	// get the std io pipes from the newly created process
//	stdin, stdout, _, err := process.Stdio()
//	if err != nil {
//		logrus.Errorf("[ServiceVMExportLayer]  getting std pipes failed %s", err)
//		return nil, err
//	}

//	header := &protocolCommandHeader{
//		Command:     cmdExport,
//		Version:     0,
//		PayloadSize: fileInfo.Size(),
//	}

//	err = sendData(header, vhdFile, stdin)
//	if err != nil {
//		logrus.Errorf("[ServiceVMExportLayer]  getting std pipes failed %s", err)
//		return nil, err
//	}

//	payloadSize, err := waitForResponse(stdout)
//	if err != nil {
//		return nil, err
//	}

//	reader, writer := io.Pipe()
//	go func() {
//		defer writer.Close()
//		//defer sendClose(stdout) @TODO @soccerGB. Can this be removed.
//		logrus.Debugf("Copying result over hvsock")
//		io.CopyN(writer, stdout, payloadSize)
//	}()
//	return reader, nil
//}

//func exportSandbox(sandboxFolder string) (io.ReadCloser, error) {
//	sandboxPath := path.Join(sandboxFolder, layerSandboxName)
//	logrus.Debugf("ServiceVMAttachSandbox: Creating sandbox path: %s", sandboxPath)

//	controllerNumber, controllerLocation, err := attachVHDX(sandboxPath)
//	if err != nil {
//		return nil, err
//	}
//	defer detachVHDX(controllerNumber, controllerLocation)
//	logrus.Debugf("ServiceVMExportSandbox: Got Controller number: %d controllerLocation: %d\n", controllerNumber, controllerLocation)

//	hdr := &protocolCommandHeader{
//		Command:     cmdExportSandbox,
//		Version:     version1,
//		PayloadSize: scsiCodeHeaderSize,
//	}

//	scsiHeader := &scsiCodeHeader{
//		controllerNumber:   controllerNumber,
//		controllerLocation: controllerLocation,
//	}

// @jhoward - no just call serialize on each in turn and append them in order: hdr, scsiHeader
//	data, err := serializeSCSI(hdr, scsiHeader)
//	if err != nil {
//		return nil, err
//	}

//	conn, err := connect()
//	if err != nil {
//		return nil, err
//	}

//	logrus.Debugf("Exporting sandbox to tar: %v", data)
//	_, err = conn.Write(data)
//	if err != nil {
//		return nil, err
//	}

//	payloadSize, err := waitForResponse(conn)
//	if err != nil {
//		return nil, err
//	}

//	reader, writer := io.Pipe()
//	go func() {
//		logrus.Debugf("Copying result over hvsock")
//		io.CopyN(writer, conn, payloadSize)
//		closeConnection(conn)
//		writer.Close()
//		logrus.Debugf("Done copying result over hvsock")
//	}()
//	return reader, nil
//}

func connect() (hvsock.Conn, error) {
	hvAddr := hvsock.HypervAddr{VMID: serviceVMId, ServiceID: serviceVMSocketID}
	return hvsock.Dial(hvAddr)
}

// closeConnection closes a connection to a utility VM
func closeConnection(rc io.WriteCloser) error {
	err := sendSerializedData(&protocolCommandHeader{
		Command:     cmdTerminate,
		Version:     version1,
		PayloadSize: 0,
	}, rc)

	if err != nil {
		rc.Close()
		return err
	}
	return rc.Close()

}
