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

// TODO: Add comments to exported methods and functions.
//
//nolint:revive // exported symbols should have comment
package api

import (
	"fmt"
	"strings"
)

//
// Notes:
//   OwningPlugins, FieldOwners and CompoundFieldOwners are not protected
//   against concurrent access and therefore not goroutine safe.
//
//   None of these functions are used by plugins directly. These are used by
//   the runtime adaptation code to track container adjustments and updates
//   requested by plugins, and to detect conflicting requests.
//

func NewOwningPlugins() *OwningPlugins {
	return &OwningPlugins{
		Owners: make(map[string]*FieldOwners),
	}
}

// Note: ClaimHooks, HooksOwner, ClearHooks, and ClearRdt are handwritten
// functions as they did not match the pattern used to generate the rest of the
// functions.

func (o *OwningPlugins) ClaimHooks(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimHooks(plugin)
}

func (o *OwningPlugins) HooksOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_OciHooks.Key())
}

func (o *OwningPlugins) ClearHooks(id, plugin string) {
	o.mustOwnersFor(id).ClearHooks(plugin)
}

func (o *OwningPlugins) ClearRdt(id, plugin string) {
	o.mustOwnersFor(id).ClearRdt(plugin)
}

func (o *OwningPlugins) mustOwnersFor(id string) *FieldOwners {
	f, ok := o.Owners[id]
	if !ok {
		f = NewFieldOwners()
		o.Owners[id] = f
	}
	return f
}

func (o *OwningPlugins) ownersFor(id string) *FieldOwners {
	f, ok := o.Owners[id]
	if !ok {
		return nil
	}
	return f
}

func NewFieldOwners() *FieldOwners {
	return &FieldOwners{
		Simple:   make(map[int32]string),
		Compound: make(map[int32]*CompoundFieldOwners),
	}
}

func (f *FieldOwners) IsCompoundConflict(field int32, key, plugin string) error {
	m, ok := f.Compound[field]
	if !ok {
		f.Compound[field] = NewCompoundFieldOwners()
		return nil
	}

	other, claimed := m.Owners[key]
	if !claimed {
		return nil
	}

	clearer, ok := IsMarkedForRemoval(other)
	if ok {
		if clearer == plugin {
			return nil
		}
		other = clearer
	}

	return f.Conflict(field, plugin, other, key)
}

func (f *FieldOwners) IsSimpleConflict(field int32, plugin string) error {
	other, claimed := f.Simple[field]
	if !claimed {
		return nil
	}

	clearer, ok := IsMarkedForRemoval(other)
	if ok {
		if clearer == plugin {
			return nil
		}
		other = clearer
	}

	return f.Conflict(field, plugin, other)
}

func (f *FieldOwners) claimCompound(field int32, entry, plugin string) error {
	if err := f.IsCompoundConflict(field, entry, plugin); err != nil {
		return err
	}

	f.Compound[field].Owners[entry] = plugin
	return nil
}

func (f *FieldOwners) claimSimple(field int32, plugin string) error {
	if err := f.IsSimpleConflict(field, plugin); err != nil {
		return err
	}

	f.Simple[field] = plugin
	return nil
}

func (f *FieldOwners) ClaimHooks(plugin string) error {
	f.accumulateSimple(Field_OciHooks.Key(), plugin)
	return nil
}

func (f *FieldOwners) HooksOwner() (string, bool) {
	return f.simpleOwner(Field_OciHooks.Key())
}

func (f *FieldOwners) ClearHooks(plugin string) {
	f.clearSimple(Field_OciHooks.Key(), plugin)
}

func (f *FieldOwners) clearCompound(field int32, key, plugin string) {
	m, ok := f.Compound[field]
	if !ok {
		m = NewCompoundFieldOwners()
		f.Compound[field] = m
	}
	m.Owners[key] = MarkForRemoval(plugin)
}

func (f *FieldOwners) clearSimple(field int32, plugin string) {
	f.Simple[field] = MarkForRemoval(plugin)
}

func (f *FieldOwners) ClearRdt(plugin string) {
	f.clearSimple(int32(Field_RdtClosID), plugin)
	f.clearSimple(int32(Field_RdtSchemata), plugin)
	f.clearSimple(int32(Field_RdtEnableMonitoring), plugin)
}

func (f *FieldOwners) accumulateSimple(field int32, plugin string) {
	old, ok := f.simpleOwner(field)
	if ok {
		plugin = old + "," + plugin
	}
	f.Simple[field] = plugin
}

func (f *FieldOwners) Conflict(field int32, plugin, other string, qualifiers ...string) error {
	return fmt.Errorf("plugins %q and %q both tried to set %s",
		plugin, other, qualify(field, qualifiers...))
}

func (f *FieldOwners) compoundOwnerMap(field int32) (map[string]string, bool) {
	if f == nil {
		return nil, false
	}

	m, ok := f.Compound[field]
	if !ok {
		return nil, false
	}

	return m.Owners, true
}

func (f *FieldOwners) compoundOwner(field int32, key string) (string, bool) {
	if f == nil {
		return "", false
	}

	m, ok := f.Compound[field]
	if !ok {
		return "", false
	}

	plugin, ok := m.Owners[key]
	return plugin, ok
}

func (f *FieldOwners) simpleOwner(field int32) (string, bool) {
	if f == nil {
		return "", false
	}

	plugin, ok := f.Simple[field]
	return plugin, ok
}

func (o *OwningPlugins) NamespaceOwners(id string) (map[string]string, bool) {
	return o.ownersFor(id).compoundOwnerMap(Field_Namespace.Key())
}

func qualify(field int32, qualifiers ...string) string {
	return Field(field).String() + " " + strings.Join(append([]string{}, qualifiers...), " ")
}

func NewCompoundFieldOwners() *CompoundFieldOwners {
	return &CompoundFieldOwners{
		Owners: make(map[string]string),
	}
}

func (f Field) Key() int32 {
	return int32(f)
}
