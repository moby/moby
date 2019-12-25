// +build !windows

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
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// NewPipeIO creates pipe pairs to be used with runc
func NewPipeIO(uid, gid int, opts ...IOOpt) (i IO, err error) {
	option := defaultIOOption()
	for _, o := range opts {
		o(option)
	}
	var (
		pipes                 []*pipe
		stdin, stdout, stderr *pipe
	)
	// cleanup in case of an error
	defer func() {
		if err != nil {
			for _, p := range pipes {
				p.Close()
			}
		}
	}()
	if option.OpenStdin {
		if stdin, err = newPipe(); err != nil {
			return nil, err
		}
		pipes = append(pipes, stdin)
		if err = unix.Fchown(int(stdin.r.Fd()), uid, gid); err != nil {
			return nil, errors.Wrap(err, "failed to chown stdin")
		}
	}
	if option.OpenStdout {
		if stdout, err = newPipe(); err != nil {
			return nil, err
		}
		pipes = append(pipes, stdout)
		if err = unix.Fchown(int(stdout.w.Fd()), uid, gid); err != nil {
			return nil, errors.Wrap(err, "failed to chown stdout")
		}
	}
	if option.OpenStderr {
		if stderr, err = newPipe(); err != nil {
			return nil, err
		}
		pipes = append(pipes, stderr)
		if err = unix.Fchown(int(stderr.w.Fd()), uid, gid); err != nil {
			return nil, errors.Wrap(err, "failed to chown stderr")
		}
	}
	return &pipeIO{
		in:  stdin,
		out: stdout,
		err: stderr,
	}, nil
}
