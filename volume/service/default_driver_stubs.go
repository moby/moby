//go:build !linux && !windows
// +build !linux,!windows

package service // import "github.com/docker/docker/volume/service"

import (
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/volume/drivers"
)

func setupDefaultDriver(_ *drivers.Store, _ string, _ idtools.Identity) error { return nil }
