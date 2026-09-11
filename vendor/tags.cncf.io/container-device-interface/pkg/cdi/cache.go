/*
   Copyright © 2021 The CDI Authors

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

package cdi

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	oci "github.com/opencontainers/runtime-spec/specs-go"
	cdi "tags.cncf.io/container-device-interface/specs-go"
)

// Option is an option to change some aspect of default CDI behavior.
type Option func(*Cache)

// Cache stores CDI Specs loaded from Spec directories.
type Cache struct {
	mu        sync.RWMutex
	specDirs  []string
	specs     map[string][]*Spec
	devices   map[string]*Device
	errors    map[string][]error
	dirErrors map[string]error

	autoRefresh bool
	watch       *watch
}

// Lock locks the cache.
//
// Deprecated: Cache locking is an implementation detail and should not be managed by callers.
func (c *Cache) Lock() {
	c.mu.Lock()
}

// Unlock unlocks the cache.
//
// Deprecated: Cache locking is an implementation detail and should not be managed by callers.
func (c *Cache) Unlock() {
	c.mu.Unlock()
}

// WithAutoRefresh returns an option to control automatic Cache refresh.
// By default, auto-refresh is enabled, the list of Spec directories are
// monitored and the Cache is automatically refreshed whenever a change
// is detected. This option can be used to disable this behavior when a
// manually refreshed mode is preferable.
func WithAutoRefresh(autoRefresh bool) Option {
	return func(c *Cache) {
		c.autoRefresh = autoRefresh
	}
}

// NewCache creates a new CDI Cache. The cache is populated from a set
// of CDI Spec directories. These can be specified using a WithSpecDirs
// option. The default set of directories is exposed in DefaultSpecDirs.
//
// Note:
//
//	The error returned by this function is always nil and it is only
//	returned to maintain API compatibility with consumers.
func NewCache(options ...Option) (*Cache, error) {
	return newCache(options...), nil
}

// newCache creates a CDI cache with the supplied options.
// This function allows testing without handling the nil error returned by the
// NewCache function.
func newCache(options ...Option) *Cache {
	specDirs := slices.Clone(DefaultSpecDirs)
	for i := range specDirs {
		specDirs[i] = filepath.Clean(specDirs[i])
	}
	c := &Cache{
		specDirs:    specDirs,
		autoRefresh: true,
		watch:       &watch{},
	}
	c.configure(options...)
	return c
}

// Configure applies options to the Cache. Updates and refreshes the
// Cache if options have changed.
func (c *Cache) Configure(options ...Option) error {
	if len(options) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.configure(options...)

	return nil
}

// Configure the Cache. Start/stop CDI Spec directory watch, refresh
// the Cache if necessary.
func (c *Cache) configure(options ...Option) {
	c.watch.stop()
	c.dirErrors = nil

	for _, o := range options {
		o(c)
	}

	if c.autoRefresh {
		c.watch.start(c)
	}
	c.refresh()
}

// Refresh rescans the CDI Spec directories and refreshes the Cache.
// In manual refresh mode the cache is always refreshed. In auto-
// refresh mode the cache is only refreshed if it is out of date.
func (c *Cache) Refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// force a refresh in manual mode
	if !c.autoRefresh {
		c.refresh()
	} else {
		c.refreshIfRequired()
	}

	// collect and return cached errors.
	var errs []error
	for _, specErrs := range c.errors {
		errs = append(errs, errors.Join(specErrs...))
	}
	return errors.Join(errs...)
}

// Refresh the Cache by rescanning CDI Spec directories and files.
func (c *Cache) refresh() {
	var (
		specs      = map[string][]*Spec{}
		devices    = map[string]*Device{}
		conflicts  = map[string]struct{}{}
		specErrors = map[string][]error{}
	)

	// collect errors per spec file path and once globally
	collectError := func(err error, paths ...string) {
		for _, path := range paths {
			specErrors[path] = append(specErrors[path], err)
		}
	}
	// resolve conflicts based on device Spec priority (order of precedence)
	resolveConflict := func(name string, dev *Device, old *Device) bool {
		devSpec, oldSpec := dev.GetSpec(), old.GetSpec()
		devPrio, oldPrio := devSpec.GetPriority(), oldSpec.GetPriority()
		switch {
		case devPrio > oldPrio:
			return false
		case devPrio == oldPrio:
			devPath, oldPath := devSpec.GetPath(), oldSpec.GetPath()
			collectError(fmt.Errorf("conflicting device %q (specs %q, %q)",
				name, devPath, oldPath), devPath, oldPath)
			conflicts[name] = struct{}{}
		}
		return true
	}

	_ = scanSpecDirs(c.specDirs, func(path string, priority int, spec *Spec, err error) error {
		if err != nil {
			collectError(fmt.Errorf("failed to load CDI Spec %w", err), path)
			return nil
		}

		vendor := spec.GetVendor()
		specs[vendor] = append(specs[vendor], spec)

		for _, dev := range spec.devices {
			qualified := dev.GetQualifiedName()
			other, ok := devices[qualified]
			if ok {
				if resolveConflict(qualified, dev, other) {
					continue
				}
			}
			devices[qualified] = dev
		}

		return nil
	})

	for conflict := range conflicts {
		delete(devices, conflict)
	}

	c.specs = specs
	c.devices = devices
	c.errors = specErrors
}

// refreshIfRequired triggers a refresh if necessary.
func (c *Cache) refreshIfRequired() {
	// We need to refresh if a missing Spec dir appears (added to watch) in auto-refresh mode.
	if c.autoRefresh && c.watch.update(c.dirErrors) {
		c.refresh()
	}
}

// InjectDevices injects the given qualified devices to an OCI Spec. It
// returns any unresolvable devices and an error if injection fails for
// any of the devices. Might trigger a cache refresh, in which case any
// errors encountered can be obtained using GetErrors().
func (c *Cache) InjectDevices(ociSpec *oci.Spec, devices ...string) ([]string, error) {
	if ociSpec == nil {
		return devices, fmt.Errorf("can't inject devices, nil OCI Spec")
	}

	c.mu.Lock()
	c.refreshIfRequired()
	cachedDevices := c.devices
	c.mu.Unlock()

	edits := &ContainerEdits{}
	specs := map[*Spec]struct{}{}

	var unresolved []string
	for _, device := range devices {
		d := cachedDevices[device]
		if d == nil {
			unresolved = append(unresolved, device)
			continue
		}
		if _, ok := specs[d.GetSpec()]; !ok {
			specs[d.GetSpec()] = struct{}{}
			edits.Append(d.GetSpec().edits())
		}
		edits.Append(d.edits())
	}

	if len(unresolved) > 0 {
		return unresolved, fmt.Errorf("unresolvable CDI devices %s",
			strings.Join(unresolved, ", "))
	}

	if err := edits.Apply(ociSpec); err != nil {
		return nil, fmt.Errorf("failed to inject devices: %w", err)
	}

	return nil, nil
}

// highestPrioritySpecDir returns the Spec directory with highest priority
// and its priority.
func (c *Cache) highestPrioritySpecDir() (string, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.specDirs) == 0 {
		return "", -1
	}

	prio := len(c.specDirs) - 1
	dir := c.specDirs[prio]

	return dir, prio
}

// WriteSpec writes a Spec file with the given content into the highest
// priority Spec directory. If name has a "json" or "yaml" extension it
// choses the encoding. Otherwise the default YAML encoding is used.
func (c *Cache) WriteSpec(raw *cdi.Spec, name string) error {
	specDir, prio := c.highestPrioritySpecDir()
	if specDir == "" {
		return errors.New("no Spec directories to write to")
	}

	path := filepath.Join(specDir, name)
	if ext := filepath.Ext(path); ext != ".json" && ext != ".yaml" {
		path += defaultSpecExt
	}

	spec, err := newSpec(raw, path, prio)
	if err != nil {
		return err
	}

	return spec.write(true)
}

// RemoveSpec removes a Spec with the given name from the highest
// priority Spec directory. This function can be used to remove a
// Spec previously written by WriteSpec(). If the file exists and
// its removal fails RemoveSpec returns an error.
func (c *Cache) RemoveSpec(name string) error {
	specDir, _ := c.highestPrioritySpecDir()
	if specDir == "" {
		return errors.New("no Spec directories to remove from")
	}

	path := filepath.Join(specDir, name)
	if ext := filepath.Ext(path); ext != ".json" && ext != ".yaml" {
		path += defaultSpecExt
	}

	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return err
}

// GetDevice returns the cached device for the given qualified name. Might trigger
// a cache refresh, in which case any errors encountered can be obtained using
// GetErrors().
func (c *Cache) GetDevice(device string) *Device {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.refreshIfRequired()

	return c.devices[device]
}

// ListDevices lists all cached devices by qualified name. Might trigger a cache
// refresh, in which case any errors encountered can be obtained using GetErrors().
func (c *Cache) ListDevices() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.refreshIfRequired()

	return slices.Sorted(maps.Keys(c.devices))
}

// ListVendors lists all vendors known to the cache. Might trigger a cache refresh,
// in which case any errors encountered can be obtained using GetErrors().
func (c *Cache) ListVendors() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.refreshIfRequired()

	return slices.Sorted(maps.Keys(c.specs))
}

// ListClasses lists all device classes known to the cache. Might trigger a cache
// refresh, in which case any errors encountered can be obtained using GetErrors().
func (c *Cache) ListClasses() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.refreshIfRequired()

	var classes []string
	seen := map[string]struct{}{}
	for _, specs := range c.specs {
		for _, spec := range specs {
			class := spec.GetClass()
			if _, ok := seen[class]; ok {
				continue
			}
			seen[class] = struct{}{}
			classes = append(classes, class)
		}
	}
	slices.Sort(classes)

	return classes
}

// GetVendorSpecs returns all specs for the given vendor. Might trigger a cache
// refresh, in which case any errors encountered can be obtained using GetErrors().
func (c *Cache) GetVendorSpecs(vendor string) []*Spec {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.refreshIfRequired()

	return c.specs[vendor]
}

// GetSpecErrors returns all errors encountered for the spec during the
// last cache refresh.
func (c *Cache) GetSpecErrors(spec *Spec) []error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return slices.Clone(c.errors[spec.GetPath()])
}

// GetErrors returns all errors encountered during the last
// cache refresh.
func (c *Cache) GetErrors() map[string][]error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	errsByPath := make(map[string][]error, len(c.errors)+len(c.dirErrors))
	for path, errs := range c.errors {
		errsByPath[path] = slices.Clone(errs)
	}
	for path, err := range c.dirErrors {
		errsByPath[path] = append(errsByPath[path], err)
	}

	return errsByPath
}

// GetSpecDirectories returns the CDI Spec directories currently in use.
func (c *Cache) GetSpecDirectories() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return slices.Clone(c.specDirs)
}

// GetSpecDirErrors returns any errors related to configured Spec directories.
func (c *Cache) GetSpecDirErrors() map[string]error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return maps.Clone(c.dirErrors)
}

// Our fsnotify helper wrapper.
type watch struct {
	watcher *fsnotify.Watcher
	tracked map[string]bool
}

// Start watching the Spec directories configured in the cache, recording
// watch errors and triggering cache refreshes as directories change.
func (w *watch) start(c *Cache) {
	c.dirErrors = make(map[string]error)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		for _, dir := range c.specDirs {
			c.dirErrors[dir] = fmt.Errorf("failed to create watcher: %w", err)
		}
		return
	}

	w.watcher = watcher
	w.tracked = make(map[string]bool, len(c.specDirs))
	for _, dir := range c.specDirs {
		if err := watcher.Add(dir); err != nil {
			c.dirErrors[dir] = fmt.Errorf("failed to monitor for changes: %w", err)
			w.tracked[dir] = false
			continue
		}
		w.tracked[dir] = true
	}

	go w.watch(watcher, c)
}

// Stop watching directories.
func (w *watch) stop() {
	if w.watcher == nil {
		return
	}

	_ = w.watcher.Close()
	w.watcher = nil
	w.tracked = nil
}

// Watch Spec directory changes, triggering a refresh if necessary.
func (w *watch) watch(fsw *fsnotify.Watcher, c *Cache) {
	// Watch for Spec file changes. Atomic writes may create a temporary file and
	// rename it into place. On Linux, fsnotify reports the destination of such a
	// rename as a Create event, so Create must be watched on all platforms.
	eventMask := fsnotify.Create | fsnotify.Rename | fsnotify.Remove | fsnotify.Write

	for {
		select {
		case event, ok := <-fsw.Events:
			if !ok {
				return
			}

			fsOp := event.Op & eventMask
			if fsOp == 0 {
				continue
			}

			c.mu.Lock()
			_, isTracked := w.tracked[event.Name]

			// Ignore changes unrelated to Spec files or configured Spec directories.
			if ext := filepath.Ext(event.Name); ext != ".json" && ext != ".yaml" && !isTracked {
				c.mu.Unlock()
				continue
			}
			if fsOp&fsnotify.Remove != 0 && isTracked {
				w.markRemoved(c.dirErrors, event.Name)
			}
			w.update(c.dirErrors)
			c.refresh()
			c.mu.Unlock()

		case _, ok := <-fsw.Errors:
			if !ok {
				return
			}
		}
	}
}

// markRemoved marks a configured Spec directory as not currently watched so
// its watch can be restored if the directory is recreated.
func (w *watch) markRemoved(dirErrors map[string]error, dir string) {
	w.tracked[dir] = false
	dirErrors[dir] = errors.New("directory removed")
}

// update restores watches for configured directories that are not currently
// being watched. It reports whether any watches were restored.
func (w *watch) update(dirErrors map[string]error) bool {
	// If we failed to create an fsnotify.Watcher we have a nil watcher here
	// (but with autoRefresh left on). One known case when this can happen is
	// if we have too many open files. In that case we always return true and
	// force a refresh.
	if w.watcher == nil {
		return true
	}

	var update bool
	for dir, watched := range w.tracked {
		if watched {
			continue
		}

		if err := w.watcher.Add(dir); err != nil {
			dirErrors[dir] = fmt.Errorf("failed to monitor for changes: %w", err)
			continue
		}

		w.tracked[dir] = true
		delete(dirErrors, dir)
		update = true
	}

	return update
}
