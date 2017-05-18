// +build !linux,!darwin,!windows

package networkallocator

const initializers = nil

// PredefinedNetworks returns the list of predefined network structures
func PredefinedNetworks() []PredefinedNetworkData {
	return nil
}
