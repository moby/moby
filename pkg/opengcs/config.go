// +build windows

package opengcs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

// Mode is the operational mode, both requested, and actual after verification
type Mode uint

// Constants for the actual mode after validation
const (
	ModeActualError = iota
	ModeActualVhdx
	ModeActualKernelInitrd
)

// Constants for the requested mode
const (
	ModeRequestAuto = iota // VHDX will be priority over kernel+initrd
	ModeRequestVhdx
	ModeRequestKernelInitrd
)

// Constants for the

// Config is the structure used to configuring a utility VM to be used
// as a service VM. There are two ways of starting. Either supply a VHD,
// or a Kernel+Initrd. For the latter, both must be supplied, and both
// must be in the same directory.
//
// VHD is the priority.
//
// All paths are full host path-names.
type Config struct {
	Kernel        string // Kernel for Utility VM (embedded in a UEFI bootloader)
	Initrd        string // Initrd image for Utility VM
	Vhdx          string // VHD for booting the utility VM
	Name          string // Name of the utility VM
	Svm           bool   // Is a service VM
	RequestedMode Mode   // What mode is preferred when validating
	ActualMode    Mode   // What mode was obtained during validation
}

// DefaultConfig generates a default config from a set of options
// If baseDir is not supplied, defaults to $env:ProgramFiles\lcow
func DefaultConfig(options []string) (Config, error) {
	baseDir := filepath.Join(os.Getenv("ProgramFiles"), "lcow")

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return Config{}, fmt.Errorf("opengcs: cannot create default utility VM configuration as directory '%s' was not found", baseDir)
	}

	config := Config{
		Vhdx:   filepath.Join(baseDir, `uvm.vhdx`),
		Kernel: filepath.Join(baseDir, `bootx64.efi`),
		Initrd: filepath.Join(baseDir, `initrd.img`),
		Svm:    false,
	}

	for _, v := range options {
		opt := strings.SplitN(v, "=", 2)
		if len(opt) == 2 {
			switch strings.ToLower(opt[0]) {
			case "lcowuvmkernel":
				config.Kernel = opt[1]
			case "lcowuvminitrd":
				config.Initrd = opt[1]
			case "lcowuvmvhdx":
				config.Vhdx = opt[1]
			}
		}
	}

	return config, nil
}

// validate validates a Config structure for starting a utility VM.
func (config *Config) validate() error {
	config.ActualMode = ModeActualError

	if config.RequestedMode == ModeRequestVhdx && config.Vhdx == "" {
		return fmt.Errorf("opengcs: config is invalid - request for VHDX mode did not supply a VHDX")
	}
	if config.RequestedMode == ModeRequestKernelInitrd && (config.Kernel == "" || config.Initrd == "") {
		return fmt.Errorf("opengcs: config is invalid - request for Kernel+Initrd mode must supply both kernel and initrd")
	}

	// Validate that if VHDX requested or auto, it exists.
	if config.RequestedMode == ModeRequestAuto || config.RequestedMode == ModeRequestVhdx {
		if _, err := os.Stat(config.Vhdx); os.IsNotExist(err) {
			if config.RequestedMode == ModeRequestVhdx {
				return fmt.Errorf("opengcs: mode requested was VHDX but '%s' could not be found", config.Vhdx)
			}
		} else {
			config.ActualMode = ModeActualVhdx
			return nil
		}
	}

	// So must be kernel+initrd, or auto where we fallback as the VHDX doesn't exist
	if config.Initrd == "" || config.Kernel == "" {
		if config.RequestedMode == ModeRequestKernelInitrd {
			return fmt.Errorf("opengcs: both initrd and kernel options for utility VM boot must be supplied")
		}
		return fmt.Errorf("opengcs: configuration is invalid")
	}
	if _, err := os.Stat(config.Kernel); os.IsNotExist(err) {
		return fmt.Errorf("opengcs: kernel '%s' was not found", config.Kernel)
	}
	if _, err := os.Stat(config.Initrd); os.IsNotExist(err) {
		return fmt.Errorf("opengcs: initrd '%s' was not found", config.Initrd)
	}
	dk, _ := filepath.Split(config.Kernel)
	di, _ := filepath.Split(config.Initrd)
	if dk != di {
		return fmt.Errorf("initrd '%s' and kernel '%s' must be located in the same directory", config.Initrd, config.Kernel)
	}

	config.ActualMode = ModeActualKernelInitrd
	return nil
}

// Create creates a utility VM from a configuration.
func (config *Config) Create() (hcsshim.Container, error) {
	logrus.Debugf("opengcs Create: %+v", config)

	if err := config.validate(); err != nil {
		return nil, err
	}

	configuration := &hcsshim.ContainerConfig{
		HvPartition:                 true,
		Name:                        config.Name,
		SystemType:                  "container",
		ContainerType:               "linux",
		Servicing:                   config.Svm, // TODO @jhowardmsft Need to stop overloading this field but needs platform change that is in-flight
		TerminateOnLastHandleClosed: true,
	}

	if config.ActualMode == ModeActualVhdx {
		configuration.HvRuntime = &hcsshim.HvRuntime{
			ImagePath: config.Vhdx,
		}
	} else {
		// TODO @jhowardmsft - with a platform change that is in-flight, remove ImagePath for
		// initrd/kernel boot. Current platform requires it.
		dir, _ := filepath.Split(config.Initrd)
		configuration.HvRuntime = &hcsshim.HvRuntime{
			ImagePath:       dir,
			LinuxInitrdPath: config.Initrd,
			LinuxKernelPath: config.Kernel,
		}
	}

	configurationS, _ := json.Marshal(configuration)
	logrus.Debugf("opengcs Create: Calling HCS with '%s'", string(configurationS))
	uvm, err := hcsshim.CreateContainer(config.Name, configuration)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("opengcs Create: uvm created, starting...")
	err = uvm.Start()
	if err != nil {
		logrus.Debugf("opengcs Create: uvm failed to start: %s", err)
		// Make sure we don't leave it laying around as it's been created in HCS
		uvm.Terminate()
		return nil, err
	}

	logrus.Debugf("opengcs Create: uvm %s is running", config.Name)
	return uvm, nil
}
