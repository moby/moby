// +build !linux,!darwin,!windows

package networkallocator

func getInitializers() []initializer {
	return nil
}
