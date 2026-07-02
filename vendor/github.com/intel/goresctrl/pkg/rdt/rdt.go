/*
Copyright 2019 Intel Corporation

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

// Package rdt implements an API for managing Intel® RDT technologies via the
// resctrl pseudo-filesystem of the Linux kernel. It provides flexible
// configuration with a hierarchical approach for easy management of exclusive
// cache allocations.
//
// Goresctrl supports all available RDT technologies, i.e. L2 and L3 Cache
// Allocation (CAT) with Code and Data Prioritization (CDP) and Memory
// Bandwidth Allocation (MBA) plus Cache Monitoring (CMT) and Memory Bandwidth
// Monitoring (MBM).
//
// Basic usage example:
//
//	rdt.SetLogger(slog.Default().WithGroup("rdt"))
//
//	if err := rdt.Initialize(""); err != nil {
//		return fmt.Errorf("RDT not supported: %v", err)
//	}
//
//	if err := rdt.SetConfigFromFile("/path/to/rdt.conf.yaml", false); err != nil {
//		return fmt.Errorf("RDT configuration failed: %v", err)
//	}
//
//	if cls, ok := rdt.GetClass("my-class"); ok {
//	   //  Set PIDs 12345 and 12346 to class "my-class"
//		if err := cls.AddPids("12345", "12346"); err != nil {
//			return fmt.Errorf("failed to add PIDs to RDT class: %v", err)
//		}
//	}
package rdt

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/intel/goresctrl/pkg/utils"
)

const (
	// RootClassName is the name we use in our config for the special class
	// that configures the "root" resctrl group of the system
	RootClassName = "system/default"
	// RootClassAlias is an alternative name for the root class
	RootClassAlias = ""
)

type control struct {
	*slog.Logger

	resctrlGroupPrefix string
	conf               config
	rawConf            Config

	// Registry of ctrl/mon group annotations used for custom prometheus metrics labels.
	// We keep them here so that after Initialize() the annotations are cleared.
	annotations map[string]groupAnnotations
}

var log *slog.Logger = slog.Default()

var info *resctrlInfo

var rdt *control

// SetLogger sets the logger instance to be used by the package. This function
// may be called even before Initialize().
func SetLogger(l *slog.Logger) {
	log = l
	if rdt != nil {
		rdt.setLogger(l)
	}
}

// Initialize detects RDT from the system and initializes control interface of
// the package.
//
// NOTE: Monitoring group annotations (used for custom prometheus metrics
// labels) are lost on (re-)initialization.
func Initialize(resctrlGroupPrefix string) error {
	var err error

	info = nil
	rdt = nil

	// Get info from the resctrl filesystem
	info, err = getRdtInfo()
	if err != nil {
		return err
	}

	r := &control{
		Logger:             log,
		resctrlGroupPrefix: resctrlGroupPrefix,
		annotations:        make(map[string]groupAnnotations),
	}

	// Sanity check that we're able to read the groups from the resctrl filesystem
	if _, err = r.classesFromResctrlFs(); err != nil {
		return fmt.Errorf("failed to read classes from resctrl fs: %v", err)
	}

	rdt = r

	return nil
}

// SetResctrlGroupPrefix changes the prefix from the one that was set with
// Initialize().
func SetResctrlGroupPrefix(resctrlGroupPrefix string) error {
	if rdt != nil {
		rdt.resctrlGroupPrefix = resctrlGroupPrefix
		return nil
	}
	return fmt.Errorf("rdt not initialized")
}

// SetConfig  (re-)configures the resctrl filesystem according to the specified
// configuration.
func SetConfig(c *Config, force bool) error {
	if rdt != nil {
		return rdt.setConfig(c, force)
	}
	return fmt.Errorf("rdt not initialized")
}

// SetConfigFromData takes configuration as raw data, parses it and
// reconfigures the resctrl filesystem.
func SetConfigFromData(data []byte, force bool) error {
	cfg := &Config{}
	if err := yaml.UnmarshalStrict(data, cfg); err != nil {
		return fmt.Errorf("failed to parse configuration data: %v", err)
	}

	return SetConfig(cfg, force)
}

// SetConfigFromFile reads configuration from the filesystem and reconfigures
// the resctrl filesystem.
func SetConfigFromFile(path string, force bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	if err := SetConfigFromData(data, force); err != nil {
		return err
	}

	log.Info("configuration successfully loaded", "path", path)
	return nil
}

// GetClass returns one RDT class.
func GetClass(name string) (CtrlGroup, bool) {
	if rdt != nil {
		if cls, err := rdt.getClass(name); err != nil {
			log.Error("failed to get RDT class", "name", name, "error", err)
		} else {
			return cls, true
		}
	}
	return nil, false
}

// GetClasses returns all available RDT classes.
func GetClasses() []CtrlGroup {
	if rdt != nil {
		if classes, err := rdt.getClasses(); err != nil {
			log.Error("failed to get RDT classes", "error", err)
		} else {
			ret := make([]CtrlGroup, len(classes))
			for i, v := range classes {
				ret[i] = v
			}
			return ret
		}
	}
	return []CtrlGroup{}
}

// MonSupported returns true if RDT monitoring features are available.
func MonSupported() bool {
	if rdt != nil {
		return rdt.monSupported()
	}
	return false
}

// GetMonFeatures returns the available monitoring stats of each available
// monitoring technology.
func GetMonFeatures() map[MonResource][]string {
	if rdt != nil {
		return rdt.getMonFeatures()
	}
	return map[MonResource][]string{}
}

// IsQualifiedClassName returns true if given string qualifies as a class name
func IsQualifiedClassName(name string) bool {
	// Must be qualified as a file name
	return name == RootClassName || (len(name) < 4096 && name != "." && name != ".." && !strings.ContainsAny(name, "/\n"))
}

func (c *control) getClass(name string) (*ctrlGroup, error) {
	cls, err := c.classFromResctrlFs(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get class %q from resctrl fs: %w", name, err)
	}
	return cls, nil
}

func (c *control) getClasses() ([]*ctrlGroup, error) {
	classes, err := c.classesFromResctrlFs()
	if err != nil {
		return []*ctrlGroup{}, fmt.Errorf("failed to get classes from resctrl fs: %w", err)
	}

	ret := make([]*ctrlGroup, 0, len(classes))
	for _, v := range classes {
		ret = append(ret, v)
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].Name() < ret[j].Name() })

	return ret, nil
}

func (c *control) monSupported() bool {
	return info.l3mon.Supported()
}

func (c *control) getMonFeatures() map[MonResource][]string {
	ret := make(map[MonResource][]string)
	if info.l3mon.Supported() {
		ret[MonResourceL3] = append([]string{}, info.l3mon.monFeatures...)
	}

	return ret
}

func (c *control) setLogger(l *slog.Logger) {
	c.Logger = l
}

func (c *control) setConfig(newConfig *Config, force bool) error {
	c.Info("configuration update")

	conf, err := (*newConfig).resolve()
	if err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	err = c.configureResctrl(conf, force)
	if err != nil {
		return fmt.Errorf("resctrl configuration failed: %v", err)
	}

	c.conf = conf
	// TODO: we'd better create a deep copy
	c.rawConf = *newConfig
	c.Info("configuration finished")

	return nil
}

func (c *control) configureResctrl(conf config, force bool) error {
	// TODO: Think a better (more structured) way to log this
	log.Debug("applying resolved configuration:\n" + utils.DumpJSON(conf))

	// Remove stale resctrl groups
	classesFromFs, err := c.classesFromResctrlFs()
	if err != nil {
		return err
	}

	for _, cls := range classesFromFs {
		if _, ok := conf.Classes[cls.name]; !isRootClass(cls.name) && !ok {
			if !force {
				tasks, err := cls.GetPids()
				if err != nil {
					return fmt.Errorf("failed to get resctrl group tasks: %v", err)
				}
				if len(tasks) > 0 {
					return fmt.Errorf("refusing to remove non-empty resctrl group %q", cls.relPath(""))
				}
			}
			log.Debug("removing existing resctrl group", "name", cls.Name(), "path", cls.path(""))
			err = groupRemoveFunc(cls.path(""))
			if err != nil {
				return fmt.Errorf("failed to remove resctrl group %q: %v", cls.relPath(""), err)
			}
		}
	}

	// Try to apply given configuration
	for name, class := range conf.Classes {
		cg, ok := classesFromFs[name]
		if !ok {
			cg, err = newCtrlGroup(c.resctrlGroupPrefix, c.resctrlGroupPrefix, name)
			if err != nil {
				return err
			}
		}
		partition := conf.Partitions[class.Partition]
		if err := cg.configure(name, class, partition, conf.Options); err != nil {
			return err
		}
	}

	if err := c.pruneMonGroups(); err != nil {
		return err
	}

	return nil
}

func (c *control) classesFromResctrlFs() (map[string]*ctrlGroup, error) {
	return c.classesFromResctrlFsPrefix(c.resctrlGroupPrefix)
}

func (c *control) classesFromResctrlFsPrefix(prefix string) (map[string]*ctrlGroup, error) {
	names := []string{RootClassName}
	if g, err := resctrlGroupsFromFs(prefix, info.resctrlPath); err != nil {
		return nil, err
	} else {
		for _, n := range g {
			names = append(names, n[len(prefix):])
		}
	}

	classes := make(map[string]*ctrlGroup, len(names)+1)
	for _, name := range names {
		g, err := newCtrlGroup(prefix, c.resctrlGroupPrefix, name)
		if err != nil {
			return nil, err
		}
		classes[name] = g
	}

	return classes, nil
}

func (c *control) classFromResctrlFs(name string) (*ctrlGroup, error) {
	return c.classFromResctrlFsPrefix(c.resctrlGroupPrefix, name)
}

func (c *control) classFromResctrlFsPrefix(prefix, name string) (*ctrlGroup, error) {
	path := info.resctrlPath
	if !isRootClass(name) {
		path = filepath.Join(path, prefix+name)
	}
	if !isResctrlGroup(path) {
		return nil, fmt.Errorf("resctrl group not found for class %q (%q)", name, path)
	}

	g, err := newCtrlGroup(prefix, c.resctrlGroupPrefix, name)
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (c *control) pruneMonGroups() error {
	classes, err := c.getClasses()
	if err != nil {
		return err
	}
	for name, cls := range classes {
		if err := cls.pruneMonGroups(); err != nil {
			return fmt.Errorf("failed to prune stale monitoring groups of %q: %v", name, err)
		}
	}
	return nil
}

func (c *control) readRdtFile(rdtPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(info.resctrlPath, rdtPath))
}

func (c *control) writeRdtFile(rdtPath string, data []byte) error {
	if err := os.WriteFile(filepath.Join(info.resctrlPath, rdtPath), data, 0644); err != nil {
		return c.cmdError(err)
	}
	return nil
}

func (c *control) cmdError(origErr error) error {
	errData, readErr := c.readRdtFile(filepath.Join("info", "last_cmd_status"))
	if readErr != nil {
		return origErr
	}
	cmdStatus := strings.TrimSpace(string(errData))
	if len(cmdStatus) > 0 && cmdStatus != "ok" {
		return fmt.Errorf("%s", cmdStatus)
	}
	return origErr
}

func (c *control) setAnnotations(g *resctrlGroup, annotations map[string]string) {
	c.annotations[annotationKey(g)] = annotations
}

func (c *control) getAnnotations(g *resctrlGroup) map[string]string {
	return c.annotations[annotationKey(g)]
}
func (c *control) deleteAnnotations(g *resctrlGroup) {
	delete(c.annotations, annotationKey(g))
}

func annotationKey(g *resctrlGroup) string {
	// we use the dirnames to avoid ambiguity caused by the group prefix (if changed)
	if g.parent != nil {
		return g.parent.dirName() + "/" + g.dirName()
	}
	return g.dirName()
}

func resctrlGroupsFromFs(prefix string, path string) ([]string, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	grps := make([]string, 0, len(files))
	for _, file := range files {
		filename := file.Name()
		if strings.HasPrefix(filename, prefix) && isResctrlGroup(filepath.Join(path, filename)) {
			grps = append(grps, filename)
		}
	}
	return grps, nil
}

func isResctrlGroup(path string) bool {
	if s, err := os.Stat(filepath.Join(path, "tasks")); err == nil && !s.IsDir() {
		return true
	}
	return false
}

func isRootClass(name string) bool {
	return name == RootClassName || name == RootClassAlias
}

func unaliasClassName(name string) string {
	if isRootClass(name) {
		return RootClassName
	}
	return name
}
