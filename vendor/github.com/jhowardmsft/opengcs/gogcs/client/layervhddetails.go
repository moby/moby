// +build windows

package client

import (
	"fmt"
	"os"
	"path/filepath"
)

// LayerVhdDetails is a utility for getting a file name, size and indication of
// sandbox for a VHD(x) in a folder. A read-only layer will be layer.vhd. A
// read-write layer will be sandbox.vhdx.
func LayerVhdDetails(folder string) (string, int64, bool, error) {
	var fileInfo os.FileInfo
	isSandbox := false
	filename := filepath.Join(folder, "layer.vhd")
	var err error

	if fileInfo, err = os.Stat(filename); err != nil {
		filename = filepath.Join(folder, "sandbox.vhdx")
		if fileInfo, err = os.Stat(filename); err != nil {
			if os.IsNotExist(err) {
				return "", 0, isSandbox, fmt.Errorf("could not find layer or sandbox in %s", folder)
			}
			return "", 0, isSandbox, fmt.Errorf("error locating layer or sandbox in %s: %s", folder, err)
		}
		isSandbox = true
	}
	return filename, fileInfo.Size(), isSandbox, nil
}
