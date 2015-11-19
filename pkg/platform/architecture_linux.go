// Package platform provides helper function to get the runtime architecture
// for different platforms.
package platform

import (
	"syscall"
)

// runtimeArchitecture get the name of the current architecture (x86, x86_64, …)
func runtimeArchitecture() (string, error) {
	utsname := &syscall.Utsname{}
	if err := syscall.Uname(utsname); err != nil {
		return "", err
	}
	return charsToString(utsname.Machine), nil
}

func charsToString(ca [65]int8) string {
	s := make([]byte, len(ca))
	var lens int
	for ; lens < len(ca); lens++ {
		if ca[lens] == 0 {
			break
		}
		s[lens] = uint8(ca[lens])
	}
	return string(s[0:lens])
}
