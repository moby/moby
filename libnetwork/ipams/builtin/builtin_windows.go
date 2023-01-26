//go:build windows
// +build windows

package builtin

import (
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipams/windowsipam"
)

// Init registers the built-in ipam service with libnetwork
func Init(ic ipamapi.Callback, l, g interface{}) error {
	initFunc := windowsipam.GetInit(windowsipam.DefaultIPAM)

	err := initBuiltin(ic, l, g)
	if err != nil {
		return err
	}

	return initFunc(ic, l, g)
}
