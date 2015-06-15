// +build linux,!nokernelcheck

package main

import (
	"github.com/docker/docker/pkg/parsers/kernel"
)

const (
	MajorVersion = 3
	MinorVersion = 10
)

func checkKernelVersion() (bool, *kernel.KernelVersionInfo) {
	kernelVersion, _ := kernel.GetKernelVersion()

	isValid := kernelVersion.Major >= MajorVersion && kernelVersion.Minor >= MinorVersion

	return isValid, kernelVersion
}
