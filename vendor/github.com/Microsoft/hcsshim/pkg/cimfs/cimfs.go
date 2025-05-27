//go:build windows
// +build windows

package cimfs

import (
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/sirupsen/logrus"
)

func IsCimFSSupported() bool {
	rv, err := osversion.BuildRevision()
	if err != nil {
		logrus.WithError(err).Warn("get build revision")
	}
	return osversion.Build() == 20348 && rv >= 2031
}
