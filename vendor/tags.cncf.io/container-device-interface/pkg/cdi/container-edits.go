/*
   Copyright © 2021 The CDI Authors

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

package cdi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	oci "github.com/opencontainers/runtime-spec/specs-go"
	"tags.cncf.io/container-device-interface/internal/ociedit"
	cdi "tags.cncf.io/container-device-interface/specs-go"
)

const (
	// PrestartHook is the name of the OCI "prestart" hook.
	PrestartHook = "prestart"
	// CreateRuntimeHook is the name of the OCI "createRuntime" hook.
	CreateRuntimeHook = "createRuntime"
	// CreateContainerHook is the name of the OCI "createContainer" hook.
	CreateContainerHook = "createContainer"
	// StartContainerHook is the name of the OCI "startContainer" hook.
	StartContainerHook = "startContainer"
	// PoststartHook is the name of the OCI "poststart" hook.
	PoststartHook = "poststart"
	// PoststopHook is the name of the OCI "poststop" hook.
	PoststopHook = "poststop"

	// NoPermissions requests empty cgroup permissions for a device.
	NoPermissions = "none"
)

var (
	// Names of recognized hooks.
	validHookNames = map[string]struct{}{
		PrestartHook:        {},
		CreateRuntimeHook:   {},
		CreateContainerHook: {},
		StartContainerHook:  {},
		PoststartHook:       {},
		PoststopHook:        {},
	}
)

// ContainerEdits represent updates to be applied to an OCI Spec.
// These updates can be specific to a CDI device, or they can be
// specific to a CDI Spec. In the former case these edits should
// be applied to all OCI Specs where the corresponding CDI device
// is injected. In the latter case, these edits should be applied
// to all OCI Specs where at least one devices from the CDI Spec
// is injected.
type ContainerEdits struct {
	*cdi.ContainerEdits
}

// Apply edits to the given OCI Spec. Updates the OCI Spec in place.
// Returns an error if the update fails.
func (e *ContainerEdits) Apply(spec *oci.Spec) error {
	if spec == nil {
		return errors.New("can't edit nil OCI Spec")
	}
	if e == nil || e.ContainerEdits == nil {
		return nil
	}

	editor, err := ociedit.NewSpecEditor(spec)
	if err != nil {
		return fmt.Errorf("error creating spec editor: %w", err)
	}
	if len(e.Env) > 0 {
		editor.AddMultipleProcessEnv(e.Env)
	}

	for _, d := range e.DeviceNodes {
		dn := DeviceNode{d}

		err := dn.fillMissingInfo()
		if err != nil {
			return err
		}
		dev := dn.toOCI()
		if dev.UID == nil && spec.Process != nil {
			if uid := spec.Process.User.UID; uid > 0 {
				dev.UID = &uid
			}
		}
		if dev.GID == nil && spec.Process != nil {
			if gid := spec.Process.User.GID; gid > 0 {
				dev.GID = &gid
			}
		}

		editor.RemoveDevice(dev.Path)
		editor.AddDevice(dev)

		if dev.Type == "b" || dev.Type == "c" {
			access := d.Permissions
			switch access {
			case "":
				access = "rwm"
			case NoPermissions:
				access = ""
			}
			editor.AddLinuxResourcesDevice(true, dev.Type, &dev.Major, &dev.Minor, access)
		}
	}

	if len(e.NetDevices) > 0 {
		for _, dev := range e.NetDevices {
			editor.SetLinuxNetDevice(dev.HostInterfaceName, (&LinuxNetDevice{dev}).toOCI())
		}
	}

	if len(e.Mounts) > 0 {
		for _, m := range e.Mounts {
			mnt := &Mount{m}

			editor.RemoveMount(m.ContainerPath)

			if !specHasUserNamespace(spec) {
				editor.AddMount(mnt.toOCI())
			} else {
				editor.AddMount(mnt.toOCI(withIDMapForBindMount()))
			}
		}
		sortMounts(editor)
	}

	for _, h := range e.Hooks {
		ociHook := (&Hook{h}).toOCI()
		switch h.HookName {
		case PrestartHook:
			editor.AddPreStartHook(ociHook)
		case PoststartHook:
			editor.AddPostStartHook(ociHook)
		case PoststopHook:
			editor.AddPostStopHook(ociHook)
		case CreateRuntimeHook:
			editor.AddCreateRuntimeHook(ociHook)
		case CreateContainerHook:
			editor.AddCreateContainerHook(ociHook)
		case StartContainerHook:
			editor.AddStartContainerHook(ociHook)
		default:
			return fmt.Errorf("unknown hook name %q", h.HookName)
		}
	}

	if e.IntelRdt != nil {
		editor.SetLinuxIntelRdt((&IntelRdt{e.IntelRdt}).toOCI())
	}

	for _, additionalGID := range e.AdditionalGIDs {
		if additionalGID == 0 {
			continue
		}
		editor.AddProcessAdditionalGID(additionalGID)
	}

	return nil
}

// Validate container edits.
func (e *ContainerEdits) Validate() error {
	if e == nil || e.ContainerEdits == nil {
		return nil
	}

	if err := ValidateEnv(e.Env); err != nil {
		return fmt.Errorf("invalid container edits: %w", err)
	}
	for _, d := range e.DeviceNodes {
		if err := (&DeviceNode{d}).Validate(); err != nil {
			return err
		}
	}
	for _, h := range e.Hooks {
		if err := (&Hook{h}).Validate(); err != nil {
			return err
		}
	}
	for _, m := range e.Mounts {
		if err := (&Mount{m}).Validate(); err != nil {
			return err
		}
	}
	if e.IntelRdt != nil {
		if err := (&IntelRdt{e.IntelRdt}).Validate(); err != nil {
			return err
		}
	}
	if err := ValidateNetDevices(e.NetDevices); err != nil {
		return err
	}

	return nil
}

// Append other edits into this one. If called with a nil receiver,
// allocates and returns newly allocated edits.
func (e *ContainerEdits) Append(o *ContainerEdits) *ContainerEdits {
	if o == nil || o.ContainerEdits == nil {
		return e
	}
	if e == nil {
		e = &ContainerEdits{}
	}
	if e.ContainerEdits == nil {
		e.ContainerEdits = &cdi.ContainerEdits{}
	}

	e.Env = append(e.Env, o.Env...)
	e.DeviceNodes = append(e.DeviceNodes, o.DeviceNodes...)
	e.NetDevices = append(e.NetDevices, o.NetDevices...)
	e.Hooks = append(e.Hooks, o.Hooks...)
	e.Mounts = append(e.Mounts, o.Mounts...)
	if o.IntelRdt != nil {
		e.IntelRdt = o.IntelRdt
	}
	e.AdditionalGIDs = append(e.AdditionalGIDs, o.AdditionalGIDs...)

	return e
}

// isEmpty returns true if these edits are empty. This is valid in a
// global Spec context but invalid in a Device context.
func (e *ContainerEdits) isEmpty() bool {
	if e == nil {
		return false
	}
	if len(e.Env) > 0 {
		return false
	}
	if len(e.DeviceNodes) > 0 {
		return false
	}
	if len(e.Hooks) > 0 {
		return false
	}
	if len(e.Mounts) > 0 {
		return false
	}
	if len(e.AdditionalGIDs) > 0 {
		return false
	}
	if e.IntelRdt != nil {
		return false
	}
	if len(e.NetDevices) > 0 {
		return false
	}
	return true
}

// ValidateEnv validates the given environment variables.
func ValidateEnv(env []string) error {
	for _, v := range env {
		if strings.IndexByte(v, byte('=')) <= 0 {
			return fmt.Errorf("invalid environment variable %q", v)
		}
	}
	return nil
}

// ValidateNetDevices validates the given net devices.
func ValidateNetDevices(devices []*cdi.LinuxNetDevice) error {
	var (
		hostSeen = map[string]string{}
		nameSeen = map[string]string{}
	)

	for _, dev := range devices {
		if err := (&LinuxNetDevice{dev}).Validate(); err != nil {
			return err
		}
		if other, ok := hostSeen[dev.HostInterfaceName]; ok {
			return fmt.Errorf("invalid linux net device, duplicate HostInterfaceName %q with names %q and %q",
				dev.HostInterfaceName, dev.Name, other)
		}
		hostSeen[dev.HostInterfaceName] = dev.Name

		if other, ok := nameSeen[dev.Name]; ok {
			return fmt.Errorf("invalid linux net device, duplicate Name %q with HostInterfaceName %q and %q",
				dev.Name, dev.HostInterfaceName, other)
		}
		nameSeen[dev.Name] = dev.HostInterfaceName
	}

	return nil
}

// LinuxNetDevice is a CDI Spec LinuxNetDevice wrapper, used for OCI conversion and validating.
type LinuxNetDevice struct {
	*cdi.LinuxNetDevice
}

// Validate LinuxNetDevice.
func (d *LinuxNetDevice) Validate() error {
	if d.HostInterfaceName == "" {
		return errors.New("invalid linux net device, empty HostInterfaceName")
	}
	if d.Name == "" {
		return errors.New("invalid linux net device, empty Name")
	}
	return nil
}

// DeviceNode is a CDI Spec DeviceNode wrapper, used for validating DeviceNodes.
type DeviceNode struct {
	*cdi.DeviceNode
}

// Validate a CDI Spec DeviceNode.
func (d *DeviceNode) Validate() error {
	validTypes := map[string]struct{}{
		"":  {},
		"b": {},
		"c": {},
		"u": {},
		"p": {},
	}

	if d.Path == "" {
		return errors.New("invalid (empty) device path")
	}
	if _, ok := validTypes[d.Type]; !ok {
		return fmt.Errorf("device %q: invalid type %q", d.Path, d.Type)
	}
	switch {
	case d.Permissions == "":
	case d.Permissions == NoPermissions:
	case strings.Trim(d.Permissions, "rwm") != "":
		return fmt.Errorf("device %q: invalid permissions %q",
			d.Path, d.Permissions)
	}

	return nil
}

// Hook is a CDI Spec Hook wrapper, used for validating hooks.
type Hook struct {
	*cdi.Hook
}

// Validate a hook.
func (h *Hook) Validate() error {
	if _, ok := validHookNames[h.HookName]; !ok {
		return fmt.Errorf("invalid hook name %q", h.HookName)
	}
	if h.Path == "" {
		return fmt.Errorf("invalid hook %q with empty path", h.HookName)
	}
	if err := ValidateEnv(h.Env); err != nil {
		return fmt.Errorf("invalid hook %q: %w", h.HookName, err)
	}
	return nil
}

// Mount is a CDI Mount wrapper, used for validating mounts.
type Mount struct {
	*cdi.Mount
}

// Validate a mount.
func (m *Mount) Validate() error {
	if m.HostPath == "" {
		return errors.New("invalid mount, empty host path")
	}
	if m.ContainerPath == "" {
		return errors.New("invalid mount, empty container path")
	}
	return nil
}

// IntelRdt is a CDI IntelRdt wrapper.
// This is used for validation and conversion to OCI specifications.
type IntelRdt struct {
	*cdi.IntelRdt
}

// ValidateIntelRdt validates the IntelRdt configuration.
//
// Deprecated: ValidateIntelRdt is deprecated use IntelRdt.Validate() instead.
func ValidateIntelRdt(i *cdi.IntelRdt) error {
	return (&IntelRdt{i}).Validate()
}

// Validate validates the IntelRdt configuration.
func (i *IntelRdt) Validate() error {
	// ClosID must be a valid Linux filename. Exception: "/" refers to the root CLOS.
	switch c := i.ClosID; {
	case c == "/":
	case len(c) >= 4096, c == ".", c == "..", strings.ContainsAny(c, "/\n"):
		return errors.New("invalid ClosID")
	}
	return nil
}

// sortMounts sorts the mounts in the given OCI Spec.
func sortMounts(editor ociedit.SpecEditor) {
	mounts := editor.Mounts()
	sort.Stable(orderedMounts(mounts))
	editor.SetMounts(mounts)
}

// orderedMounts defines how to sort an OCI Spec Mount slice.
// This is the almost the same implementation sa used by CRI-O and Docker,
// with a minor tweak for stable sorting order (easier to test):
//
//	https://github.com/moby/moby/blob/17.05.x/daemon/volumes.go#L26
type orderedMounts []oci.Mount

// Len returns the number of mounts. Used in sorting.
func (m orderedMounts) Len() int {
	return len(m)
}

// Less returns true if the number of parts (a/b/c would be 3 parts) in the
// mount indexed by parameter 1 is less than that of the mount indexed by
// parameter 2. Used in sorting.
func (m orderedMounts) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

// Swap swaps two items in an array of mounts. Used in sorting
func (m orderedMounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// parts returns the number of parts in the destination of a mount. Used in sorting.
func (m orderedMounts) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].Destination), string(os.PathSeparator))
}

// specHasUserNamespace returns true if the OCI Spec has a Linux UserNamespace.
func specHasUserNamespace(spec *oci.Spec) bool {
	if spec == nil || spec.Linux == nil {
		return false
	}
	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == oci.UserNamespace {
			return true
		}
	}
	return false
}
