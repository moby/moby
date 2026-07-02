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

// Package manager is used to manage mounts in a bolt database, normally
// backed by a tempfs.
/*
Database schema

	v1
	╘══*namespace*
	   ├──mounts
	   │  ╘══*mount name*
	   │     ├──id : <varuint64>                  - Unique ID for mount (auto incrementing)
	   │     ├──createdat : <binary time>         - Created at
	   │     ├──updatedat : <binary time>         - Updated at
	   │     ├──lease : <string>                  - Lease
	   │     ├──active
	   │     │  ╘══*order*
	   │     │     ├──mountedat : <binary time>   - Mounted at
	   │     │     ├──state : <enum>              - (0 - unmounted, 1 - filesystem, 2 - device, 3 - process)
	   │     │     ├──type : <string>             - Mount type
	   │     │     ├──source : <string>           - Mount source
	   │     │     ├──target : <string>           - Mount target (relative to previous mount point)
	   │     │     ├──mp : <string>               - Mount point for filesystem mount
	   │     │     ├──mpg : <bool>                - Whether the mount point is auto generated (NOT USED)
	   │     │     ├──dev : <string>              - Device active for this mount (NOT USED)
	   │     │     ├──pid : <int>                 - Process created for this mount (NOT USED)
	   │     │     └──options : <string>          - Comma separate options
	   │     ├──system
	   │     │  ╘══*order*
	   │     │     ├──type : <string>             - Mount type
	   │     │     ├──source : <string>           - Mount source
	   │     │     ├──target : <string>           - Mount target (relative to previous mount point)
	   │     │     └──options : <string>          - Comma separate options
	   │     └──labels
	   │        ╘══*key* : <string>               - Label value
	   ├──leases
	   │  ╘══*lease id*
	   │     ╘══*mount name*: nil
	   └──unmountq                                (CURRENTLY NOT USED, may remove)
	      └──*mount name + auto-increment*
	         ├──type : <string>                   - Mount type
	         ├──target : <string>                 - Path to check && unmount
	         ├──rm : <bool>                       - Whether to remove target after unmount
	         ├──dev : <string>                    - Device to check before unmount
	         ├──pid : <int>                       - Process to check and kill
	         ├──target : <string>                 - Path to unmount
	         ├──state : <enum>                    - (0 - unmounted, 1 - filesystem, 2 - device, 3 - process)
	         └─ order : <int>                     - Order in which was mounted, unmount high to low
*/
package manager

var (
	bucketKeyID         = []byte("id")
	bucketKeyMounts     = []byte("mounts")
	bucketKeyLeases     = []byte("leases")
	bucketKeyLease      = []byte("lease")
	bucketKeyActive     = []byte("active")
	bucketKeyType       = []byte("type")
	bucketKeyMountedAt  = []byte("mat")
	bucketKeyMountPoint = []byte("mp")
	bucketKeyLabels     = []byte("labels")

	labelGCContainerBackRef = []byte("containerd.io/gc.bref.container")
	labelGCContentBackRef   = []byte("containerd.io/gc.bref.content")
	labelGCImageBackRef     = []byte("containerd.io/gc.bref.image")
	labelGCSnapBackRef      = []byte("containerd.io/gc.bref.snapshot.")
)
