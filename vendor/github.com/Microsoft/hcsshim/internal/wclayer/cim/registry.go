//go:build windows

package cim

import (
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"

	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/winapi"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// enableCimBoot Opens the SYSTEM registry hive at path `hivePath` and updates it to include a CIMFS Start
// registry key. This prepares the uvm to boot from a cim file if requested. The registry changes required to
// actually make the uvm boot from a cim will be added in the uvm config (look at
// addBootFromCimRegistryChanges for details).  This registry key needs to be available in the early boot
// phase and so including it in the uvm config doesn't work.
func enableCimBoot(hivePath string) (err error) {
	dataZero := make([]byte, 4)
	dataOne := make([]byte, 4)
	binary.LittleEndian.PutUint32(dataOne, 1)
	dataFour := make([]byte, 4)
	binary.LittleEndian.PutUint32(dataFour, 4)

	bootGUID, err := windows.UTF16FromString(bootContainerID)
	if err != nil {
		return fmt.Errorf("failed to encode boot guid to utf16: %w", err)
	}

	overrideBootPath, err := windows.UTF16FromString("\\Windows\\")
	if err != nil {
		return fmt.Errorf("failed to encode override boot path to utf16: %w", err)
	}

	regChanges := []struct {
		keyPath   string
		valueName string
		valueType winapi.RegType
		data      *byte
		dataLen   uint32
	}{
		{"ControlSet001\\Control", "BootContainerGuid", winapi.REG_TYPE_SZ, (*byte)(unsafe.Pointer(&bootGUID[0])), 2 * uint32(len(bootGUID))},
		{"ControlSet001\\Services\\UnionFS", "Start", winapi.REG_TYPE_DWORD, &dataZero[0], uint32(len(dataZero))},
		{"ControlSet001\\Services\\wcifs", "Start", winapi.REG_TYPE_DWORD, &dataFour[0], uint32(len(dataZero))},
		// The bootmgr loads the uvm files from the cim and so uses the relative path `UtilityVM\\Files` inside the cim to access the uvm files. However, once the cim is mounted UnionFS will merge the correct directory (UtilityVM\\Files) of the cim with the scratch and then that point onwards we don't need to use the relative path. Below registry key tells the kernel that the boot path that was provided in BCD should now be overriden with this new path.
		{"Setup", "BootPathOverride", winapi.REG_TYPE_SZ, (*byte)(unsafe.Pointer(&overrideBootPath[0])), 2 * uint32(len(overrideBootPath))},
	}

	var storeHandle winapi.ORHKey
	if err = winapi.OROpenHive(hivePath, &storeHandle); err != nil {
		return fmt.Errorf("failed to open registry store at %s: %w", hivePath, err)
	}

	for _, change := range regChanges {
		var changeKey winapi.ORHKey
		if err = winapi.ORCreateKey(storeHandle, change.keyPath, 0, 0, 0, &changeKey, nil); err != nil {
			return fmt.Errorf("failed to open reg key %s: %w", change.keyPath, err)
		}

		if err = winapi.ORSetValue(changeKey, change.valueName, uint32(change.valueType), change.data, change.dataLen); err != nil {
			return fmt.Errorf("failed to set value for regkey %s\\%s : %w", change.keyPath, change.valueName, err)
		}
	}

	// remove the existing file first
	if err := os.Remove(hivePath); err != nil {
		return fmt.Errorf("failed to remove existing registry %s: %w", hivePath, err)
	}

	if err = winapi.ORSaveHive(winapi.ORHKey(storeHandle), hivePath, uint32(osversion.Get().MajorVersion), uint32(osversion.Get().MinorVersion)); err != nil {
		return fmt.Errorf("error saving the registry store: %w", err)
	}

	// close hive irrespective of the errors
	if err := winapi.ORCloseHive(winapi.ORHKey(storeHandle)); err != nil {
		return fmt.Errorf("error closing registry store; %w", err)
	}
	return nil

}

// mergeHive merges the hive located at parentHivePath with the hive located at deltaHivePath and stores
// the result into the file at mergedHivePath. If a file already exists at path `mergedHivePath` then it
// throws an error.
func mergeHive(parentHivePath, deltaHivePath, mergedHivePath string) (err error) {
	var baseHive, deltaHive, mergedHive winapi.ORHKey
	if err := winapi.OROpenHive(parentHivePath, &baseHive); err != nil {
		return fmt.Errorf("failed to open base hive %s: %w", parentHivePath, err)
	}
	defer func() {
		err2 := winapi.ORCloseHive(baseHive)
		if err == nil {
			err = errors.Wrap(err2, "failed to close base hive")
		}
	}()
	if err := winapi.OROpenHive(deltaHivePath, &deltaHive); err != nil {
		return fmt.Errorf("failed to open delta hive %s: %w", deltaHivePath, err)
	}
	defer func() {
		err2 := winapi.ORCloseHive(deltaHive)
		if err == nil {
			err = errors.Wrap(err2, "failed to close delta hive")
		}
	}()
	if err := winapi.ORMergeHives([]winapi.ORHKey{baseHive, deltaHive}, &mergedHive); err != nil {
		return fmt.Errorf("failed to merge hives: %w", err)
	}
	defer func() {
		err2 := winapi.ORCloseHive(mergedHive)
		if err == nil {
			err = errors.Wrap(err2, "failed to close merged hive")
		}
	}()
	if err := winapi.ORSaveHive(mergedHive, mergedHivePath, uint32(osversion.Get().MajorVersion), uint32(osversion.Get().MinorVersion)); err != nil {
		return fmt.Errorf("failed to save hive: %w", err)
	}
	return
}

// getOsBuildNumberFromRegistry fetches the "CurrentBuild" value at path
// "Microsoft\Windows NT\CurrentVersion" from the SOFTWARE registry hive at path
// `regHivePath`. This is used to detect the build version of the uvm.
func getOsBuildNumberFromRegistry(regHivePath string) (_ string, err error) {
	var storeHandle, keyHandle winapi.ORHKey
	var dataType, dataLen uint32
	keyPath := "Microsoft\\Windows NT\\CurrentVersion"
	valueName := "CurrentBuild"
	dataLen = 16 // build version string can't be more than 5 wide chars?
	dataBuf := make([]byte, dataLen)

	if err = winapi.OROpenHive(regHivePath, &storeHandle); err != nil {
		return "", fmt.Errorf("failed to open registry store at %s: %w", regHivePath, err)
	}
	defer func() {
		if closeErr := winapi.ORCloseHive(storeHandle); closeErr != nil {
			log.L.WithFields(logrus.Fields{
				"error": closeErr,
				"hive":  regHivePath,
			}).Warnf("failed to close hive")
		}
	}()

	if err = winapi.OROpenKey(storeHandle, keyPath, &keyHandle); err != nil {
		return "", fmt.Errorf("failed to open key at %s: %w", keyPath, err)
	}
	defer func() {
		if closeErr := winapi.ORCloseKey(keyHandle); closeErr != nil {
			log.L.WithFields(logrus.Fields{
				"error": closeErr,
				"hive":  regHivePath,
				"key":   keyPath,
				"value": valueName,
			}).Warnf("failed to close hive key")
		}
	}()

	if err = winapi.ORGetValue(keyHandle, "", valueName, &dataType, &dataBuf[0], &dataLen); err != nil {
		return "", fmt.Errorf("failed to get value of %s: %w", valueName, err)
	}

	if dataType != uint32(winapi.REG_TYPE_SZ) {
		return "", fmt.Errorf("unexpected build number data type (%d)", dataType)
	}

	return winapi.ParseUtf16LE(dataBuf[:(dataLen - 2)]), nil
}
