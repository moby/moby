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
	"strings"

	"github.com/containerd/containerd/containers"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// SpecOpts sets spec specific information to a newly generated OCI spec
type SpecOpts func(context.Context, Client, *containers.Container, *specs.Spec) error

// WithProcessArgs replaces the args on the generated spec
func WithProcessArgs(args ...string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		s.Process.Args = args
		return nil
	}
}

// WithProcessCwd replaces the current working directory on the generated spec
func WithProcessCwd(cwd string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		s.Process.Cwd = cwd
		return nil
	}
}

// WithHostname sets the container's hostname
func WithHostname(name string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		s.Hostname = name
		return nil
	}
}

// WithEnv appends environment variables
func WithEnv(environmentVariables []string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		if len(environmentVariables) > 0 {
			s.Process.Env = replaceOrAppendEnvValues(s.Process.Env, environmentVariables)
		}
		return nil
	}
}

// WithMounts appends mounts
func WithMounts(mounts []specs.Mount) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		s.Mounts = append(s.Mounts, mounts...)
		return nil
	}
}

// replaceOrAppendEnvValues returns the defaults with the overrides either
// replaced by env key or appended to the list
func replaceOrAppendEnvValues(defaults, overrides []string) []string {
	cache := make(map[string]int, len(defaults))
	for i, e := range defaults {
		parts := strings.SplitN(e, "=", 2)
		cache[parts[0]] = i
	}

	for _, value := range overrides {
		// Values w/o = means they want this env to be removed/unset.
		if !strings.Contains(value, "=") {
			if i, exists := cache[value]; exists {
				defaults[i] = "" // Used to indicate it should be removed
			}
			continue
		}

		// Just do a normal set/update
		parts := strings.SplitN(value, "=", 2)
		if i, exists := cache[parts[0]]; exists {
			defaults[i] = value
		} else {
			defaults = append(defaults, value)
		}
	}

	// Now remove all entries that we want to "unset"
	for i := 0; i < len(defaults); i++ {
		if defaults[i] == "" {
			defaults = append(defaults[:i], defaults[i+1:]...)
			i--
		}
	}

	return defaults
}
