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

package api

import (
	"fmt"
)

// AddPlugin records a plugin for the validation request.
func (v *ValidateContainerAdjustmentRequest) AddPlugin(name, index string) {
	v.Plugins = append(v.Plugins, &PluginInstance{
		Name:  name,
		Index: index,
	})
}

// AddResponse records the container adjustments and updates to validate from a CreateContainerResponse.
func (v *ValidateContainerAdjustmentRequest) AddResponse(rpl *CreateContainerResponse) {
	v.Adjust = rpl.Adjust
	v.Update = rpl.Update
}

// AddOwners sets the owning plugins for the container adjustment request.
func (v *ValidateContainerAdjustmentRequest) AddOwners(owners *OwningPlugins) {
	v.Owners = owners
}

// ValidationResult returns the validation result as an error (non-nil if rejected).
func (v *ValidateContainerAdjustmentResponse) ValidationResult(plugin string) error {
	if !v.Reject {
		return nil
	}

	reason := v.Reason
	if reason == "" {
		reason = "unknown rejection reason"
	}

	return fmt.Errorf("validator %q rejected container adjustment, reason: %s", plugin, reason)
}

// GetPluginMap returns a map of plugin name to PluginInstance.
func (v *ValidateContainerAdjustmentRequest) GetPluginMap() map[string]*PluginInstance {
	if v == nil {
		return nil
	}

	plugins := make(map[string]*PluginInstance)
	for _, p := range v.Plugins {
		plugins[p.Name] = &PluginInstance{Name: p.Name}
		plugins[p.Index+"-"+p.Name] = p
	}

	return plugins
}
