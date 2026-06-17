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

package rdt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

type groupAnnotations = map[string]string

// Functions for creating and removing resctrl groups from the filesystem. These are
// configurable because of unit tests.
var groupCreateFunc func(string, os.FileMode) error = os.Mkdir
var groupRemoveFunc func(string) error = os.Remove

// CtrlGroup defines the interface of one goresctrl managed RDT class. It maps
// to one CTRL group directory in the goresctrl pseudo-filesystem.
type CtrlGroup interface {
	ResctrlGroup

	// CreateMonGroup creates a new monitoring group under this CtrlGroup.
	CreateMonGroup(name string, annotations map[string]string) (MonGroup, error)

	// DeleteMonGroup deletes a monitoring group from this CtrlGroup.
	DeleteMonGroup(name string) error

	// DeleteMonGroups deletes all monitoring groups from this CtrlGroup.
	DeleteMonGroups() error

	// GetMonGroup returns a specific monitoring group under this CtrlGroup.
	GetMonGroup(name string) (MonGroup, bool)

	// GetMonGroups returns all monitoring groups under this CtrlGroup.
	GetMonGroups() []MonGroup
}

// ResctrlGroup is the generic interface for resctrl CTRL and MON groups. It
// maps to one CTRL or MON group directory in the goresctrl pseudo-filesystem.
type ResctrlGroup interface {
	// Name returns the name of the group.
	Name() string

	// GetPids returns the process ids assigned to the group.
	GetPids() ([]string, error)

	// AddPids assigns the given process ids to the group.
	AddPids(pids ...string) error

	// GetMonData retrieves the monitoring data of the group.
	GetMonData() MonData

	// GetAnnotations returns the annotations associated with the group
	GetAnnotations() map[string]string

	// Private methods for internal use
	dirName() string
}

// MonGroup represents the interface to a RDT monitoring group. It maps to one
// MON group in the goresctrl filesystem.
type MonGroup interface {
	ResctrlGroup

	// Parent returns the CtrlGroup under which the monitoring group exists.
	Parent() CtrlGroup
}

// MonData contains monitoring stats of one monitoring group.
type MonData struct {
	L3 MonL3Data
}

// MonL3Data contains L3 monitoring stats of one monitoring group.
type MonL3Data map[uint64]MonLeafData

// MonLeafData represents the raw numerical stats from one RDT monitor data leaf.
type MonLeafData map[string]uint64

// MonResource is the type of RDT monitoring resource.
type MonResource string

const (
	// MonResourceL3 is the RDT L3 cache monitor resource.
	MonResourceL3 MonResource = "l3"
)

type ctrlGroup struct {
	resctrlGroup

	monPrefix string
}

type monGroup struct {
	resctrlGroup
}

type resctrlGroup struct {
	prefix string
	name   string
	parent *ctrlGroup // parent for MON groups
}

func newCtrlGroup(prefix, monPrefix, name string) (*ctrlGroup, error) {
	cg := &ctrlGroup{
		resctrlGroup: resctrlGroup{prefix: prefix, name: name},
		monPrefix:    monPrefix,
	}

	if err := groupCreateFunc(cg.path(""), 0755); err != nil && !os.IsExist(err) {
		return nil, err
	}

	return cg, nil
}

func (c *ctrlGroup) CreateMonGroup(name string, annotations map[string]string) (MonGroup, error) {
	log.Debug("adding monitoring group", "class", c.Name(), "name", name)
	mg, err := newMonGroup(c.monPrefix, name, c)
	if err != nil {
		return nil, fmt.Errorf("failed to create new monitoring group %q: %v", name, err)
	}

	// Store annotations in the reqistry
	rdt.setAnnotations(&mg.resctrlGroup, annotations)

	return mg, err
}

func (c *ctrlGroup) DeleteMonGroup(name string) error {
	mg, err := c.monGroupFromResctrlFs(name)
	if err != nil {
		return err
	}

	// Drop annotations from the reqistry
	rdt.deleteAnnotations(&mg.resctrlGroup)

	log.Debug("deleting monitoring group", "class", c.Name(), "name", mg.Name(), "path", mg.relPath(""))
	if err := mg.rmdir(); err != nil {
		return fmt.Errorf("failed to remove monitoring group %q (%s): %v", name, mg.relPath(""), err)
	}

	return nil
}

func (c *ctrlGroup) DeleteMonGroups() error {
	grps, err := c.monGroupsFromResctrlFs()
	if err != nil {
		return err
	}
	for _, mg := range grps {
		log.Debug("deleting monitoring group", "class", c.Name(), "name", mg.Name(), "path", mg.relPath(""))
		if err := mg.rmdir(); err != nil {
			return err
		}
	}
	return nil
}

func (c *ctrlGroup) GetMonGroup(name string) (MonGroup, bool) {
	mg, err := c.monGroupFromResctrlFs(name)
	if err != nil {
		log.Error("failed to get monitoring group from resctrl filesystem", "className", c.Name(), "error", err)
		return nil, false
	}
	return mg, true
}

func (c *ctrlGroup) GetMonGroups() []MonGroup {
	grps, err := c.monGroupsFromResctrlFs()
	if err != nil {
		log.Error("failed to get monitoring groups from resctrl filesystem", "className", c.Name(), "error", err)
		return nil
	}
	ret := make([]MonGroup, len(grps))
	for i, mg := range grps {
		ret[i] = mg
	}
	return ret
}

func (c *ctrlGroup) configure(name string, class *classConfig,
	partition *partitionConfig, options Options) error {
	schemata := ""

	// Handle cache allocation
	for _, lvl := range []cacheLevel{L2, L3} {
		switch {
		case info.cat[lvl].unified.Supported():
			schema, err := class.CATSchema[lvl].toStr(catSchemaTypeUnified, partition.CAT[lvl])
			if err != nil {
				return err
			}
			schemata += schema
		case info.cat[lvl].data.Supported() || info.cat[lvl].code.Supported():
			schema, err := class.CATSchema[lvl].toStr(catSchemaTypeCode, partition.CAT[lvl])
			if err != nil {
				return err
			}
			schemata += schema

			schema, err = class.CATSchema[lvl].toStr(catSchemaTypeData, partition.CAT[lvl])
			if err != nil {
				return err
			}
			schemata += schema
		default:
			if class.CATSchema[lvl].Alloc != nil && !options.cat(lvl).Optional {
				return fmt.Errorf("%s cache allocation for %q specified in configuration but not supported by system", lvl, name)
			}
		}
	}

	// Handle memory bandwidth allocation
	switch {
	case info.mb.Supported():
		schemata += class.MBSchema.toStr(partition.MB)
	default:
		if class.MBSchema != nil && !options.MB.Optional {
			return fmt.Errorf("memory bandwidth allocation for %q specified in configuration but not supported by system", name)
		}
	}

	if len(schemata) > 0 {
		log.Debug("writing schemata", "schemata", schemata, "path", c.path(""))
		if err := rdt.writeRdtFile(c.relPath("schemata"), []byte(schemata)); err != nil {
			return err
		}
	} else {
		log.Debug("empty schemata")
	}

	return nil
}

func (c *ctrlGroup) monGroupsFromResctrlFs() ([]*monGroup, error) {
	names, err := resctrlGroupsFromFs(c.monPrefix, c.path("mon_groups"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	grps := make([]*monGroup, 0, len(names))
	for _, name := range names {
		name = name[len(c.monPrefix):]
		mg, err := newMonGroup(c.monPrefix, name, c)
		if err != nil {
			return nil, err
		}
		grps = append(grps, mg)
	}

	sort.Slice(grps, func(i, j int) bool { return grps[i].Name() < grps[j].Name() })

	return grps, nil
}

func (c *ctrlGroup) monGroupFromResctrlFs(name string) (*monGroup, error) {
	path := filepath.Join(c.path("mon_groups"), c.monPrefix+name)
	if !isResctrlGroup(path) {
		return nil, fmt.Errorf("resctrl group not found for monitoring group %q (%q)", name, path)
	}

	mg, err := newMonGroup(c.monPrefix, name, c)
	if err != nil {
		return nil, err
	}

	return mg, nil
}

// Remove empty monitoring groups
func (c *ctrlGroup) pruneMonGroups() error {
	grps, err := c.monGroupsFromResctrlFs()
	if err != nil {
		return err
	}
	for _, mg := range grps {
		pids, err := mg.GetPids()
		if err != nil {
			return fmt.Errorf("failed to get pids for monitoring group %q: %v", mg.relPath(""), err)
		}
		if len(pids) == 0 {
			if err := mg.rmdir(); err != nil {
				return fmt.Errorf("failed to remove monitoring group %q: %v", mg.relPath(""), err)
			}
		}
	}
	return nil
}

func (r *resctrlGroup) Name() string {
	return r.name
}

func (r *resctrlGroup) GetPids() ([]string, error) {
	data, err := rdt.readRdtFile(r.relPath("tasks"))
	if err != nil {
		return []string{}, err
	}
	split := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(split[0]) > 0 {
		return split, nil
	}
	return []string{}, nil
}

func (r *resctrlGroup) AddPids(pids ...string) (err error) {
	f, err := os.OpenFile(r.path("tasks"), os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	for _, pid := range pids {
		if _, err := f.WriteString(pid + "\n"); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				log.Debug("no task", "pid", pid)
			} else {
				return fmt.Errorf("failed to assign processes %v to class %q: %v", pids, r.name, rdt.cmdError(err))
			}
		}
	}
	return
}

func (r *resctrlGroup) GetMonData() MonData {
	m := MonData{}

	if info.l3mon.Supported() {
		l3, err := r.getMonL3Data()
		if err != nil {
			log.Error("failed to retrieve L3 monitoring data", "error", err)
		} else {
			m.L3 = l3
		}
	}

	return m
}

func (r *resctrlGroup) GetAnnotations() map[string]string {
	return rdt.getAnnotations(r)
}

func (r *resctrlGroup) getMonL3Data() (MonL3Data, error) {
	files, err := os.ReadDir(r.path("mon_data"))
	if err != nil {
		return nil, err
	}

	m := MonL3Data{}
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, "mon_L3_") {
			// Parse cache id from the dirname
			id, err := strconv.ParseUint(strings.TrimPrefix(name, "mon_L3_"), 10, 32)
			if err != nil {
				// Just log an error and continue, we try to retrieve as much info as possible
				log.Error("failed to parse L3 monitor data directory name", "fileName", name, "error", err)
				continue
			}

			data, err := r.getMonLeafData(filepath.Join("mon_data", name))
			if err != nil {
				log.Error("failed to read monitor data", "error", err)
				continue
			}

			m[id] = data
		}
	}

	return m, nil
}

func (r *resctrlGroup) getMonLeafData(path string) (MonLeafData, error) {
	files, err := os.ReadDir(r.path(path))
	if err != nil {
		return nil, err
	}

	m := make(MonLeafData, len(files))

	for _, file := range files {
		name := file.Name()

		// We expect that all the files in the dir are regular files
		val, err := readFileUint64(r.path(path, name))
		if err != nil {
			// Just log an error and continue, we want to retrieve as much info as possible
			log.Error("failed to read data file", "error", err)
			continue
		}

		m[name] = val
	}
	return m, nil
}

func (r *resctrlGroup) relPath(elem ...string) string {
	if r.parent == nil {
		return filepath.Join(append([]string{r.dirName()}, elem...)...)
	}
	// Parent is only intended for MON groups - non-root CTRL groups are considered
	// as peers to the root CTRL group (as they are in HW) and do not have a parent
	return r.parent.relPath(append([]string{"mon_groups", r.dirName()}, elem...)...)
}

func (r *resctrlGroup) path(elem ...string) string {
	return filepath.Join(info.resctrlPath, r.relPath(elem...))
}

func (r *resctrlGroup) rmdir() error {
	return groupRemoveFunc(r.path(""))
}

func (r *resctrlGroup) dirName() string {
	if r.name == RootClassName {
		return ""
	}
	return r.prefix + r.name
}

func newMonGroup(prefix string, name string, parent *ctrlGroup) (*monGroup, error) {
	mg := &monGroup{
		resctrlGroup: resctrlGroup{prefix: prefix, name: name, parent: parent},
	}

	if err := groupCreateFunc(mg.path(""), 0755); err != nil && !os.IsExist(err) {
		return nil, err
	}

	return mg, nil
}

func (m *monGroup) Parent() CtrlGroup {
	return m.parent
}
