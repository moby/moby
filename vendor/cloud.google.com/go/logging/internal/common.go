// Copyright 2016 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"unicode"
)

const (
	// ProdAddr is the production address.
	ProdAddr = "logging.googleapis.com:443"
)

// InstrumentOnce guards instrumenting logs one time
var InstrumentOnce = new(sync.Once)

// LogPath creates a formatted path from a parent and a logID.
func LogPath(parent, logID string) string {
	logID = strings.Replace(logID, "/", "%2F", -1)
	return fmt.Sprintf("%s/logs/%s", parent, logID)
}

// LogIDFromPath parses and returns the ID from a log path.
func LogIDFromPath(parent, path string) string {
	start := len(parent) + len("/logs/")
	if len(path) < start {
		return ""
	}
	logID := path[start:]
	return strings.Replace(logID, "%2F", "/", -1)
}

// VersionGo returns the Go runtime version. The returned string
// has no whitespace, suitable for reporting in header.
func VersionGo() string {
	const develPrefix = "devel +"

	s := runtime.Version()
	if strings.HasPrefix(s, develPrefix) {
		s = s[len(develPrefix):]
		if p := strings.IndexFunc(s, unicode.IsSpace); p >= 0 {
			s = s[:p]
		}
		return s
	}

	notSemverRune := func(r rune) bool {
		return !strings.ContainsRune("0123456789.", r)
	}

	if strings.HasPrefix(s, "go1") {
		s = s[2:]
		var prerelease string
		if p := strings.IndexFunc(s, notSemverRune); p >= 0 {
			s, prerelease = s[:p], s[p:]
		}
		if strings.HasSuffix(s, ".") {
			s += "0"
		} else if strings.Count(s, ".") < 2 {
			s += ".0"
		}
		if prerelease != "" {
			s += "-" + prerelease
		}
		return s
	}
	return "UNKNOWN"
}
