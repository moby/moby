package system

import (
	"fmt"

	"github.com/Microsoft/hcsshim"
	"github.com/docker/docker/pkg/opengcs" // TODO @jhowardmsft This will move to a Microsoft repo imminently
)

// LCOWSupported returns true if Linux containers on Windows are supported.
func LCOWSupported() bool {
	return lcowSupported
}

// StartUVM starts a service utility VM
func StartUVM(options []string) (hcsshim.Container, error) {
	config, err := opengcs.DefaultConfig(options)
	if err != nil {
		return nil, fmt.Errorf("failed to start utility VM - could not generate opengcs configuration: %s", err)
	}
	config.Name = "LinuxServiceVM" // TODO @jhowardmsft - This requires an in-flight platform change. Can't hard code it to this longer term
	config.Svm = true              // TODO @jhowardmsft - This again in-flight platform change. We shouldn't need it.
	uvm, err := config.Create()
	if err != nil {
		return nil, fmt.Errorf("failed to start utility VM: %s", err)
	}
	return uvm, err
}
