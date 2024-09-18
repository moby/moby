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

package specs

import "errors"

// errDeprecated is returned for the ToOCI functions below.
// This should provide better guidance for user when migrating from the API
// below to the APIs provided in the cdi package.
var errDeprecated = errors.New("deprecated; Use cdi package functions instead")

// ToOCI returns the opencontainers runtime Spec Hook for this Hook.
//
// Deprecated: This function has been moved to tags.cncf.io/container-device-interface/pkg/cdi.Hook.toOCI
// and made private.
func (h *Hook) ToOCI() error {
	return errDeprecated
}

// ToOCI returns the opencontainers runtime Spec Mount for this Mount.
//
// Deprecated: This function has been moved to tags.cncf.io/container-device-interface/pkg/cdi.Mount.toOCI
// and made private.
func (m *Mount) ToOCI() error {
	return errDeprecated
}

// ToOCI returns the opencontainers runtime Spec LinuxDevice for this DeviceNode.
//
// Deprecated: This function has been moved to tags.cncf.io/container-device-interface/pkg/cdi.DeviceNode.toOCI
// and made private.
func (d *DeviceNode) ToOCI() error {
	return errDeprecated
}

// ToOCI returns the opencontainers runtime Spec LinuxIntelRdt for this IntelRdt config.
//
// Deprecated: This function has been moved to tags.cncf.io/container-device-interface/pkg/cdi.IntelRdt.toOCI
// and made private.
func (i *IntelRdt) ToOCI() error {
	return errDeprecated
}
