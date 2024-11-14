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

package apparmor

// HostSupports returns true if apparmor is enabled for the host:
//   - On Linux returns true if apparmor is enabled, apparmor_parser is
//     present, and if we are not running docker-in-docker.
//   - On non-Linux returns false.
//
// This is derived from libcontainer/apparmor.IsEnabled(), with the addition
// of checks for apparmor_parser to be present and docker-in-docker.
func HostSupports() bool {
	return hostSupports()
}
