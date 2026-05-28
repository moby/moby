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

// FromOCILinuxScheduler returns a LinuxScheduler corresponding to the OCI
// Scheduler.
func FromOCILinuxScheduler(o *rspec.Scheduler) *LinuxScheduler {
	if o == nil {
		return nil
	}

	sch := &LinuxScheduler{
		Policy:   FromOCISchedulerPolicy(o.Policy),
		Nice:     o.Nice,
		Priority: o.Priority,
		Flags:    FromOCILinuxSchedulerFlags(o.Flags),
		Runtime:  o.Runtime,
		Deadline: o.Deadline,
		Period:   o.Period,
	}

	return sch
}

// ToOCI returns the OCI Scheduler corresponding to the LinuxScheduler.
func (sch *LinuxScheduler) ToOCI() *rspec.Scheduler {
	if sch == nil {
		return nil
	}

	if sch.Policy == LinuxSchedulerPolicy_SCHED_NONE {
		return nil
	}

	return &rspec.Scheduler{
		Policy:   sch.Policy.ToOCI(),
		Nice:     sch.Nice,
		Priority: sch.Priority,
		Flags:    ToOCILinuxSchedulerFlags(sch.Flags),
		Runtime:  sch.Runtime,
		Deadline: sch.Deadline,
		Period:   sch.Period,
	}
}

// FromOCISchedulerPolicy returns the SchedulerPolicy corresponding to the
// given OCI SchedulerPolicy.
func FromOCISchedulerPolicy(o rspec.LinuxSchedulerPolicy) LinuxSchedulerPolicy {
	return LinuxSchedulerPolicy(LinuxSchedulerPolicy_value[string(o)])
}

// ToOCI returns the OCI SchedulerPolicy corresponding to the given
// SchedulerPolicy.
func (p LinuxSchedulerPolicy) ToOCI() rspec.LinuxSchedulerPolicy {
	if p == LinuxSchedulerPolicy_SCHED_NONE {
		return rspec.LinuxSchedulerPolicy("")
	}
	return rspec.LinuxSchedulerPolicy(LinuxSchedulerPolicy_name[int32(p)])
}

// FromOCILinuxSchedulerFlags returns the LinuxSchedulerFlags corresponding to
// the given OCI LinuxSchedulerFlags.
func FromOCILinuxSchedulerFlags(o []rspec.LinuxSchedulerFlag) []LinuxSchedulerFlag {
	if o == nil {
		return nil
	}

	flags := make([]LinuxSchedulerFlag, len(o))
	for i, f := range o {
		flags[i] = LinuxSchedulerFlag(LinuxSchedulerFlag_value[string(f)])
	}

	return flags
}

// ToOCILinuxSchedulerFlags returns the OCI LinuxSchedulerFlags corresponding
// to the LinuxSchedulerFlags.
func ToOCILinuxSchedulerFlags(f []LinuxSchedulerFlag) []rspec.LinuxSchedulerFlag {
	if f == nil {
		return nil
	}

	flags := make([]rspec.LinuxSchedulerFlag, len(f))
	for i, f := range f {
		flags[i] = rspec.LinuxSchedulerFlag(LinuxSchedulerFlag_name[int32(f)])
	}

	return flags
}
