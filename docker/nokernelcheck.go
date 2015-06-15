// +build nokernelcheck !linux

package main

// Do not check the kernel version, always return true
func checkKernelVersion() (bool, string) {
	return true, ""
}
