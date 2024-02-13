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

// Package epoch provides SOURCE_DATE_EPOCH utilities.
package epoch

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// SourceDateEpochEnv is the SOURCE_DATE_EPOCH env var.
// See https://reproducible-builds.org/docs/source-date-epoch/
const SourceDateEpochEnv = "SOURCE_DATE_EPOCH"

// SourceDateEpoch returns the SOURCE_DATE_EPOCH env var as *time.Time.
// If the env var is not set, SourceDateEpoch returns nil without an error.
func SourceDateEpoch() (*time.Time, error) {
	v, ok := os.LookupEnv(SourceDateEpochEnv)
	if !ok || v == "" {
		return nil, nil // not an error
	}
	i64, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value %q: %w", SourceDateEpochEnv, v, err)
	}
	unix := time.Unix(i64, 0).UTC()
	return &unix, nil
}

// SourceDateEpochOrNow returns the SOURCE_DATE_EPOCH time if available,
// otherwise returns the current time.
func SourceDateEpochOrNow() time.Time {
	epoch, err := SourceDateEpoch()
	if err != nil {
		logrus.WithError(err).Warnf("Invalid %s", SourceDateEpochEnv)
	}
	if epoch != nil {
		return *epoch
	}
	return time.Now().UTC()
}

// SetSourceDateEpoch sets the SOURCE_DATE_EPOCH env var.
func SetSourceDateEpoch(tm time.Time) {
	os.Setenv(SourceDateEpochEnv, fmt.Sprintf("%d", tm.Unix()))
}

// UnsetSourceDateEpoch unsets the SOURCE_DATE_EPOCH env var.
func UnsetSourceDateEpoch() {
	os.Unsetenv(SourceDateEpochEnv)
}
