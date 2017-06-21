// +build windows

package client

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

// Mode is the operational mode, both requested, and actual after verification
type Mode uint

const (
	// Constants for the actual mode after validation

	// ModeActualError means an error has occurred during validation
	ModeActualError = iota
	// ModeActualVhdx means that we are going to use VHDX boot after validation
	ModeActualVhdx
	// ModeActualKernelInitrd means that we are going to use kernel+initrd for boot after validation
	ModeActualKernelInitrd

	// Constants for the requested mode

	// ModeRequestAuto means auto-select the boot mode for a utility VM
	ModeRequestAuto = iota // VHDX will be priority over kernel+initrd
	// ModeRequestVhdx means request VHDX boot if possible
	ModeRequestVhdx
	// ModeRequestKernelInitrd means request Kernel+initrd boot if possible
	ModeRequestKernelInitrd

	// defaultUvmTimeoutSeconds is the default time to wait for utility VM operations
	defaultUvmTimeoutSeconds = 5 * 60

	// DefaultSandboxSizeMB is the size of the default sandbox size in MB
	DefaultSandboxSizeMB = 20 * 1024 * 1024
)

// Config is the structure used to configuring a utility VM to be used
// as a service VM. There are two ways of starting. Either supply a VHD,
// or a Kernel+Initrd. For the latter, both must be supplied, and both
// must be in the same directory.
//
// VHD is the priority.
//
// All paths are full host path-names.
type Config struct {
	Kernel            string            // Kernel for Utility VM (embedded in a UEFI bootloader)
	Initrd            string            // Initrd image for Utility VM
	Vhdx              string            // VHD for booting the utility VM
	Name              string            // Name of the utility VM
	RequestedMode     Mode              // What mode is preferred when validating
	ActualMode        Mode              // What mode was obtained during validation
	UvmTimeoutSeconds int               // How long to wait for the utility VM to respond in seconds
	Uvm               hcsshim.Container // The actual container
}

// GenerateDefault generates a default config from a set of options
// If baseDir is not supplied, defaults to $env:ProgramFiles\lcow
func (config *Config) GenerateDefault(options []string) error {
	baseDir := filepath.Join(os.Getenv("ProgramFiles"), "lcow")

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return fmt.Errorf("opengcs: cannot create default utility VM configuration as directory '%s' was not found", baseDir)
	}

	if config.UvmTimeoutSeconds < 0 {
		return fmt.Errorf("opengcs: cannot generate a config when supplied a negative utility VM timeout")
	}

	envTimeoutSeconds := 0
	optTimeoutSeconds := 0

	if config.UvmTimeoutSeconds != 0 {
		envTimeout := os.Getenv("OPENGCS_UVM_TIMEOUT_SECONDS")
		if len(envTimeout) > 0 {
			var err error
			if envTimeoutSeconds, err = strconv.Atoi(envTimeout); err != nil {
				return fmt.Errorf("opengcs: OPENGCS_UVM_TIMEOUT_SECONDS could not be interpreted as an integer")
			}
			if envTimeoutSeconds < 0 {
				return fmt.Errorf("opengcs: OPENGCS_UVM_TIMEOUT_SECONDS cannot be negative")
			}
		}
	}

	config.Vhdx = filepath.Join(baseDir, `uvm.vhdx`)
	config.Kernel = filepath.Join(baseDir, `bootx64.efi`)
	config.Initrd = filepath.Join(baseDir, `initrd.img`)

	for _, v := range options {
		opt := strings.SplitN(v, "=", 2)
		if len(opt) == 2 {
			switch strings.ToLower(opt[0]) {
			case "opengcskernel":
				config.Kernel = opt[1]
			case "opengcsinitrd":
				config.Initrd = opt[1]
			case "opengcsvhdx":
				config.Vhdx = opt[1]
			case "opengcstimeoutsecs":
				var err error
				if optTimeoutSeconds, err = strconv.Atoi(opt[1]); err != nil {
					return fmt.Errorf("opengcs: opengcstimeoutsecs option could not be interpreted as an integer")
				}
				if optTimeoutSeconds < 0 {
					return fmt.Errorf("opengcs: opengcstimeoutsecs option cannot be negative")
				}
			}
		}
	}

	// Which timeout are we going to take? If not through option or environment,
	// then use the default constant, otherwise the maximum of the option or
	// environment supplied setting. A requested on in the config supplied
	// overrides all of this.
	if config.UvmTimeoutSeconds == 0 {
		config.UvmTimeoutSeconds = defaultUvmTimeoutSeconds
		if optTimeoutSeconds != 0 || envTimeoutSeconds != 0 {
			config.UvmTimeoutSeconds = optTimeoutSeconds
			if envTimeoutSeconds > optTimeoutSeconds {
				config.UvmTimeoutSeconds = envTimeoutSeconds
			}
		}
	}

	return nil
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
func (config *Config) Create() error {
	logrus.Debugf("opengcs Create: %+v", config)

	if err := config.validate(); err != nil {
		return err
	}

	configuration := &hcsshim.ContainerConfig{
		HvPartition:                 true,
		Name:                        config.Name,
		SystemType:                  "container",
		ContainerType:               "linux",
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
	logrus.Debugf("opengcs Create: calling HCS with '%s'", string(configurationS))
	uvm, err := hcsshim.CreateContainer(config.Name, configuration)
	if err != nil {
		return err
	}
	logrus.Debugf("opengcs Create: uvm created, starting...")
	err = uvm.Start()
	if err != nil {
		logrus.Debugf("opengcs Create: uvm failed to start: %s", err)
		// Make sure we don't leave it laying around as it's been created in HCS
		uvm.Terminate()
		return err
	}

	config.Uvm = uvm
	logrus.Debugf("opengcs Create: uvm %s is running", config.Name)
	return nil
}
