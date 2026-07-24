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

package v2

// maxSocketDirLen is the maximum length of the socket directory path.
// Unix socket paths are limited to 108 characters on Linux, minus a
// null terminator gives 107 usable characters. The socket path passed
// to the kernel is directory + "/" (1) + sha256 hash (64); the
// "unix://" scheme is stripped before net.Listen and does not count.
// So the directory can be at most 107 - 1 - 64 = 42.
const maxSocketDirLen = 42
