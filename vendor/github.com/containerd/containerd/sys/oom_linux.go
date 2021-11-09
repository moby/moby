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

package sys

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/containerd/containerd/pkg/userns"
	"golang.org/x/sys/unix"
)

const (
	// OOMScoreAdjMin is from OOM_SCORE_ADJ_MIN https://github.com/torvalds/linux/blob/v5.10/include/uapi/linux/oom.h#L9
	OOMScoreAdjMin = -1000
	// OOMScoreAdjMax is from OOM_SCORE_ADJ_MAX https://github.com/torvalds/linux/blob/v5.10/include/uapi/linux/oom.h#L10
	OOMScoreAdjMax = 1000
)

// AdjustOOMScore sets the oom score for the provided pid. If the provided score
// is out of range (-1000 - 1000), it is clipped to the min/max value.
func AdjustOOMScore(pid, score int) error {
	if score > OOMScoreAdjMax {
		score = OOMScoreAdjMax
	} else if score < OOMScoreAdjMin {
		score = OOMScoreAdjMin
	}
	return SetOOMScore(pid, score)
}

// SetOOMScore sets the oom score for the provided pid
func SetOOMScore(pid, score int) error {
	if score > OOMScoreAdjMax || score < OOMScoreAdjMin {
		return fmt.Errorf("value out of range (%d): OOM score must be between %d and %d", score, OOMScoreAdjMin, OOMScoreAdjMax)
	}
	path := fmt.Sprintf("/proc/%d/oom_score_adj", pid)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.WriteString(strconv.Itoa(score)); err != nil {
		if os.IsPermission(err) && (!runningPrivileged() || userns.RunningInUserNS()) {
			return nil
		}
		return err
	}
	return nil
}

// GetOOMScoreAdj gets the oom score for a process. It returns 0 (zero) if either
// no oom score is set, or a sore is set to 0.
func GetOOMScoreAdj(pid int) (int, error) {
	path := fmt.Sprintf("/proc/%d/oom_score_adj", pid)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// runningPrivileged returns true if the effective user ID of the
// calling process is 0
func runningPrivileged() bool {
	return unix.Geteuid() == 0
}
