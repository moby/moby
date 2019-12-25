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

package runc

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func (r *Runc) command(context context.Context, args ...string) *exec.Cmd {
	command := r.Command
	if command == "" {
		command = DefaultCommand
	}
	cmd := exec.CommandContext(context, command, append(r.args(), args...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: r.Setpgid,
	}
	cmd.Env = filterEnv(os.Environ(), "NOTIFY_SOCKET") // NOTIFY_SOCKET introduces a special behavior in runc but should only be set if invoked from systemd
	if r.PdeathSignal != 0 {
		cmd.SysProcAttr.Pdeathsig = r.PdeathSignal
	}

	return cmd
}

func filterEnv(in []string, names ...string) []string {
	out := make([]string, 0, len(in))
loop0:
	for _, v := range in {
		for _, k := range names {
			if strings.HasPrefix(v, k+"=") {
				continue loop0
			}
		}
		out = append(out, v)
	}
	return out
}
