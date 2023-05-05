//go:build !linux && !windows
// +build !linux,!windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package oci

import (
	"context"

	"github.com/containerd/containerd/containers"
)

// WithHostDevices adds all the hosts device nodes to the container's spec
func WithHostDevices(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	setLinux(s)

	devs, err := HostDevices()
	if err != nil {
		return err
	}
	s.Linux.Devices = append(s.Linux.Devices, devs...)
	return nil
}

// WithDevices recursively adds devices from the passed in path.
// If devicePath is a dir it traverses the dir to add all devices in that dir.
// If devicePath is not a dir, it attempts to add the single device.
func WithDevices(devicePath, containerPath, permissions string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		devs, err := getDevices(devicePath, containerPath)
		if err != nil {
			return err
		}
		s.Linux.Devices = append(s.Linux.Devices, devs...)
		return nil
	}
}

// WithCPUCFS sets the container's Completely fair scheduling (CFS) quota and period
func WithCPUCFS(quota int64, period uint64) SpecOpts {
	return func(ctx context.Context, _ Client, c *containers.Container, s *Spec) error {
		return nil
	}
}

func escapeAndCombineArgs(args []string) string {
	panic("not supported")
}
