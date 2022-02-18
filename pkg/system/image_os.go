package system // import "github.com/moby/moby/pkg/system"
import (
	"runtime"
	"strings"
)

// IsOSSupported determines if an operating system is supported by the host.
func IsOSSupported(os string) bool {
	return strings.EqualFold(runtime.GOOS, os)
}
