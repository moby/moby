//go:build !linux && !windows

package service

import (
	"github.com/moby/moby/daemon/internal/idtools"
	"github.com/moby/moby/daemon/volume/drivers"
)

func setupDefaultDriver(_ *drivers.Store, _ string, _ idtools.Identity) error { return nil }
