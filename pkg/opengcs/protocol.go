// +build windows

package opengcs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	cmdImport = iota
	cmdExport
	cmdCreateSandbox
	cmdExportSandbox
	cmdTerminate
	cmdResponseOK
	cmdResponseFail

	version1 = iota

	serviceVMHeaderSize   = 16
	scsiCodeHeaderSize    = 8
	sandboxInfoHeaderSize = 4
	serviceVMName         = "LinuxServiceVM"
	socketID              = "E9447876-BA98-444F-8C14-6A2FFF773E87"
)

var protocolTimeout = (5 * time.Minute)

type scsiCodeHeader struct {
	controllerNumber   uint32
	controllerLocation uint32
}
type sandboxInfoHeader struct {
	maxSandboxSizeInMB uint32
}

type protocolCommandHeader struct {
	Command     uint32
	Version     uint32
	PayloadSize int64
}

func waitForResponse(r io.Reader) (int64, error) {
	buf := make([]byte, serviceVMHeaderSize)
	var err error

	done := make(chan error, 1)
	go func() {
		_, err = io.ReadFull(r, buf)
		done <- err
	}()

	timeout := time.After(protocolTimeout)
	select {
	case <-timeout:
		return 0, fmt.Errorf("opengcs: waitForResponse: operation timed out")
	case err = <-done:
		if err != nil {
			return 0, fmt.Errorf("opengcs: waitForResponse: Failed to receive from utility VM: %s", err)
		}
	}

	hdr, err := deserializeHeader(buf)
	if err != nil {
		return 0, err
	}

	if hdr.Command != cmdResponseOK {
		logrus.Debugf("[waitForResponse] hdr.Command = 0x%0x", hdr.Command)
		return 0, fmt.Errorf("Service VM failed")
	}
	return hdr.PayloadSize, nil
}

// sendData sends a header and a payload to a service VM.
func sendData(hdr *protocolCommandHeader, payload io.Reader, dest io.Writer) error {
	// Send the header
	logrus.Debugf("opengcs: sendData: sending header command=%d version=%d size=%d", hdr.Command, hdr.Command, hdr.PayloadSize)
	if err := sendSerializedData(hdr, dest); err != nil {
		return err
	}

	// break into 4Kb chunks
	var (
		maxTransferSize       int64 = 4096
		bytesToTransfer       int64
		totalBytesTransferred int64
	)

	type result struct {
		bytesTransferred int64
		err              error
	}

	bytesLeft := hdr.PayloadSize
	for bytesLeft > 0 {
		if bytesLeft >= maxTransferSize {
			bytesToTransfer = maxTransferSize
		} else {
			bytesToTransfer = bytesLeft
		}
		logrus.Debugf("opengcs: sendData: sending chunk of %d bytes", bytesToTransfer)

		done := make(chan result, 1)
		go func() {
			r := result{}
			r.bytesTransferred, r.err = io.CopyN(dest, payload, bytesToTransfer)
			done <- r
		}()

		var r result
		timeout := time.After(protocolTimeout)
		select {
		case <-timeout:
			return fmt.Errorf("opengcs: sendData: operation timed out")
		case r = <-done:
			if r.err != nil && r.err != io.EOF {
				return fmt.Errorf("opengcs: sendData: after sending %d bytes with %d remaining, failed to send a chunk of %d bytes to the utility vm: %s", totalBytesTransferred, bytesLeft, bytesToTransfer, r.err)
			}
		}

		totalBytesTransferred += r.bytesTransferred
		bytesLeft -= r.bytesTransferred
		logrus.Debugf("opengcs: sendData bytes sent so far: %d remaining %d", totalBytesTransferred, bytesLeft)
	}
	logrus.Debugf("opengcs: sendData successful")
	return nil
}

// readHeader reads a header from a service VM.
func readHeader(r io.Reader) (*protocolCommandHeader, error) {
	hdr := &protocolCommandHeader{}
	buf, err := serialize(hdr)
	if err != nil {
		return nil, err
	}

	done := make(chan error, 1)
	go func() {
		_, err = io.ReadFull(r, buf)
		done <- err
	}()

	timeout := time.After(protocolTimeout)
	select {
	case <-timeout:
		return nil, fmt.Errorf("opengcs: readHeader: operation timed out")
	case err = <-done:
		if err != nil {
			return nil, fmt.Errorf("opengcs: readHeader: Failed to receive from utility VM: %s", err)
		}
	}
	return deserializeHeader(buf)
}

// deserializeHeader converts a byte array (from the service VM) into
// a go-structure for a protocol command header.
func deserializeHeader(hdr []byte) (*protocolCommandHeader, error) {
	buf := bytes.NewBuffer(hdr)
	hdrPtr := &protocolCommandHeader{}
	if err := binary.Read(buf, binary.BigEndian, hdrPtr); err != nil {
		return nil, err
	}
	return hdrPtr, nil
}

// sendSerializedData sends a go-structure to a service VM after serializing it.
func sendSerializedData(data interface{}, dest io.Writer) error {
	dataBytes, err := serialize(data)
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		_, err = dest.Write(dataBytes)
		done <- err
	}()

	timeout := time.After(protocolTimeout)
	select {
	case <-timeout:
		return fmt.Errorf("opengcs: sendSerializedData: operation timed out")
	case err = <-done:
		if err != nil {
			return fmt.Errorf("opengcs: sendSerializedData: Failed to send: %s", err)
		}
	}
	return nil
}

// serialize converts a go-structure into a byte array
func serialize(data interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.BigEndian, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
