// +build windows

package client

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	winio "github.com/Microsoft/go-winio/vhd"
	//	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

// dismount is a simple utility function wrapping a conditional HotRemove. It would
// have been easier if you could cancel a deferred function, but this works just
// as well.
func (config *Config) dismount(file string) error {
	logrus.Debugf("opengcs: CreateExt4Vhdx: hot-remove of %s", file)
	err := config.HotRemoveVhd(file)
	if err != nil {
		logrus.Warnf("failed to hot-remove: %s", err)
	}
	return err
}

// CreateExt4Vhdx does what it says on the tin. It is the responsibility of the caller to synchronise
// simultaneous attempts to create the cache file.
func (config *Config) CreateExt4Vhdx(destFile string, sizeGB uint32, cacheFile string) error {
	// Smallest we can accept is the default sandbox size as we can't size down, only expand.
	if sizeGB < DefaultVhdxSizeGB {
		sizeGB = DefaultVhdxSizeGB
	}

	logrus.Debugf("opengcs: CreateExt4Vhdx: %s size:%dGB cache:%s", destFile, sizeGB, cacheFile)

	// Retrieve from cache if the default size and already on disk
	if cacheFile != "" && sizeGB == DefaultVhdxSizeGB {
		if _, err := os.Stat(cacheFile); err == nil {
			if err := CopyFile(cacheFile, destFile, false); err != nil {
				return fmt.Errorf("failed to copy cached file '%s' to '%s': %s", cacheFile, destFile, err)
			}
			logrus.Debugf("opengcs: CreateExt4Vhdx: %s fulfilled from cache", destFile)
			return nil
		}
	}

	// Must have a utility VM to operate on
	if config.Uvm == nil {
		return fmt.Errorf("no utility VM")
	}

	// Create the VHDX
	if err := winio.CreateVhdx(destFile, sizeGB, defaultVhdxBlockSizeMB); err != nil {
		return fmt.Errorf("failed to create VHDx %s: %s", destFile, err)
	}

	defer config.DebugGCS()

	// Attach it to the utility VM, but don't mount it (as there's no filesystem on it)
	if err := config.HotAddVhd(destFile, "", false, false); err != nil {
		return fmt.Errorf("opengcs: CreateExt4Vhdx: failed to hot-add %s to utility VM: %s", cacheFile, err)
	}

	// Get the list of mapped virtual disks to find the controller and LUN IDs
	logrus.Debugf("opengcs: CreateExt4Vhdx: %s querying mapped virtual disks", destFile)
	mvdControllers, err := config.Uvm.MappedVirtualDisks()
	if err != nil {
		return fmt.Errorf("failed to get mapped virtual disks: %s", err)
	}

	// Find our mapped disk from the list of all currently added.
	controller := -1
	lun := -1
	for controllerNumber, controllerElement := range mvdControllers {
		for diskNumber, diskElement := range controllerElement.MappedVirtualDisks {
			if diskElement.HostPath == destFile {
				controller = controllerNumber
				lun = diskNumber
				break
			}
		}
	}
	if controller == -1 || lun == -1 {
		config.dismount(destFile)
		return fmt.Errorf("failed to find %s in mapped virtual disks after hot-adding", destFile)
	}
	logrus.Debugf("opengcs: CreateExt4Vhdx: %s at C=%d L=%d", destFile, controller, lun)

	// Validate /sys/bus/scsi/devices/C:0:0:L exists as a directory
	testdCommand := fmt.Sprintf(`test -d /sys/bus/scsi/devices/%d:0:0:%d`, controller, lun)
	testdProc, err := config.RunProcess(testdCommand, nil, nil, nil)
	if err != nil {
		config.dismount(destFile)
		return fmt.Errorf("failed to `%s` following hot-add %s to utility VM: %s", testdCommand, destFile, err)
	}
	defer testdProc.Close()
	testdProc.WaitTimeout(time.Duration(int(time.Second) * config.UvmTimeoutSeconds))
	testdExitCode, err := testdProc.ExitCode()
	if err != nil {
		config.dismount(destFile)
		return fmt.Errorf("failed to get exit code from `%s` following hot-add %s to utility VM: %s", testdCommand, destFile, err)
	}
	if testdExitCode != 0 {
		config.dismount(destFile)
		return fmt.Errorf("`%s` return non-zero exit code (%d) following hot-add %s to utility VM", testdCommand, testdExitCode, destFile)
	}

	// Get the device from under the block subdirectory by doing a simple ls. This will come back as (eg) `sda`
	lsCommand := fmt.Sprintf(`ls /sys/bus/scsi/devices/%d:0:0:%d/block`, controller, lun)
	var lsOutput bytes.Buffer
	lsProc, err := config.RunProcess(lsCommand, nil, &lsOutput, nil)
	if err != nil {
		config.dismount(destFile)
		return fmt.Errorf("failed to `%s` following hot-add %s to utility VM: %s", lsCommand, destFile, err)
	}
	defer lsProc.Close()
	lsProc.WaitTimeout(time.Duration(int(time.Second) * config.UvmTimeoutSeconds))
	lsExitCode, err := lsProc.ExitCode()
	if err != nil {
		config.dismount(destFile)
		return fmt.Errorf("failed to get exit code from `%s` following hot-add %s to utility VM: %s", lsCommand, destFile, err)
	}
	if lsExitCode != 0 {
		config.dismount(destFile)
		return fmt.Errorf("`%s` return non-zero exit code (%d) following hot-add %s to utility VM", lsCommand, lsExitCode, destFile)
	}
	device := fmt.Sprintf(`/dev/%s`, strings.TrimSpace(lsOutput.String()))
	logrus.Debugf("opengcs: CreateExt4Vhdx: %s: device at %s", destFile, device)

	// Format it ext4
	mkfsCommand := fmt.Sprintf(`mkfs.ext4 -q -E lazy_itable_init=1 -O ^has_journal,sparse_super2,uninit_bg,^resize_inode %s`, device)
	var mkfsStderr bytes.Buffer
	mkfsProc, err := config.RunProcess(mkfsCommand, nil, nil, &mkfsStderr)
	if err != nil {
		config.dismount(destFile)
		return fmt.Errorf("failed to RunProcess %q following hot-add %s to utility VM: %s", destFile, mkfsCommand, err)
	}
	defer mkfsProc.Close()
	mkfsProc.WaitTimeout(time.Duration(int(time.Second) * config.UvmTimeoutSeconds))
	mkfsExitCode, err := mkfsProc.ExitCode()
	if err != nil {
		config.dismount(destFile)
		return fmt.Errorf("failed to get exit code from `%s` following hot-add %s to utility VM: %s", mkfsCommand, destFile, err)
	}
	if mkfsExitCode != 0 {
		config.dismount(destFile)
		return fmt.Errorf("`%s` return non-zero exit code (%d) following hot-add %s to utility VM: %s", mkfsCommand, mkfsExitCode, destFile, strings.TrimSpace(mkfsStderr.String()))
	}

	// Dismount before we copy it
	if err := config.dismount(destFile); err != nil {
		return fmt.Errorf("failed to hot-remove: %s", err)
	}

	// Populate the cache.
	if cacheFile != "" && (sizeGB == DefaultVhdxSizeGB) {
		if err := CopyFile(destFile, cacheFile, true); err != nil {
			return fmt.Errorf("failed to seed cache '%s' from '%s': %s", destFile, cacheFile, err)
		}
	}

	logrus.Debugf("opengcs: CreateExt4Vhdx: %s created (non-cache)", destFile)
	return nil
}
