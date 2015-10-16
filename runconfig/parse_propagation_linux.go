// +build linux

package runconfig

import (
	"fmt"
)

// Ensure a valid string for mount propagation has been passed in.
func validateRootPropagation(rootPropagation string) error {
	if rootPropagation == "" {
		return nil
	}

	validFlags := []string{"private", "rprivate", "slave", "rslave", "shared", "rshared"}

	for _, flag := range validFlags {
		if rootPropagation == flag {
			return nil
		}
	}

	return fmt.Errorf("Invalid value '%s' used for root mount propagation. Supported values are [r]private, [r]slave and [r]shared", rootPropagation)
}
