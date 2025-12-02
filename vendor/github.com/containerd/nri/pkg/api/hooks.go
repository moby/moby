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
	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

// Append appends the given hooks to the existing ones.
func (hooks *Hooks) Append(h *Hooks) *Hooks {
	if h == nil {
		return hooks
	}
	hooks.Prestart = append(hooks.Prestart, h.Prestart...)
	hooks.CreateRuntime = append(hooks.CreateRuntime, h.CreateRuntime...)
	hooks.CreateContainer = append(hooks.CreateContainer, h.CreateContainer...)
	hooks.StartContainer = append(hooks.StartContainer, h.StartContainer...)
	hooks.Poststart = append(hooks.Poststart, h.Poststart...)
	hooks.Poststop = append(hooks.Poststop, h.Poststop...)

	return hooks
}

// Hooks returns itself it any of its hooks is set. Otherwise it returns nil.
func (hooks *Hooks) Hooks() *Hooks {
	if hooks == nil {
		return nil
	}

	if len(hooks.Prestart) > 0 {
		return hooks
	}
	if len(hooks.CreateRuntime) > 0 {
		return hooks
	}
	if len(hooks.CreateContainer) > 0 {
		return hooks
	}
	if len(hooks.StartContainer) > 0 {
		return hooks
	}
	if len(hooks.Poststart) > 0 {
		return hooks
	}
	if len(hooks.Poststop) > 0 {
		return hooks
	}

	return nil
}

// ToOCI returns the hook for an OCI runtime Spec.
func (h *Hook) ToOCI() rspec.Hook {
	return rspec.Hook{
		Path:    h.Path,
		Args:    DupStringSlice(h.Args),
		Env:     DupStringSlice(h.Env),
		Timeout: h.Timeout.Get(),
	}
}

// FromOCIHooks returns hooks from an OCI runtime Spec.
func FromOCIHooks(o *rspec.Hooks) *Hooks {
	if o == nil {
		return nil
	}
	return &Hooks{
		Prestart:        FromOCIHookSlice(o.Prestart),
		CreateRuntime:   FromOCIHookSlice(o.CreateRuntime),
		CreateContainer: FromOCIHookSlice(o.CreateContainer),
		StartContainer:  FromOCIHookSlice(o.StartContainer),
		Poststart:       FromOCIHookSlice(o.Poststart),
		Poststop:        FromOCIHookSlice(o.Poststop),
	}
}

// FromOCIHookSlice returns a hook slice from an OCI runtime Spec.
func FromOCIHookSlice(o []rspec.Hook) []*Hook {
	var hooks []*Hook
	for _, h := range o {
		hooks = append(hooks, &Hook{
			Path:    h.Path,
			Args:    DupStringSlice(h.Args),
			Env:     DupStringSlice(h.Env),
			Timeout: Int(h.Timeout),
		})
	}
	return hooks
}
