// +build linux

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

package seccomp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// WithProfile receives the name of a file stored on disk comprising a json
// formatted seccomp profile, as specified by the opencontainers/runtime-spec.
// The profile is read from the file, unmarshaled, and set to the spec.
func WithProfile(profile string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Linux.Seccomp = &specs.LinuxSeccomp{}
		f, err := ioutil.ReadFile(profile)
		if err != nil {
			return fmt.Errorf("Cannot load seccomp profile %q: %v", profile, err)
		}
		if err := json.Unmarshal(f, s.Linux.Seccomp); err != nil {
			return fmt.Errorf("Decoding seccomp profile failed %q: %v", profile, err)
		}
		return nil
	}
}

// WithDefaultProfile sets the default seccomp profile to the spec.
// Note: must follow the setting of process capabilities
func WithDefaultProfile() oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Linux.Seccomp = DefaultProfile(s)
		return nil
	}
}
