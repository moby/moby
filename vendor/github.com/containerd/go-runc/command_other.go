//go:build !linux

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
)

func (r *Runc) command(context context.Context, args ...string) *exec.Cmd {
	return r.commandWithCustomLogFile(context, "", args...)
}

func (r *Runc) commandWithCustomLogFile(context context.Context, logFile string, args ...string) *exec.Cmd {
	command := r.Command
	if command == "" {
		command = DefaultCommand
	}
	cmd := exec.CommandContext(context, command, append(r.args(logFile), args...)...) // #nosec G702 -- executing the caller-configured runtime is the purpose of this package.
	cmd.Env = append(os.Environ(), extraEnv(context)...)
	cmd.Dir = r.WorkDir
	return cmd
}
