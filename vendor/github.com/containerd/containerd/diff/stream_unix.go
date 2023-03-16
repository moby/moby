//go:build !windows

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

package diff

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/containerd/containerd/protobuf"
	"github.com/containerd/containerd/protobuf/proto"
	"github.com/containerd/typeurl/v2"
	exec "golang.org/x/sys/execabs"
)

// NewBinaryProcessor returns a binary processor for use with processing content streams
func NewBinaryProcessor(ctx context.Context, imt, rmt string, stream StreamProcessor, name string, args, env []string, payload typeurl.Any) (StreamProcessor, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, env...)

	var payloadC io.Closer
	if payload != nil {
		pb := protobuf.FromAny(payload)
		data, err := proto.Marshal(pb)
		if err != nil {
			return nil, err
		}
		r, w, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		go func() {
			io.Copy(w, bytes.NewReader(data))
			w.Close()
		}()

		cmd.ExtraFiles = append(cmd.ExtraFiles, r)
		payloadC = r
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", mediaTypeEnvVar, imt))
	var (
		stdin  io.Reader
		closer func() error
		err    error
	)
	if f, ok := stream.(RawProcessor); ok {
		stdin = f.File()
		closer = f.File().Close
	} else {
		stdin = stream
	}
	cmd.Stdin = stdin
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = w

	stderr := bytes.NewBuffer(nil)
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &binaryProcessor{
		cmd:    cmd,
		r:      r,
		mt:     rmt,
		stderr: stderr,
		done:   make(chan struct{}),
	}
	go p.wait()

	// close after start and dup
	w.Close()
	if closer != nil {
		closer()
	}
	if payloadC != nil {
		payloadC.Close()
	}
	return p, nil
}

type binaryProcessor struct {
	cmd    *exec.Cmd
	r      *os.File
	mt     string
	stderr *bytes.Buffer

	mu  sync.Mutex
	err error

	// There is a race condition between waiting on c.cmd.Wait() and setting c.err within
	// c.wait(), and reading that value from c.Err().
	// Use done to wait for the returned error to be captured and set.
	done chan struct{}
}

func (c *binaryProcessor) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *binaryProcessor) wait() {
	if err := c.cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			c.mu.Lock()
			c.err = errors.New(c.stderr.String())
			c.mu.Unlock()
		}
	}
	close(c.done)
}

func (c *binaryProcessor) Wait(ctx context.Context) error {
	select {
	case <-c.done:
		return c.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *binaryProcessor) File() *os.File {
	return c.r
}

func (c *binaryProcessor) MediaType() string {
	return c.mt
}

func (c *binaryProcessor) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

func (c *binaryProcessor) Close() error {
	err := c.r.Close()
	if kerr := c.cmd.Process.Kill(); err == nil {
		err = kerr
	}
	return err
}
