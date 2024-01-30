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

package oci

import (
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func defaultMounts() []specs.Mount {
	return []specs.Mount{
		{
			Destination: "/dev",
			Type:        "devfs",
			Source:      "devfs",
			Options:     []string{"ruleset=4"},
		},
		{
			Destination: "/dev/fd",
			Type:        "fdescfs",
			Source:      "fdescfs",
		},
	}
}

// appendOSMounts modifies the mount spec to mount emulated Linux filesystems on FreeBSD,
// as per: https://wiki.freebsd.org/LinuxJails
func appendOSMounts(s *Spec, os string) {
	// No-op for FreeBSD containers
	if os != "linux" {
		return
	}
	/* The nosuid noexec options are for consistency with Linux mounts: on FreeBSD it is
	   by default impossible to execute anything from these filesystems.
	*/
	var mounts = []specs.Mount{
		{
			Destination: "/proc",
			Type:        "linprocfs",
			Source:      "linprocfs",
			Options:     []string{"nosuid", "noexec"},
		},
		{
			Destination: "/sys",
			Type:        "linsysfs",
			Source:      "linsysfs",
			Options:     []string{"nosuid", "noexec", "nodev"},
		},
	}

	s.Mounts = append(mounts, s.Mounts...)
}
