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

// FromOCILinuxMemoryPolicy returns a LinuxMemoryPolicy corresponding to the
// OCI LinuxMemoryPolicy.
func FromOCILinuxMemoryPolicy(o *rspec.LinuxMemoryPolicy) *LinuxMemoryPolicy {
	if o == nil {
		return nil
	}

	memoryPolicy := &LinuxMemoryPolicy{
		Mode:  FromOCIMemoryPolicyMode(o.Mode),
		Nodes: o.Nodes,
		Flags: FromOCIMemoryPolicyFlags(o.Flags...),
	}

	return memoryPolicy
}

// ToOCI returns the OCI LinuxMemoryPolicy corresponding to the LinuxMemoryPolicy.
func (memoryPolicy *LinuxMemoryPolicy) ToOCI() *rspec.LinuxMemoryPolicy {
	if memoryPolicy == nil {
		return nil
	}

	return &rspec.LinuxMemoryPolicy{
		Mode:  memoryPolicy.Mode.ToOCI(),
		Nodes: memoryPolicy.Nodes,
		Flags: ToOCIMemoryPolicyFlags(memoryPolicy.Flags...),
	}
}

// FromOCIMemoryPolicyMode returns memory policy mode corresponding to the
// mode in the OCI LinuxMemoryPolicy.
func FromOCIMemoryPolicyMode(mode rspec.MemoryPolicyModeType) MpolMode {
	return MpolMode(MpolMode_value[string(mode)])
}

// FromOCIMemoryPolicyFlags returns memory policy flags corresponding to the
// flags in the OCI LinuxMemoryPolicy.
func FromOCIMemoryPolicyFlags(flags ...rspec.MemoryPolicyFlagType) []MpolFlag {
	if flags == nil {
		return nil
	}

	mpolFlags := make([]MpolFlag, len(flags))
	for i, flag := range flags {
		mpolFlags[i] = MpolFlag(MpolFlag_value[string(flag)])
	}
	return mpolFlags
}

// ToOCI returns the OCI MemoryPolicyMode corresponding to the given
// memory policy mode.
func (mode MpolMode) ToOCI() rspec.MemoryPolicyModeType {
	return rspec.MemoryPolicyModeType(MpolMode_name[int32(mode)])
}

// ToOCIMemoryPolicyFlags returns OCI MemoryPolicyFlags corresponding
// to given memory policy mode flags.
func ToOCIMemoryPolicyFlags(flags ...MpolFlag) []rspec.MemoryPolicyFlagType {
	if flags == nil {
		return nil
	}

	ociFlags := make([]rspec.MemoryPolicyFlagType, len(flags))
	for i, flag := range flags {
		ociFlags[i] = rspec.MemoryPolicyFlagType(MpolFlag_name[int32(flag)])
	}
	return ociFlags
}
