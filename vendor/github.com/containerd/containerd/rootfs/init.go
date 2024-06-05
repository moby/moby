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

package rootfs

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/log"
	digest "github.com/opencontainers/go-digest"
)

var (
	initializers = map[string]initializerFunc{}
)

type initializerFunc func(string) error

// Mounter handles mount and unmount
type Mounter interface {
	Mount(target string, mounts ...mount.Mount) error
	Unmount(target string) error
}

// InitRootFS initializes the snapshot for use as a rootfs
func InitRootFS(ctx context.Context, name string, parent digest.Digest, readonly bool, snapshotter snapshots.Snapshotter, mounter Mounter) ([]mount.Mount, error) {
	_, err := snapshotter.Stat(ctx, name)
	if err == nil {
		return nil, errors.New("rootfs already exists")
	}
	// TODO: ensure not exist error once added to snapshot package

	parentS := parent.String()

	initName := defaultInitializer
	initFn := initializers[initName]
	if initFn != nil {
		parentS, err = createInitLayer(ctx, parentS, initName, initFn, snapshotter, mounter)
		if err != nil {
			return nil, err
		}
	}

	if readonly {
		return snapshotter.View(ctx, name, parentS)
	}

	return snapshotter.Prepare(ctx, name, parentS)
}

func createInitLayer(ctx context.Context, parent, initName string, initFn func(string) error, snapshotter snapshots.Snapshotter, mounter Mounter) (_ string, retErr error) {
	initS := fmt.Sprintf("%s %s", parent, initName)
	if _, err := snapshotter.Stat(ctx, initS); err == nil {
		return initS, nil
	}
	// TODO: ensure not exist error once added to snapshot package

	// Create tempdir
	td, err := os.MkdirTemp(os.Getenv("XDG_RUNTIME_DIR"), "create-init-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(td)

	mounts, err := snapshotter.Prepare(ctx, td, parent)
	if err != nil {
		return "", err
	}

	defer func() {
		if retErr != nil {
			if rerr := snapshotter.Remove(ctx, td); rerr != nil {
				log.G(ctx).Errorf("Failed to remove snapshot %s: %v", td, rerr)
			}
		}
	}()

	if err = mounter.Mount(td, mounts...); err != nil {
		return "", err
	}

	if err = initFn(td); err != nil {
		if merr := mounter.Unmount(td); merr != nil {
			log.G(ctx).Errorf("Failed to unmount %s: %v", td, merr)
		}
		return "", err
	}

	if err = mounter.Unmount(td); err != nil {
		return "", err
	}

	if err := snapshotter.Commit(ctx, initS, td); err != nil {
		return "", err
	}

	return initS, nil
}
