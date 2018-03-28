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

// Mount is the lingua franca of containerd. A mount represents a
// serialized mount syscall. Components either emit or consume mounts.
type Mount struct {
	// Type specifies the host-specific of the mount.
	Type string
	// Source specifies where to mount from. Depending on the host system, this
	// can be a source path or device.
	Source string
	// Options contains zero or more fstab-style mount options. Typically,
	// these are platform specific.
	Options []string
}

// All mounts all the provided mounts to the provided target
func All(mounts []Mount, target string) error {
	for _, m := range mounts {
		if err := m.Mount(target); err != nil {
			return err
		}
	}
	return nil
}
