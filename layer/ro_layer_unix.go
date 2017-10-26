// +build !windows

package layer

func (rl *roLayer) OS() OS {
	return ""
}
