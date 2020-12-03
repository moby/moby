// +build !windows,!linux

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

package shim

import (
	"context"
	"io"
	"net/url"
	"os"
	"sync"
	"syscall"

	"github.com/containerd/console"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/fifo"
	"github.com/pkg/errors"
)

type unixPlatform struct {
}

func (p *unixPlatform) CopyConsole(ctx context.Context, console console.Console, id, stdin, stdout, stderr string, wg *sync.WaitGroup) (cons console.Console, retErr error) {
	var cwg sync.WaitGroup
	if stdin != "" {
		in, err := fifo.OpenFifo(ctx, stdin, syscall.O_RDONLY, 0)
		if err != nil {
			return nil, err
		}
		cwg.Add(1)
		go func() {
			cwg.Done()
			p := bufPool.Get().(*[]byte)
			defer bufPool.Put(p)

			io.CopyBuffer(console, in, *p)
		}()
	}
	uri, err := url.Parse(stdout)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse stdout uri")
	}

	switch uri.Scheme {
	case "binary":
		ns, err := namespaces.NamespaceRequired(ctx)
		if err != nil {
			return nil, err
		}

		cmd := runtime.NewBinaryCmd(uri, id, ns)

		// In case of unexpected errors during logging binary start, close open pipes
		var filesToClose []*os.File

		defer func() {
			if retErr != nil {
				runtime.CloseFiles(filesToClose...)
			}
		}()

		// Create pipe to be used by logging binary for Stdout
		outR, outW, err := os.Pipe()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create stdout pipes")
		}
		filesToClose = append(filesToClose, outR)

		// Stderr is created for logging binary but unused when terminal is true
		serrR, _, err := os.Pipe()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create stderr pipes")
		}
		filesToClose = append(filesToClose, serrR)

		r, w, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		filesToClose = append(filesToClose, r)

		cmd.ExtraFiles = append(cmd.ExtraFiles, outR, serrR, w)

		wg.Add(1)
		cwg.Add(1)
		go func() {
			cwg.Done()
			io.Copy(outW, console)
			outW.Close()
			wg.Done()
		}()

		if err := cmd.Start(); err != nil {
			return nil, errors.Wrap(err, "failed to start logging binary process")
		}

		// Close our side of the pipe after start
		if err := w.Close(); err != nil {
			return nil, errors.Wrap(err, "failed to close write pipe after start")
		}

		// Wait for the logging binary to be ready
		b := make([]byte, 1)
		if _, err := r.Read(b); err != nil && err != io.EOF {
			return nil, errors.Wrap(err, "failed to read from logging binary")
		}
		cwg.Wait()

	default:
		outw, err := fifo.OpenFifo(ctx, stdout, syscall.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
		outr, err := fifo.OpenFifo(ctx, stdout, syscall.O_RDONLY, 0)
		if err != nil {
			return nil, err
		}
		wg.Add(1)
		cwg.Add(1)
		go func() {
			cwg.Done()
			p := bufPool.Get().(*[]byte)
			defer bufPool.Put(p)
			io.CopyBuffer(outw, console, *p)
			outw.Close()
			outr.Close()
			wg.Done()
		}()
		cwg.Wait()
	}
	return console, nil
}

func (p *unixPlatform) ShutdownConsole(ctx context.Context, cons console.Console) error {
	return nil
}

func (p *unixPlatform) Close() error {
	return nil
}

func (s *Service) initPlatform() error {
	s.platform = &unixPlatform{}
	return nil
}
