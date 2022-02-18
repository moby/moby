//go:build !linux && !windows
// +build !linux,!windows

package service // import "github.com/moby/moby/volume/service"

import (
	"github.com/moby/moby/pkg/idtools"
	"github.com/moby/moby/volume/drivers"
)

func setupDefaultDriver(_ *drivers.Store, _ string, _ idtools.Identity) error { return nil }
