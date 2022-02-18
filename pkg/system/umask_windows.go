package system // import "github.com/moby/moby/pkg/system"

// Umask is not supported on the windows platform.
func Umask(newmask int) (oldmask int, err error) {
	// should not be called on cli code path
	return 0, ErrNotSupportedPlatform
}
