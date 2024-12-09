//go:build !linux

package osl

import (
	"time"

	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/types"
)

type Interface struct{}

func ValidateAdvertiseAddrNMsgs(count int) error {
	return types.InvalidParameterErrorf(netlabel.AdvertiseAddrNMsgs + " is not supported on Windows")
}

func ValidateAdvertiseAddrInterval(interval time.Duration) error {
	return types.InvalidParameterErrorf(netlabel.AdvertiseAddrIntervalMs + " is not supported on Windows")
}
