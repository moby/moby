// +build !linux

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

// WithAllCurrentCapabilities propagates the effective capabilities of the caller process to the container process.
// The capability set may differ from WithAllKnownCapabilities when running in a container.
//nolint: deadcode, unused
var WithAllCurrentCapabilities = func(ctx context.Context, client Client, c *containers.Container, s *Spec) error {
	return WithCapabilities(nil)(ctx, client, c, s)
}

// WithAllKnownCapabilities sets all the the known linux capabilities for the container process
//nolint: deadcode, unused
var WithAllKnownCapabilities = func(ctx context.Context, client Client, c *containers.Container, s *Spec) error {
	return WithCapabilities(nil)(ctx, client, c, s)
}
