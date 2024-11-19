//go:build gofuzz

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

package local

import (
	"bufio"
	"bytes"
	"context"
	_ "crypto/sha256"
	"io"
	"testing"

	"github.com/opencontainers/go-digest"

	"github.com/containerd/containerd/v2/core/content"
)

func FuzzContentStoreWriter(data []byte) int {
	t := &testing.T{}
	ctx := context.Background()
	ctx, _, cs, cleanup := contentStoreEnv(t)
	defer cleanup()

	cw, err := cs.Writer(ctx, content.WithRef("myref"))
	if err != nil {
		return 0
	}
	if err := cw.Close(); err != nil {
		return 0
	}

	// reopen, so we can test things
	cw, err = cs.Writer(ctx, content.WithRef("myref"))
	if err != nil {
		return 0
	}

	err = checkCopyFuzz(int64(len(data)), cw, bufio.NewReader(io.NopCloser(bytes.NewReader(data))))
	if err != nil {
		return 0
	}
	expected := digest.FromBytes(data)

	if err = cw.Commit(ctx, int64(len(data)), expected); err != nil {
		return 0
	}
	return 1
}

func checkCopyFuzz(size int64, dst io.Writer, src io.Reader) error {
	nn, err := io.Copy(dst, src)
	if err != nil {
		return err
	}

	if nn != size {
		return err
	}
	return nil
}
