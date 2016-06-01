// +build cgo,linux cgo,freebsd

package system

/*
#include <unistd.h>
*/
import "C"

func GetClockTicks() int {
	return int(C.sysconf(C._SC_CLK_TCK))
}

func GetLongBit() int {
	return int(C.sysconf(C._SC_LONG_BIT))
}
