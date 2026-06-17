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

package v2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"

	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/timeout"
	"golang.org/x/sync/errgroup"
)

// LoadExistingShims loads existing shims from the path specified by stateDir
// rootDir is for cleaning up the unused paths of removed shims.
func (m *ShimManager) LoadExistingShims(ctx context.Context, stateDir string, rootDir string) error {
	nsDirs, err := os.ReadDir(stateDir)
	if err != nil {
		return err
	}
	for _, nsd := range nsDirs {
		if !nsd.IsDir() {
			continue
		}
		ns := nsd.Name()
		// skip hidden directories
		if len(ns) > 0 && ns[0] == '.' {
			continue
		}
		log.G(ctx).WithField("namespace", ns).Debug("loading tasks in namespace")
		if err := m.loadShims(namespaces.WithNamespace(ctx, ns), stateDir); err != nil {
			log.G(ctx).WithField("namespace", ns).WithError(err).Error("loading tasks in namespace")
			continue
		}
		if err := m.cleanupWorkDirs(namespaces.WithNamespace(ctx, ns), rootDir); err != nil {
			log.G(ctx).WithField("namespace", ns).WithError(err).Error("cleanup working directory in namespace")
			continue
		}
	}
	return nil
}

func (m *ShimManager) loadShims(ctx context.Context, stateDir string) error {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("namespace", ns))

	shimDirs, err := os.ReadDir(filepath.Join(stateDir, ns))
	if err != nil {
		return err
	}
	eg, ctx2 := errgroup.WithContext(ctx)
	eg.SetLimit(runtime.GOMAXPROCS(0))
	var errLoad error
	for _, sd := range shimDirs {
		if !sd.IsDir() {
			continue
		}
		id := sd.Name()
		// skip hidden directories
		if len(id) > 0 && id[0] == '.' {
			continue
		}
		bundle, err := LoadBundle(ctx, stateDir, id)
		if err != nil {
			errLoad = err
			// fine to return error here, it is a programmer error if the context
			// does not have a namespace
			break
		}
		eg.Go(func() error {
			// fast path
			f, err := os.Open(bundle.Path)
			if err != nil {
				bundle.Delete()
				log.G(ctx2).WithError(err).Errorf("fast path read bundle path for %s", bundle.Path)
				return nil
			}

			bf, err := f.Readdirnames(-1)
			f.Close()
			if err != nil {
				bundle.Delete()
				log.G(ctx2).WithError(err).Errorf("fast path read bundle path for %s", bundle.Path)
				return nil
			}
			if len(bf) == 0 {
				bundle.Delete()
				return nil
			}
			if err := m.loadShim(ctx2, bundle); err != nil {
				log.G(ctx2).WithError(err).Errorf("failed to load shim %s", bundle.Path)
				bundle.Delete()
				return nil
			}
			return nil
		})
	}
	_ = eg.Wait()
	return errLoad
}

func (m *ShimManager) loadShim(ctx context.Context, bundle *Bundle) error {
	var (
		runtime string
		id      = bundle.ID
	)

	// If we're on 1.6+ and specified custom path to the runtime binary, path will be saved in 'shim-binary-path' file.
	if data, err := os.ReadFile(filepath.Join(bundle.Path, "shim-binary-path")); err == nil {
		runtime = string(data)
	} else if err != nil && !os.IsNotExist(err) {
		log.G(ctx).WithError(err).Error("failed to read `runtime` path from bundle")
	}

	// Query runtime name from metadata store
	if runtime == "" {
		container, err := m.containers.Get(ctx, id)
		if err != nil {
			log.G(ctx).WithError(err).Errorf("loading container %s", id)
			if err := mount.UnmountRecursive(filepath.Join(bundle.Path, "rootfs"), 0); err != nil {
				log.G(ctx).WithError(err).Errorf("failed to unmount of rootfs %s", id)
			}
			return err
		}
		runtime = container.Runtime.Name
	}

	runtime, err := m.resolveRuntimePath(runtime)
	if err != nil {
		bundle.Delete()

		return fmt.Errorf("failed to resolve runtime path: %w", err)
	}

	binaryCall := shimBinary(bundle,
		shimBinaryConfig{
			runtime:      runtime,
			address:      m.containerdAddress,
			ttrpcAddress: m.containerdTTRPCAddress,
			env:          m.env,
		})
	// TODO: It seems we can only call loadShim here if it is a sandbox shim?
	shim, err := loadShimTask(ctx, bundle, func() {
		log.G(ctx).WithField("id", id).Info("shim disconnected")

		cleanupAfterDeadShim(context.WithoutCancel(ctx), id, m.shims, m.events, binaryCall)
		// Remove self from the runtime task list.
		m.shims.Delete(ctx, id)
	})
	if err != nil {
		cleanupAfterDeadShim(ctx, id, m.shims, m.events, binaryCall)
		return fmt.Errorf("unable to load shim %q: %w", id, err)
	}

	// There are 3 possibilities for the loaded shim here:
	// 1. It could be a shim that is running a task.
	// 2. It could be a sandbox shim.
	// 3. Or it could be a shim that was created for running a task but
	// something happened (probably a containerd crash) and the task was never
	// created. This shim process should be cleaned up here. Look at
	// containerd/containerd#6860 for further details.

	_, sgetErr := m.sandboxStore.Get(ctx, id)
	pInfo, pidErr := shim.Pids(ctx)
	if sgetErr != nil && errors.Is(sgetErr, errdefs.ErrNotFound) && (len(pInfo) == 0 || errors.Is(pidErr, errdefs.ErrNotFound)) {
		log.G(ctx).WithField("id", id).Info("cleaning leaked shim process")
		// We are unable to get Pids from the shim and it's not a sandbox
		// shim. We should clean it up her.
		// No need to do anything for removeTask since we never added this shim.
		shim.delete(ctx, false, func(ctx context.Context, id string) {})
	} else {
		m.shims.Add(ctx, shim.ShimInstance)
	}
	return nil
}

func loadShimTask(ctx context.Context, bundle *Bundle, onClose func()) (_ *shimTask, retErr error) {
	shim, err := loadShim(ctx, bundle, onClose)
	if err != nil {
		return nil, err
	}
	// Check connectivity, TaskService is the only required service, so create a temp one to check connection.
	s, err := newShimTask(shim)
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.WithContext(ctx, loadTimeout)
	defer cancel()

	if _, err := s.PID(ctx); err != nil {
		if !errdefs.IsNotImplemented(err) {
			return nil, err
		}

		downgrader, ok := shim.(clientVersionDowngrader)
		if ok {
			if derr := downgrader.Downgrade(); derr == nil {
				log.G(ctx).WithError(err).WithField("id", shim.ID()).
					Warning("failed to call task.PID, downgrading client API version to try again")

				s, err = newShimTask(shim)
				if err != nil {
					return nil, fmt.Errorf("failed to create shim task after downgrading: %w", err)
				}
				_, err = s.PID(ctx)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (m *ShimManager) cleanupWorkDirs(ctx context.Context, rootDir string) error {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	f, err := os.Open(filepath.Join(rootDir, ns))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	dirs, err := f.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		// if the task was not loaded, cleanup and empty working directory
		// this can happen on a reboot where /run for the bundle state is cleaned up
		// but that persistent working dir is left
		if _, err := m.shims.Get(ctx, dir); err != nil {
			path := filepath.Join(rootDir, ns, dir)
			if err := os.RemoveAll(path); err != nil {
				log.G(ctx).WithError(err).Errorf("cleanup working dir %s", path)
			}
		}
	}
	return nil
}
