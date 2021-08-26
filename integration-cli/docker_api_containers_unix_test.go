//go:build !windows
// +build !windows

package main

import "github.com/moby/sys/mount"

func mountWrapper(device, target, mType, options string) error {
	return mount.Mount(device, target, mType, options)
}
