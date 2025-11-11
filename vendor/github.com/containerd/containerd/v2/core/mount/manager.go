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

package mount

import (
	"context"
	"time"
)

// Manager handles activating a mount array to be mounted by the
// system. It supports custom mount types that can be handled by
// plugins and don't need to be directly mountable. For example, this
// can be used to do device activation and setting up process or
// sockets such as for fuse or tcmu.
// The returned activation info will contain the remaining mounts
// which must be performed by the system, likely in a container's
// mount namespace. Any mounts or devices activated by the mount
// manager will be done outside the container's namespace.
type Manager interface {
	Activate(context.Context, string, []Mount, ...ActivateOpt) (ActivationInfo, error)
	Deactivate(context.Context, string) error
	Info(context.Context, string) (ActivationInfo, error)
	Update(context.Context, ActivationInfo, ...string) (ActivationInfo, error)
	List(context.Context, ...string) ([]ActivationInfo, error)
}

// Handler is an interface for plugins to perform a mount which is managed
// by a MountManager. The MountManager will be responsible for associating
// mount types to MountHandlers and determining what the plugin should be used.
// The Handler interface is intended to be used for custom mount plugins
// and does not replace the mount calls for system mounts.
type Handler interface {
	Mount(context.Context, Mount, string, []ActiveMount) (ActiveMount, error)
	Unmount(context.Context, string) error
}

// Transformer is an interface that can make changes to the mount based on
// the previous mount state. This can be used to update the values of the
// mount, such as with formatting, or for mount initialization that do not
// require runtime state, such as device formatting.
type Transformer interface {
	Transform(context.Context, Mount, []ActiveMount) (Mount, error)
}

// ActivateOptions are used to modify activation behavior. Activate may be
// performed differently based on the different scenarios, such as mounting
// to view a filesystem or preparing a filesystem for a container that may
// have specific runtime requirements. The runtime for a container may also
// have different capabilities that would allow it to handle mounts which
// would not need to be handled by the mount manager.
type ActivateOptions struct {
	// Labels are the labels to use for the activation
	Labels map[string]string

	// Temporary specifies that the mount will be used temporarily
	// and all mounts should be performed
	Temporary bool

	// AllowMountTypes indicates that the caller will handle the specified
	// mount types and should not be handled by the mount manager even if
	// there is a configured handler for the type.
	// Use "/*" suffix to match prepare mount types, such as "format/*".
	AllowMountTypes []string
}

// ActivateOpt is a function option for Activate
type ActivateOpt func(*ActivateOptions)

// WithTemporary indicates that the activation is for temporary access
// of the mounts. All mounts should be performed and a single bind
// mount is returned to access to the mounted filesystem.
func WithTemporary(o *ActivateOptions) {
	o.Temporary = true
}

// WithLabels specifies the labels to use for the stored activation info.
func WithLabels(labels map[string]string) ActivateOpt {
	return func(o *ActivateOptions) {
		o.Labels = labels
	}
}

// WithAllowMountType indicates the mount types that the peformer
// of the mounts will support. Even if there is a custom handler
// registered for the mount type to the mount handler, these mounts
// should not performed unless required to support subsequent mounts.
// For prepare mount types, use "/*" suffix to match all prepare types,
// such as "format/*".
func WithAllowMountType(mountType string) ActivateOpt {
	return func(o *ActivateOptions) {
		o.AllowMountTypes = append(o.AllowMountTypes, mountType)
	}
}

// ActiveMount represents a mount which has been mounted by a
// MountHandler or directly mounted by a mount manager.
type ActiveMount struct {
	Mount
	MountedAt *time.Time

	// MountPoint is the filesystem mount location
	MountPoint string

	// MountData is metadata used by the mount type which can also be used by
	// subsequent mounts.
	MountData map[string]string
}

// ActivationInfo represents the state of an active set of mounts being managed by a
// mount manager. The Name is unique and can be used to reference the activation
// from other resources.
type ActivationInfo struct {
	Name string

	// Active are the mounts which was successfully mounted on activate
	Active []ActiveMount

	// System is the list of system mounts to access the filesystem root
	// This will always be non-empty and a bind mount will be created
	// and filled in here when all mounts are performed
	System []Mount
	Labels map[string]string
}
