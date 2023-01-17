//go:build linux || freebsd || darwin
// +build linux freebsd darwin

package builtin

import "github.com/docker/docker/libnetwork/ipamapi"

// Init registers the built-in ipam service with libnetwork
func Init(ic ipamapi.Callback, l, g interface{}) error {
	return initBuiltin(ic, l, g)
}
