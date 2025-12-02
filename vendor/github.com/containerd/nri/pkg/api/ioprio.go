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

// FromOCILinuxIOPriority returns a LinuxIOPriority corresponding to the
// OCI LinuxIOPriority.
func FromOCILinuxIOPriority(o *rspec.LinuxIOPriority) *LinuxIOPriority {
	if o == nil {
		return nil
	}

	ioprio := &LinuxIOPriority{
		Class:    FromOCIIOPriorityClass(o.Class),
		Priority: int32(o.Priority),
	}

	return ioprio
}

// ToOCI returns the OCI LinuxIOPriority corresponding to the LinuxIOPriority.
func (ioprio *LinuxIOPriority) ToOCI() *rspec.LinuxIOPriority {
	if ioprio == nil {
		return nil
	}

	return &rspec.LinuxIOPriority{
		Class:    ioprio.Class.ToOCI(),
		Priority: int(ioprio.Priority),
	}
}

// FromOCIIOPrioClass returns the IOPrioClass corresponding the the given
// OCI IOPriorityClass.
func FromOCIIOPriorityClass(o rspec.IOPriorityClass) IOPrioClass {
	return IOPrioClass(IOPrioClass_value[string(o)])
}

// ToOCI returns the OCI IOPriorityClass corresponding to the given
// IOPrioClass.
func (c IOPrioClass) ToOCI() rspec.IOPriorityClass {
	if c == IOPrioClass_IOPRIO_CLASS_NONE {
		return rspec.IOPriorityClass("")
	}
	return rspec.IOPriorityClass(IOPrioClass_name[int32(c)])
}
