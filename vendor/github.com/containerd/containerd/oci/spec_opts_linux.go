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
	"github.com/containerd/containerd/pkg/cap"
	specs "github.com/opencontainers/runtime-spec/specs-go"
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

// WithDevices recursively adds devices from the passed in path and associated cgroup rules for that device.
// If devicePath is a dir it traverses the dir to add all devices in that dir.
// If devicePath is not a dir, it attempts to add the single device.
// If containerPath is not set then the device path is used for the container path.
func WithDevices(devicePath, containerPath, permissions string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		devs, err := getDevices(devicePath, containerPath)
		if err != nil {
			return err
		}
		for i := range devs {
			s.Linux.Devices = append(s.Linux.Devices, devs[i])
			s.Linux.Resources.Devices = append(s.Linux.Resources.Devices, specs.LinuxDeviceCgroup{
				Allow:  true,
				Type:   devs[i].Type,
				Major:  &devs[i].Major,
				Minor:  &devs[i].Minor,
				Access: permissions,
			})
		}
		return nil
	}
}

// WithAllCurrentCapabilities propagates the effective capabilities of the caller process to the container process.
// The capability set may differ from WithAllKnownCapabilities when running in a container.
var WithAllCurrentCapabilities = func(ctx context.Context, client Client, c *containers.Container, s *Spec) error {
	caps, err := cap.Current()
	if err != nil {
		return err
	}
	return WithCapabilities(caps)(ctx, client, c, s)
}

// WithAllKnownCapabilities sets all the known linux capabilities for the container process
var WithAllKnownCapabilities = func(ctx context.Context, client Client, c *containers.Container, s *Spec) error {
	caps := cap.Known()
	return WithCapabilities(caps)(ctx, client, c, s)
}

func escapeAndCombineArgs(args []string) string {
	panic("not supported")
}
