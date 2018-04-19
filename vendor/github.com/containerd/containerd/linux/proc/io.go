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

package proc

import (
	"context"
	"fmt"
	"io"
	"sync"
	"syscall"

	"github.com/containerd/fifo"
	runc "github.com/containerd/go-runc"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 32<<10)
		return &buffer
	},
}

func copyPipes(ctx context.Context, rio runc.IO, stdin, stdout, stderr string, wg, cwg *sync.WaitGroup) error {
	for name, dest := range map[string]func(wc io.WriteCloser, rc io.Closer){
		stdout: func(wc io.WriteCloser, rc io.Closer) {
			wg.Add(1)
			cwg.Add(1)
			go func() {
				cwg.Done()
				p := bufPool.Get().(*[]byte)
				defer bufPool.Put(p)
				io.CopyBuffer(wc, rio.Stdout(), *p)
				wg.Done()
				wc.Close()
				rc.Close()
			}()
		},
		stderr: func(wc io.WriteCloser, rc io.Closer) {
			wg.Add(1)
			cwg.Add(1)
			go func() {
				cwg.Done()
				p := bufPool.Get().(*[]byte)
				defer bufPool.Put(p)

				io.CopyBuffer(wc, rio.Stderr(), *p)
				wg.Done()
				wc.Close()
				rc.Close()
			}()
		},
	} {
		fw, err := fifo.OpenFifo(ctx, name, syscall.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("containerd-shim: opening %s failed: %s", name, err)
		}
		fr, err := fifo.OpenFifo(ctx, name, syscall.O_RDONLY, 0)
		if err != nil {
			return fmt.Errorf("containerd-shim: opening %s failed: %s", name, err)
		}
		dest(fw, fr)
	}
	if stdin == "" {
		rio.Stdin().Close()
		return nil
	}
	f, err := fifo.OpenFifo(ctx, stdin, syscall.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("containerd-shim: opening %s failed: %s", stdin, err)
	}
	cwg.Add(1)
	go func() {
		cwg.Done()
		p := bufPool.Get().(*[]byte)
		defer bufPool.Put(p)

		io.CopyBuffer(rio.Stdin(), f, *p)
		rio.Stdin().Close()
		f.Close()
	}()
	return nil
}
