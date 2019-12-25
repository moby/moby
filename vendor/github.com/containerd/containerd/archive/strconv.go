// +build windows

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

package archive

import (
	"strconv"
	"strings"
	"time"

	"archive/tar"
)

// Forked from https://github.com/golang/go/blob/master/src/archive/tar/strconv.go
// as archive/tar doesn't support CreationTime, but does handle PAX time parsing,
// and there's no need to re-invent the wheel.

// parsePAXTime takes a string of the form %d.%d as described in the PAX
// specification. Note that this implementation allows for negative timestamps,
// which is allowed for by the PAX specification, but not always portable.
func parsePAXTime(s string) (time.Time, error) {
	const maxNanoSecondDigits = 9

	// Split string into seconds and sub-seconds parts.
	ss, sn := s, ""
	if pos := strings.IndexByte(s, '.'); pos >= 0 {
		ss, sn = s[:pos], s[pos+1:]
	}

	// Parse the seconds.
	secs, err := strconv.ParseInt(ss, 10, 64)
	if err != nil {
		return time.Time{}, tar.ErrHeader
	}
	if len(sn) == 0 {
		return time.Unix(secs, 0), nil // No sub-second values
	}

	// Parse the nanoseconds.
	if strings.Trim(sn, "0123456789") != "" {
		return time.Time{}, tar.ErrHeader
	}
	if len(sn) < maxNanoSecondDigits {
		sn += strings.Repeat("0", maxNanoSecondDigits-len(sn)) // Right pad
	} else {
		sn = sn[:maxNanoSecondDigits] // Right truncate
	}
	nsecs, _ := strconv.ParseInt(sn, 10, 64) // Must succeed
	if len(ss) > 0 && ss[0] == '-' {
		return time.Unix(secs, -nsecs), nil // Negative correction
	}
	return time.Unix(secs, nsecs), nil
}
