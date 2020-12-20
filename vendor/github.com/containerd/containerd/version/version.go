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

package version

import "runtime"

var (
	// Package is filled at linking time
	Package = "github.com/containerd/containerd"

	// Version holds the complete version number. Filled in at linking time.
	Version = "1.4.0+unknown"

	// Revision is filled with the VCS (e.g. git) revision being used to build
	// the program at linking time.
	Revision = ""

	// GoVersion is Go tree's version.
	GoVersion = runtime.Version()
)
