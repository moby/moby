// +build !windows

package layer

import "runtime"

func (rl *roLayer) OS() string {
	return runtime.GOOS
}
