//go:build !linux

package osl

import "time"

type Interface struct{}

const (
	AdvertiseAddrNMsgsMin = 0
	AdvertiseAddrNMsgsMax = 3

	AdvertiseAddrIntervalMin = 100 * time.Millisecond
	AdvertiseAddrIntervalMax = 2 * time.Second
)
