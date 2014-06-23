// +build linux,!cgo

package system

func GetClockTicks() int {
	// when we cannot call out to C to get the sysconf it is fairly safe to
	// just return 100
	return 100
}
