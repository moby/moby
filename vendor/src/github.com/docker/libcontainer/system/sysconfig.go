// +build linux,cgo

package system

/*
#include <unistd.h>
int get_hz(void) { return sysconf(_SC_CLK_TCK); }
*/
import "C"

func GetClockTicks() int {
	return int(C.get_hz())
}
