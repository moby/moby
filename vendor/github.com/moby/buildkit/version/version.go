/*
   Copyright The BuildKit Authors.
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

import (
	"regexp"
	"sync"
)

const (
	defaultVersion = "0.0.0+unknown"
)

var (
	// Package is filled at linking time
	Package = "github.com/moby/buildkit"

	// Version holds the complete version number. Filled in at linking time.
	Version = defaultVersion

	// Revision is filled with the VCS (e.g. git) revision being used to build
	// the program at linking time.
	Revision = ""
)

var (
	reRelease *regexp.Regexp
	reDev     *regexp.Regexp
	reOnce    sync.Once
)

func UserAgent() string {
	uaVersion := defaultVersion

	reOnce.Do(func() {
		reRelease = regexp.MustCompile(`^(v[0-9]+\.[0-9]+)\.[0-9]+$`)
		reDev = regexp.MustCompile(`^(v[0-9]+\.[0-9]+)\.[0-9]+`)
	})

	if matches := reRelease.FindAllStringSubmatch(Version, 1); len(matches) > 0 {
		uaVersion = matches[0][1]
	} else if matches := reDev.FindAllStringSubmatch(Version, 1); len(matches) > 0 {
		uaVersion = matches[0][1] + "-dev"
	}

	return "buildkit/" + uaVersion
}
