/*
Copyright 2019-2021 Intel Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package blockio implements class-based cgroup blockio controller
// management for containers.
//
// Input: configuration of classes with blockio controller parameters
// (weights, throttling) for sets of block devices.
//
// Outputs:
// Option 1: Write blockio parameters of a class to a cgroup directory.
// Option 2: Return blockio parameters of a class in a OCI LinuxBlockIO
//
//	structure, that can be passed to OCI-compliant container
//	runtime.
//
// Notes:
//   - Using Weight requires bfq or cfq I/O scheduler to be
//     effective for the block devices where Weight is used.
//
// Configuration example:
//
//	Classes:
//
//	  # Define a blockio class "LowPrioThrottled".
//	  # Containers in this class will be throttled and handled as
//	  # low priority in the I/O scheduler.
//
//	  LowPrioThrottled:
//
//	    # Weight without a Devices list specifies the default
//	    # I/O scheduler weight for all devices
//	    # that are not explicitly mentioned in following items.
//	    # This will be written to cgroups(.bfq).weight.
//	    # Weights range from 10 to 1000, the default is 100.
//
//	    - Weight: 80
//
//	    # Set all parameters for all /dev/sd* and /dev/vd* block
//	    # devices.
//
//	    - Devices:
//	        - /dev/sd[a-z]
//	        - /dev/vd[a-z]
//	      ThrottleReadBps: 50M   # max read bytes per second
//	      ThrottleWriteBps: 10M  # max write bytes per second
//	      ThrottleReadIOPS: 10k  # max read io operations per second
//	      ThrottleWriteIOPS: 5k  # max write io operations per second
//	      Weight: 50             # I/O scheduler (cfq/bfq) weight for
//	                             # these devices will be written to
//	                             # cgroups(.bfq).weight_device
//
//	    # Set parameters particularly for SSD devices.
//	    # This configuration overrides above configurations for those
//	    # /dev/sd* and /dev/vd* devices whose disk id contains "SSD".
//
//	    - Devices:
//	        - /dev/disk/by-id/*SSD*
//	      ThrottleReadBps: 100M
//	      ThrottleWriteBps: 40M
//	      # Not mentioning Throttle*IOPS means no I/O operations
//	      # throttling on matching devices.
//	      Weight: 50
//
//	  # Define a blockio class "HighPrioFullSpeed".
//	  # There is no throttling on these containers, and
//	  # they will be prioritized by the I/O scheduler.
//
//	  HighPrioFullSpeed:
//	    - Weight: 400
//
// Usage example:
//
//	blockio.SetLogger(slog.Default().WithGroup("blockio"))
//	if err := blockio.SetConfigFromFile("/etc/containers/blockio.yaml", false); err != nil {
//	    return err
//	}
//	// Output option 1: write directly to cgroup "/mytestgroup"
//	if err := blockio.SetCgroupClass("/mytestgroup", "LowPrioThrottled"); err != nil {
//	    return err
//	}
//	// Output option 2: OCI LinuxBlockIO of a blockio class
//	if lbio, err := blockio.OciLinuxBlockIO("LowPrioThrottled"); err != nil {
//	    return err
//	} else {
//	    fmt.Printf("OCI LinuxBlockIO for LowPrioThrottled:\n%+v\n", lbio)
//	}
package blockio

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"

	goresctrlpath "github.com/intel/goresctrl/pkg/path"
)

const (
	// sysfsBlockDeviceIOSchedulerPaths expands (with glob) to block device scheduler files.
	// If modified, check how to parse device node from expanded paths.
	sysfsBlockDeviceIOSchedulerPaths = "/sys/block/*/queue/scheduler"
)

// tBlockDeviceInfo holds information on a block device to be configured.
// As users can specify block devices using wildcards ("/dev/disk/by-id/*SSD*")
// tBlockDeviceInfo.Origin is maintained for traceability: why this
// block device is included in configuration.
// tBlockDeviceInfo.DevNode contains resolved device node, like "/dev/sda".
type tBlockDeviceInfo struct {
	Major   int64
	Minor   int64
	DevNode string
	Origin  string
}

// Our logger instance.
var log *slog.Logger = slog.Default()

// classBlockIO connects user-defined block I/O classes to
// corresponding cgroups blockio controller parameters.
var classBlockIO = map[string]BlockIOParameters{}

// SetLogger sets the logger instance to be used by the package.
// Examples:
//
//	// Write structured records in text form to stderr, set verbosity to debug
//	logger := slog.New(slog.NewTextHandler(os.Stderr, slog.HandlerOptions{Level: slog.LevelDebug}))
//	blockio.SetLogger(logger)
func SetLogger(l *slog.Logger) {
	log = l
}

// SetConfigFromFile reads and applies blockio configuration from the
// filesystem.
func SetConfigFromFile(filename string, force bool) error {
	if data, err := os.ReadFile(filename); err == nil {
		if err = SetConfigFromData(data, force); err != nil {
			return fmt.Errorf("failed to set configuration from file %q: %s", filename, err)
		}
		return nil
	} else {
		return fmt.Errorf("failed to read config file %q: %v", filename, err)
	}
}

// SetConfigFromData parses and applies configuration from data.
func SetConfigFromData(data []byte, force bool) error {
	config := &Config{}
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return err
	}
	return SetConfig(config, force)
}

// SetConfig scans available block devices and applies new configuration.
func SetConfig(opt *Config, force bool) error {
	if opt == nil {
		// Setting nil configuration clears current configuration.
		// SetConfigFromData([]byte(""), dontcare) arrives here.
		classBlockIO = map[string]BlockIOParameters{}
		return nil
	}

	currentIOSchedulers, ioSchedulerDetectionError := getCurrentIOSchedulers()
	if ioSchedulerDetectionError != nil {
		log.Warn("configuration validation partly disabled due to I/O scheduler detection error", "error", ioSchedulerDetectionError)
	}

	classBlockIO = map[string]BlockIOParameters{}
	// Create cgroup blockio parameters for each blockio class
	for class := range opt.Classes {
		cgBlockIO, err := devicesParametersToCgBlockIO(opt.Classes[class], currentIOSchedulers)
		if err != nil {
			if force {
				log.Warn("ignoring blockio class", "className", class, "error", err)
			} else {
				return err
			}
		}
		classBlockIO[class] = cgBlockIO
	}
	return nil
}

// GetClasses returns block I/O class names
func GetClasses() []string {
	classNames := make([]string, 0, len(classBlockIO))
	for name := range classBlockIO {
		classNames = append(classNames, name)
	}
	sort.Strings(classNames)
	return classNames
}

// getCurrentIOSchedulers returns currently active I/O scheduler used for each block device in the system.
// Returns schedulers in a map: {"/dev/sda": "bfq"}
func getCurrentIOSchedulers() (map[string]string, error) {
	var ios = map[string]string{}
	glob := goresctrlpath.Path(sysfsBlockDeviceIOSchedulerPaths)
	schedulerFiles, err := filepath.Glob(glob)
	if err != nil {
		return ios, fmt.Errorf("error in I/O scheduler wildcards %#v: %w", glob, err)
	}
	for _, schedulerFile := range schedulerFiles {
		devName := strings.SplitN(schedulerFile, "/", 5)[3]
		schedulerDataB, err := os.ReadFile(schedulerFile)
		if err != nil {
			// A block device may be disconnected.
			log.Error("failed to read current I/O scheduler", "path", schedulerFile, "error", err)
			continue
		}
		schedulerData := strings.Trim(string(schedulerDataB), "\n")
		currentScheduler := ""
		if strings.IndexByte(schedulerData, ' ') == -1 {
			currentScheduler = schedulerData
		} else {
			openB := strings.Index(schedulerData, "[")
			closeB := strings.Index(schedulerData, "]")
			if -1 < openB && openB < closeB {
				currentScheduler = schedulerData[openB+1 : closeB]
			}
		}
		if currentScheduler == "" {
			log.Error("could not parse current scheduler", "path", schedulerFile)
			continue
		}

		ios["/dev/"+devName] = currentScheduler
	}
	return ios, nil
}

// deviceParametersToCgBlockIO converts single blockio class parameters into cgroups blkio format.
func devicesParametersToCgBlockIO(dps []DevicesParameters, currentIOSchedulers map[string]string) (BlockIOParameters, error) {
	errs := []error{}
	blkio := NewBlockIOParameters()
	for _, dp := range dps {
		var err error
		var weight, throttleReadBps, throttleWriteBps, throttleReadIOPS, throttleWriteIOPS int64
		weight, err = parseAndValidateQuantity("Weight", dp.Weight, -1, 10, 1000)
		errs = append(errs, err)
		throttleReadBps, err = parseAndValidateQuantity("ThrottleReadBps", dp.ThrottleReadBps, -1, 0, -1)
		errs = append(errs, err)
		throttleWriteBps, err = parseAndValidateQuantity("ThrottleWriteBps", dp.ThrottleWriteBps, -1, 0, -1)
		errs = append(errs, err)
		throttleReadIOPS, err = parseAndValidateQuantity("ThrottleReadIOPS", dp.ThrottleReadIOPS, -1, 0, -1)
		errs = append(errs, err)
		throttleWriteIOPS, err = parseAndValidateQuantity("ThrottleWriteIOPS", dp.ThrottleWriteIOPS, -1, 0, -1)
		errs = append(errs, err)
		if dp.Devices == nil {
			if weight > -1 {
				blkio.Weight = weight
			}
			if throttleReadBps > -1 || throttleWriteBps > -1 || throttleReadIOPS > -1 || throttleWriteIOPS > -1 {
				errs = append(errs, fmt.Errorf("ignoring throttling (rbps=%#v wbps=%#v riops=%#v wiops=%#v): Devices not listed",
					dp.ThrottleReadBps, dp.ThrottleWriteBps, dp.ThrottleReadIOPS, dp.ThrottleWriteIOPS))
			}
		} else {
			blockDevices, err := currentPlatform.configurableBlockDevices(dp.Devices)
			if err != nil {
				// Problems in matching block device wildcards and resolving symlinks
				// are worth reporting, but must not block configuring blkio where possible.
				log.Warn("failed to resolve block devices", "error", err)
			}
			if len(blockDevices) == 0 {
				log.Warn("no match, parameters ignored", "devices", dp.Devices)
			}
			for _, blockDeviceInfo := range blockDevices {
				if weight != -1 {
					if ios, found := currentIOSchedulers[blockDeviceInfo.DevNode]; found {
						if ios != "bfq" && ios != "cfq" {
							log.Warn("weight has no effect due to incompatible I/O scheduler (bfq or cfq required)",
								"path", blockDeviceInfo.DevNode, "scheduler", ios)
						}
					}
					blkio.WeightDevice.Update(blockDeviceInfo.Major, blockDeviceInfo.Minor, weight)
				}
				if throttleReadBps != -1 {
					blkio.ThrottleReadBpsDevice.Update(blockDeviceInfo.Major, blockDeviceInfo.Minor, throttleReadBps)
				}
				if throttleWriteBps != -1 {
					blkio.ThrottleWriteBpsDevice.Update(blockDeviceInfo.Major, blockDeviceInfo.Minor, throttleWriteBps)
				}
				if throttleReadIOPS != -1 {
					blkio.ThrottleReadIOPSDevice.Update(blockDeviceInfo.Major, blockDeviceInfo.Minor, throttleReadIOPS)
				}
				if throttleWriteIOPS != -1 {
					blkio.ThrottleWriteIOPSDevice.Update(blockDeviceInfo.Major, blockDeviceInfo.Minor, throttleWriteIOPS)
				}
			}
		}
	}
	return blkio, errors.Join(errs...)
}

// parseAndValidateQuantity parses quantities, like "64 M", and validates that they are in given range.
func parseAndValidateQuantity(fieldName string, fieldContent string,
	defaultValue int64, min int64, max int64) (int64, error) {
	// Returns field content
	if fieldContent == "" {
		return defaultValue, nil
	}
	qty, err := resource.ParseQuantity(fieldContent)
	if err != nil {
		return defaultValue, fmt.Errorf("syntax error in %#v (%#v)", fieldName, fieldContent)
	}
	value := qty.Value()
	if min != -1 && min > value {
		return defaultValue, fmt.Errorf("value of %#v (%#v) smaller than minimum (%#v)", fieldName, value, min)
	}
	if max != -1 && value > max {
		return defaultValue, fmt.Errorf("value of %#v (%#v) bigger than maximum (%#v)", fieldName, value, max)
	}
	return value, nil
}

// platformInterface includes functions that access the system. Enables mocking the system.
type platformInterface interface {
	configurableBlockDevices(devWildcards []string) ([]tBlockDeviceInfo, error)
}

// defaultPlatform versions of platformInterface functions access the underlying system.
type defaultPlatform struct{}

// currentPlatform defines which platformInterface is used: defaultPlatform or a mock, for instance.
var currentPlatform platformInterface = defaultPlatform{}

// configurableBlockDevices finds major:minor numbers for device filenames. Wildcards are allowed in filenames.
func (dpm defaultPlatform) configurableBlockDevices(devWildcards []string) ([]tBlockDeviceInfo, error) {
	// Return map {devNode: tBlockDeviceInfo}
	// Example: {"/dev/sda": {Major:8, Minor:0, Origin:"from symlink /dev/disk/by-id/ata-VendorXSSD from wildcard /dev/disk/by-id/*SSD*"}}
	errs := []error{}
	blockDevices := []tBlockDeviceInfo{}
	var origin string

	// 1. Expand wildcards to device filenames (may be symlinks)
	// Example: devMatches["/dev/disk/by-id/ata-VendorSSD"] == "from wildcard \"dev/disk/by-id/*SSD*\""
	devMatches := map[string]string{} // {devNodeOrSymlink: origin}
	for _, devWildcard := range devWildcards {
		devWildcardMatches, err := filepath.Glob(devWildcard)
		if err != nil {
			errs = append(errs, fmt.Errorf("bad device wildcard %#v: %w", devWildcard, err))
			continue
		}
		if len(devWildcardMatches) == 0 {
			errs = append(errs, fmt.Errorf("device wildcard %#v does not match any device nodes", devWildcard))
			continue
		}
		for _, devMatch := range devWildcardMatches {
			if devMatch != devWildcard {
				origin = fmt.Sprintf("from wildcard %#v", devWildcard)
			} else {
				origin = ""
			}
			devMatches[devMatch] = strings.TrimSpace(fmt.Sprintf("%v %v", devMatches[devMatch], origin))
		}
	}

	// 2. Find out real device nodes behind symlinks
	// Example: devRealPaths["/dev/sda"] == "from symlink \"/dev/disk/by-id/ata-VendorSSD\""
	devRealpaths := map[string]string{} // {devNode: origin}
	for devMatch, devOrigin := range devMatches {
		realDevNode, err := filepath.EvalSymlinks(devMatch)
		if err != nil {
			errs = append(errs, fmt.Errorf("cannot filepath.EvalSymlinks(%#v): %w", devMatch, err))
			continue
		}
		if realDevNode != devMatch {
			origin = fmt.Sprintf("from symlink %#v %v", devMatch, devOrigin)
		} else {
			origin = devOrigin
		}
		devRealpaths[realDevNode] = strings.TrimSpace(fmt.Sprintf("%v %v", devRealpaths[realDevNode], origin))
	}

	// 3. Filter out everything but block devices that are not partitions
	// Example: blockDevices[0] == {Major: 8, Minor: 0, DevNode: "/dev/sda", Origin: "..."}
	for devRealpath, devOrigin := range devRealpaths {
		origin := ""
		if devOrigin != "" {
			origin = fmt.Sprintf(" (origin: %s)", devOrigin)
		}
		fileInfo, err := os.Stat(devRealpath)
		if err != nil {
			errs = append(errs, fmt.Errorf("cannot os.Stat(%#v): %w%s", devRealpath, err, origin))
			continue
		}
		fileMode := fileInfo.Mode()
		if fileMode&os.ModeDevice == 0 {
			errs = append(errs, fmt.Errorf("file %#v is not a device%s", devRealpath, origin))
			continue
		}
		if fileMode&os.ModeCharDevice != 0 {
			errs = append(errs, fmt.Errorf("file %#v is a character device%s", devRealpath, origin))
			continue
		}
		sys, ok := fileInfo.Sys().(*syscall.Stat_t)
		major := unix.Major(uint64(sys.Rdev))
		minor := unix.Minor(uint64(sys.Rdev))
		if !ok {
			errs = append(errs, fmt.Errorf("cannot get syscall stat_t from %#v: %w%s", devRealpath, err, origin))
			continue
		}
		blockDevices = append(blockDevices, tBlockDeviceInfo{
			Major:   int64(major),
			Minor:   int64(minor),
			DevNode: devRealpath,
			Origin:  devOrigin,
		})
	}
	return blockDevices, errors.Join(errs...)
}
