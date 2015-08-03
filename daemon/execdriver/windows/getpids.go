// +build windows

package windows

import "fmt"

// GetPidsForContainer implements the exec driver Driver interface.
func (d *Driver) GetPidsForContainer(id string) ([]int, error) {
	// TODO Windows: Implementation required.
	return nil, fmt.Errorf("GetPidsForContainer: GetPidsForContainer() not implemented")
}
