//go:build !windows

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

// WithDefaultPathEnv sets the $PATH environment variable to the
// default PATH defined in this package.
func WithDefaultPathEnv(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	s.Process.Env = replaceOrAppendEnvValues(s.Process.Env, defaultUnixEnv)
	return nil
}
