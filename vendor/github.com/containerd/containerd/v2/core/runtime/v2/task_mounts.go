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

package v2

import (
	"context"
	"fmt"

	bootapi "github.com/containerd/containerd/api/runtime/bootstrap/v1"
	apitypes "github.com/containerd/containerd/api/types"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"

	"github.com/containerd/containerd/v2/core/mount"
)

// taskMountController activates and deactivates a task's rootfs mounts,
// consulting the shim's advertised MountCapabilities so the mount manager
// does not perform mounts the shim already handles itself.
//
// A nil manager is valid and means no mount manager plugin is configured;
// Activate then returns rootfs unchanged.
type taskMountController struct {
	manager mount.Manager

	// legacy discovers mount capabilities from the deprecated
	// runtime-allow-mounts annotation, for a shim that does not attach a
	// MountCapabilities extension. A nil legacy disables the fallback.
	legacy *deprecatedMountCapabilities
}

// mountActivation is the result of activating a task's rootfs mounts.
type mountActivation struct {
	// rootfs is the set of mounts the shim must still perform itself.
	rootfs []mount.Mount

	// owned reports whether this call is responsible for deactivating the
	// activation if task creation subsequently fails. It is false when the
	// mount manager is absent, did not need to run, or when an existing
	// activation for taskID was reused instead of created here.
	owned bool
}

// Activate activates rootfs for taskID, translating the mount types and
// transforms named in bootstrap's MountCapabilities extension into claims the
// mount manager will not perform on the caller's behalf. bootstrap may be
// nil, meaning the shim advertised nothing over bootstrap.
//
// If bootstrap carries no such extension, runtimeName is checked against the
// deprecated runtime-allow-mounts annotation as a migration path; see
// [deprecatedMountCapabilities].
func (c *taskMountController) Activate(ctx context.Context, taskID string, runtimeName string, bootstrap *bootapi.BootstrapResult, rootfs []mount.Mount) (mountActivation, error) {
	if c.manager == nil {
		return mountActivation{rootfs: rootfs}, nil
	}

	activateOpts := []mount.ActivateOpt{
		mount.WithLabels(map[string]string{
			"containerd.io/gc.bref.container": taskID,
		}),
	}
	activateOpts = append(activateOpts, c.mountClaimOpts(ctx, runtimeName, bootstrap)...)

	ai, err := c.manager.Activate(ctx, taskID, rootfs, activateOpts...)
	switch {
	case err == nil:
		return mountActivation{rootfs: ai.System, owned: true}, nil
	case errdefs.IsAlreadyExists(err):
		// If creation of task with same identifier, use existing mount rather than forcing
		// deactivation of the old one. The back reference will prevent racing between
		// deactivation and re-use, as the container with the same ID would still exist.
		ai, err := c.manager.Info(ctx, taskID)
		if err != nil {
			return mountActivation{}, fmt.Errorf("failed to get info on already active mount: %w", err)
		}
		return mountActivation{rootfs: ai.System}, nil
	case errdefs.IsNotImplemented(err):
		// Nothing needed the mount manager, the shim performs all the mounts.
		return mountActivation{rootfs: rootfs}, nil
	default:
		return mountActivation{}, err
	}
}

// Deactivate deactivates the mounts activated for taskID.
func (c *taskMountController) Deactivate(ctx context.Context, taskID string) error {
	if c.manager == nil {
		return nil
	}
	return c.manager.Deactivate(ctx, taskID)
}

// mountClaimOpts translates the mount capabilities of runtimeName's shim into
// activation options describing the mount types and transforms it performs
// itself. bootstrap's MountCapabilities extension is authoritative when
// present; otherwise c.legacy is consulted as a migration path.
func (c *taskMountController) mountClaimOpts(ctx context.Context, runtimeName string, bootstrap *bootapi.BootstrapResult) []mount.ActivateOpt {
	caps, err := mountCapabilitiesExtension(bootstrap)
	if err != nil {
		// The shim's mount capabilities are unreadable. Claiming nothing means
		// the mount manager does the work, which is the safe direction to
		// fail in.
		log.G(ctx).WithError(err).Error("failed to read shim mount capabilities, assuming none")
		return nil
	}
	if caps == nil {
		caps = c.legacy.lookup(ctx, runtimeName)
	}
	if caps == nil {
		return nil
	}

	opts := make([]mount.ActivateOpt, 0, len(caps.Types)+len(caps.Transforms))
	for _, t := range caps.Types {
		opts = append(opts, mount.WithAllowMountType(t))
	}
	for _, t := range caps.Transforms {
		opts = append(opts, mount.WithAllowTransform(t))
	}
	return opts
}

// mountCapabilitiesExtension extracts the MountCapabilities extension from
// bootstrap, if any. bootstrap may be nil.
func mountCapabilitiesExtension(bootstrap *bootapi.BootstrapResult) (*apitypes.MountCapabilities, error) {
	var caps apitypes.MountCapabilities
	found, err := bootstrap.FindExtension(&caps)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return &caps, nil
}
