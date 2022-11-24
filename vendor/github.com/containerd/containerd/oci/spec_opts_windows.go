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
	"errors"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows"

	"github.com/containerd/containerd/containers"
)

func escapeAndCombineArgs(args []string) string {
	escaped := make([]string, len(args))
	for i, a := range args {
		escaped[i] = windows.EscapeArg(a)
	}
	return strings.Join(escaped, " ")
}

// WithProcessCommandLine replaces the command line on the generated spec
func WithProcessCommandLine(cmdLine string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setProcess(s)
		s.Process.Args = nil
		s.Process.CommandLine = cmdLine
		return nil
	}
}

// WithHostDevices adds all the hosts device nodes to the container's spec
//
// Not supported on windows
func WithHostDevices(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	return nil
}

func DeviceFromPath(path string) (*specs.LinuxDevice, error) {
	return nil, errors.New("device from path not supported on Windows")
}

// WithDevices does nothing on Windows.
func WithDevices(devicePath, containerPath, permissions string) SpecOpts {
	return func(ctx context.Context, client Client, container *containers.Container, spec *Spec) error {
		return nil
	}
}
