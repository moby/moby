// +build windows

package syslog

import "fmt"

func New(priority Priority, tag string) (*Writer, error) {
	return Dial("", "", priority, tag)
}

func Dial(network, raddr string, priority Priority, tag string) (*Writer, error) {
	return nil, fmt.Errorf("Syslog is not supported on this platform")
}
