/*
   Copyright The ocicrypt Authors.

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

package pkcs11

import (
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

var (
	envLock sync.Mutex
)

// setEnvVars sets the environment variables given in the map and locks the environment from
// modification with the same function; if successful, you *must* call restoreEnv with the return
// value from this function
func setEnvVars(env map[string]string) ([]string, error) {
	envLock.Lock()

	if len(env) == 0 {
		return nil, nil
	}

	oldenv := os.Environ()

	for k, v := range env {
		err := os.Setenv(k, v)
		if err != nil {
			restoreEnv(oldenv)
			return nil, errors.Wrapf(err, "Could not set environment variable '%s' to '%s'", k, v)
		}
	}

	return oldenv, nil
}

func arrayToMap(elements []string) map[string]string {
	o := make(map[string]string)

	for _, element := range elements {
		p := strings.SplitN(element, "=", 2)
		if len(p) == 2 {
			o[p[0]] = p[1]
		}
	}

	return o
}

// restoreEnv restores the environment to be exactly as given in the array of strings
// and unlocks the lock
func restoreEnv(envs []string) {
	if envs != nil && len(envs) >= 0 {
		target := arrayToMap(envs)
		curr := arrayToMap(os.Environ())

		for nc, vc := range curr {
			vt, ok := target[nc]
			if !ok {
				os.Unsetenv(nc)
			} else if vc == vt {
				delete(target, nc)
			}
		}

		for nt, vt := range target {
			os.Setenv(nt, vt)
		}
	}

	envLock.Unlock()
}

func getHostAndOsType() (string, string, string) {
	ht := ""
	ot := ""
	st := ""
	switch runtime.GOOS {
	case "linux":
		ot = "linux"
		st = "gnu"
		switch runtime.GOARCH {
		case "arm":
			ht = "arm"
		case "arm64":
			ht = "aarch64"
		case "amd64":
			ht = "x86_64"
		case "ppc64le":
			ht = "powerpc64le"
		case "s390x":
			ht = "s390x"
		}
	}
	return ht, ot, st
}
