// +build windows

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
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// WithWindowsCPUCount sets the `Windows.Resources.CPU.Count` section to the
// `count` specified.
func WithWindowsCPUCount(count uint64) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		if s.Windows.Resources == nil {
			s.Windows.Resources = &specs.WindowsResources{}
		}
		if s.Windows.Resources.CPU == nil {
			s.Windows.Resources.CPU = &specs.WindowsCPUResources{}
		}
		s.Windows.Resources.CPU.Count = &count
		return nil
	}
}

// WithWindowsIgnoreFlushesDuringBoot sets `Windows.IgnoreFlushesDuringBoot`.
func WithWindowsIgnoreFlushesDuringBoot() SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		if s.Windows == nil {
			s.Windows = &specs.Windows{}
		}
		s.Windows.IgnoreFlushesDuringBoot = true
		return nil
	}
}

// WithWindowNetworksAllowUnqualifiedDNSQuery sets `Windows.Network.AllowUnqualifiedDNSQuery`.
func WithWindowNetworksAllowUnqualifiedDNSQuery() SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		if s.Windows == nil {
			s.Windows = &specs.Windows{}
		}
		if s.Windows.Network == nil {
			s.Windows.Network = &specs.WindowsNetwork{}
		}

		s.Windows.Network.AllowUnqualifiedDNSQuery = true
		return nil
	}
}

// WithHostDevices adds all the hosts device nodes to the container's spec
//
// Not supported on windows
func WithHostDevices(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	return nil
}

func deviceFromPath(path string) (*specs.LinuxDevice, error) {
	return nil, errors.New("device from path not supported on Windows")
}
