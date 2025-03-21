package unix

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/cilium/ebpf/internal"
)

// errNonLinux returns an error which wraps [internal.ErrNotSupportedOnOS] and
// includes the name of the calling function.
func errNonLinux() error {
	name := "unknown"
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		name = runtime.FuncForPC(pc).Name()
		if pos := strings.LastIndexByte(name, '.'); pos != -1 {
			name = name[pos+1:]
		}
	}
	return fmt.Errorf("unix: %s: %w", name, internal.ErrNotSupportedOnOS)
}
