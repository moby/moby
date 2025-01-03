//go:build windows

package cim

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/Microsoft/go-winio/pkg/guid"
)

const (
	bcdFilePath              = "UtilityVM\\Files\\EFI\\Microsoft\\Boot\\BCD"
	cimfsDeviceOptionsID     = "{763e9fea-502d-434f-aad9-5fabe9c91a7b}"
	vmbusDeviceID            = "{c63c9bdf-5fa5-4208-b03f-6b458b365592}"
	compositeDeviceOptionsID = "{e1787220-d17f-49e7-977a-d8fe4c8537e2}"
	bootContainerID          = "{b890454c-80de-4e98-a7ab-56b74b4fbd0c}"
)

func bcdExec(storePath string, args ...string) error {
	var out bytes.Buffer
	argsArr := []string{"/store", storePath, "/offline"}
	argsArr = append(argsArr, args...)
	cmd := exec.Command("bcdedit.exe", argsArr...)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bcd command (%s) failed: %w", cmd, err)
	}
	return nil
}

// A registry configuration required for the uvm.
func setBcdRestartOnFailure(storePath string) error {
	return bcdExec(storePath, "/set", "{default}", "restartonfailure", "yes")
}

func setBcdCimBootDevice(storePath, cimPathRelativeToVSMB string, diskID, partitionID guid.GUID) error {
	// create options for cimfs boot device
	if err := bcdExec(storePath, "/create", cimfsDeviceOptionsID, "/d", "CimFS Device Options", "/device"); err != nil {
		return err
	}

	// Set options. For now we need to set 2 options. First is the parent device i.e the device under
	// which all cim files will be available. Second is the path of the cim (from which this UVM should
	// boot) relative to the parent device. Note that even though the 2nd option is named
	// `cimfsrootdirectory` it expects a path to the cim file and not a directory path.
	if err := bcdExec(storePath, "/set", cimfsDeviceOptionsID, "cimfsparentdevice", fmt.Sprintf("vmbus=%s", vmbusDeviceID)); err != nil {
		return err
	}

	if err := bcdExec(storePath, "/set", cimfsDeviceOptionsID, "cimfsrootdirectory", fmt.Sprintf("\\%s", cimPathRelativeToVSMB)); err != nil {
		return err
	}

	// create options for the composite device
	if err := bcdExec(storePath, "/create", compositeDeviceOptionsID, "/d", "Composite Device Options", "/device"); err != nil {
		return err
	}

	// We need to specify the diskID & the partition ID of the boot disk and we need to set the cimfs boot
	// options ID
	partitionStr := fmt.Sprintf("gpt_partition={%s};{%s}", diskID, partitionID)
	if err := bcdExec(storePath, "/set", compositeDeviceOptionsID, "primarydevice", partitionStr); err != nil {
		return err
	}

	if err := bcdExec(storePath, "/set", compositeDeviceOptionsID, "secondarydevice", fmt.Sprintf("cimfs=%s,%s", bootContainerID, cimfsDeviceOptionsID)); err != nil {
		return err
	}

	if err := bcdExec(storePath, "/set", "{default}", "device", fmt.Sprintf("composite=0,%s", compositeDeviceOptionsID)); err != nil {
		return err
	}

	if err := bcdExec(storePath, "/set", "{default}", "osdevice", fmt.Sprintf("composite=0,%s", compositeDeviceOptionsID)); err != nil {
		return err
	}

	// Since our UVM file are stored under UtilityVM\Files directory inside the CIM we must prepend that
	// directory in front of paths used by bootmgr
	if err := bcdExec(storePath, "/set", "{default}", "path", "\\UtilityVM\\Files\\Windows\\System32\\winload.efi"); err != nil {
		return err
	}

	if err := bcdExec(storePath, "/set", "{default}", "systemroot", "\\UtilityVM\\Files\\Windows"); err != nil {
		return err
	}

	return nil
}

// updateBcdStoreForBoot Updates the bcd store at path layerPath + UtilityVM\Files\EFI\Microsoft\Boot\BCD` to
// boot with the disk with given ID and given partitionID.  cimPathRelativeToVSMB is the path of the cim which
// will be used for booting this UVM relative to the VSMB share. (Usually, the entire snapshots directory will
// be shared over VSMB, so if this is the cim-layers\1.cim under that directory, the value of
// `cimPathRelativeToVSMB` should be cim-layers\1.cim)
func updateBcdStoreForBoot(storePath string, cimPathRelativeToVSMB string, diskID, partitionID guid.GUID) error {
	if err := setBcdRestartOnFailure(storePath); err != nil {
		return err
	}

	if err := setBcdCimBootDevice(storePath, cimPathRelativeToVSMB, diskID, partitionID); err != nil {
		return err
	}
	return nil
}
