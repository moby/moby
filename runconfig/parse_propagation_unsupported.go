// +build !linux

package runconfig

import (
	"fmt"
)

// Ensure a valid string for mount propagation has been passed in.
func validateRootPropagation(rootPropagation string) error {
	if rootPropagation == "" {
		return nil
	}

	return fmt.Errorf("Specifying root mount propagation is not supported")
}
