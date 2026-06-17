/*
Copyright 2019-2021 Intel Corporation

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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// resctrlInfo contains information about the RDT support in the system
type resctrlInfo struct {
	resctrlPath      string
	resctrlMountOpts map[string]struct{}
	numClosids       uint64
	cat              map[cacheLevel]catInfoAll
	l3mon            l3MonInfo
	mb               mbInfo
}

type cacheLevel string

const (
	L2 cacheLevel = "L2"
	L3 cacheLevel = "L3"
)

type catInfoAll struct {
	cacheIds []uint64
	unified  catInfo
	code     catInfo
	data     catInfo
}

type catInfo struct {
	numClosids    uint64
	cbmMask       bitmask
	minCbmBits    uint64
	shareableBits bitmask
}

type l3MonInfo struct {
	numRmids    uint64
	monFeatures []string
}

type mbInfo struct {
	cacheIds      []uint64
	numClosids    uint64
	bandwidthGran uint64
	delayLinear   uint64
	minBandwidth  uint64
	mbpsEnabled   bool // true if MBA_MBps is enabled
}

var mountInfoPath string = "/proc/mounts"

// getInfo is a helper method for a "unified API" for getting L3 information
func (i catInfoAll) getInfo() catInfo {
	switch {
	case i.code.Supported():
		return i.code
	case i.data.Supported():
		return i.data
	}
	return i.unified
}

func (i catInfoAll) cbmMask() bitmask {
	mask := i.getInfo().cbmMask
	if mask != 0 {
		return mask
	}
	return bitmask(^uint64(0))
}

func (i catInfoAll) minCbmBits() uint64 {
	return i.getInfo().minCbmBits
}

func getRdtInfo() (*resctrlInfo, error) {
	var err error
	info := &resctrlInfo{cat: make(map[cacheLevel]catInfoAll)}

	info.resctrlPath, info.resctrlMountOpts, err = getResctrlMountInfo()
	if err != nil {
		return info, fmt.Errorf("failed to detect resctrl mount point: %v", err)
	}
	log.Info("detected resctrl filesystem", "path", info.resctrlPath)

	// Check that RDT is available
	infopath := filepath.Join(info.resctrlPath, "info")
	if _, err := os.Stat(infopath); err != nil {
		return info, fmt.Errorf("failed to read RDT info from %q: %v", infopath, err)
	}

	// Check CAT feature available
	for _, cl := range []cacheLevel{L2, L3} {
		cat := catInfoAll{}
		catFeatures := map[string]*catInfo{
			"":     &cat.unified,
			"CODE": &cat.code,
			"DATA": &cat.data,
		}
		for suffix, i := range catFeatures {
			dir := string(cl) + suffix
			subpath := filepath.Join(infopath, dir)
			if _, err = os.Stat(subpath); err == nil {
				*i, err = getCatInfo(subpath)
				if err != nil {
					return info, fmt.Errorf("failed to get %s info from %q: %v", dir, subpath, err)
				}
				// Overall number of closids is the minimum across all cache levels/features
				if info.numClosids == 0 || i.numClosids < info.numClosids {
					info.numClosids = i.numClosids
				}
			}
		}
		if cat.getInfo().Supported() {
			cat.cacheIds, err = getCacheIds(info.resctrlPath, string(cl))
			if err != nil {
				return info, fmt.Errorf("failed to get %s CAT cache IDs: %v", cl, err)
			}
		}
		info.cat[cl] = cat
	}

	// Check MON features available
	subpath := filepath.Join(infopath, "L3_MON")
	if _, err = os.Stat(subpath); err == nil {
		info.l3mon, err = getL3MonInfo(subpath)
		if err != nil {
			return info, fmt.Errorf("failed to get L3_MON info from %q: %v", subpath, err)
		}
	}

	// Check MBA feature available
	subpath = filepath.Join(infopath, "MB")
	if _, err = os.Stat(subpath); err == nil {
		info.mb, err = getMBInfo(subpath)
		if err != nil {
			return info, fmt.Errorf("failed to get MBA info from %q: %v", subpath, err)
		}

		info.mb.cacheIds, err = getCacheIds(info.resctrlPath, "MB")
		if err != nil {
			return info, fmt.Errorf("failed to get MBA cache IDs: %v", err)
		}
		// Overall number of closids is the minimum across all cache levels/features
		if info.numClosids == 0 || info.mb.numClosids < info.numClosids {
			info.numClosids = info.mb.numClosids
		}
	}

	return info, nil
}

func getCatInfo(basepath string) (catInfo, error) {
	var err error
	info := catInfo{}

	info.cbmMask, err = readFileBitmask(filepath.Join(basepath, "cbm_mask"))
	if err != nil {
		return info, err
	}
	info.minCbmBits, err = readFileUint64(filepath.Join(basepath, "min_cbm_bits"))
	if err != nil {
		return info, err
	}
	info.shareableBits, err = readFileBitmask(filepath.Join(basepath, "shareable_bits"))
	if err != nil {
		return info, err
	}
	info.numClosids, err = readFileUint64(filepath.Join(basepath, "num_closids"))
	if err != nil {
		return info, err
	}

	return info, nil
}

// Supported returns true if L3 cache allocation has is supported and enabled in the system
func (i catInfo) Supported() bool {
	return i.cbmMask != 0
}

func getL3MonInfo(basepath string) (l3MonInfo, error) {
	var err error
	info := l3MonInfo{}

	info.numRmids, err = readFileUint64(filepath.Join(basepath, "num_rmids"))
	if err != nil {
		return info, err
	}

	lines, err := readFileString(filepath.Join(basepath, "mon_features"))
	if err != nil {
		return info, err
	}
	info.monFeatures = strings.Split(lines, "\n")
	sort.Strings(info.monFeatures)

	return info, nil
}

// Supported returns true if L3 monitoring is supported and enabled in the system
func (i l3MonInfo) Supported() bool {
	return i.numRmids != 0 && len(i.monFeatures) > 0
}

func getMBInfo(basepath string) (mbInfo, error) {
	var err error
	info := mbInfo{}

	info.bandwidthGran, err = readFileUint64(filepath.Join(basepath, "bandwidth_gran"))
	if err != nil {
		return info, err
	}
	info.delayLinear, err = readFileUint64(filepath.Join(basepath, "delay_linear"))
	if err != nil {
		return info, err
	}
	info.minBandwidth, err = readFileUint64(filepath.Join(basepath, "min_bandwidth"))
	if err != nil {
		return info, err
	}
	info.numClosids, err = readFileUint64(filepath.Join(basepath, "num_closids"))
	if err != nil {
		return info, err
	}

	// Detect MBps mode directly from mount options as it's not visible in MB
	// info directory
	_, mountOpts, err := getResctrlMountInfo()
	if err != nil {
		return info, fmt.Errorf("failed to get resctrl mount options: %v", err)
	}
	if _, ok := mountOpts["mba_MBps"]; ok {
		info.mbpsEnabled = true
	}

	return info, nil
}

// Supported returns true if memory bandwidth allocation has is supported and enabled in the system
func (i mbInfo) Supported() bool {
	return i.minBandwidth != 0
}

func getCacheIds(basepath string, prefix string) ([]uint64, error) {
	var ids []uint64

	// Parse cache IDs from the root schemata
	data, err := readFileString(filepath.Join(basepath, "schemata"))
	if err != nil {
		return ids, fmt.Errorf("failed to read root schemata: %v", err)
	}

	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		lineSplit := strings.SplitN(trimmed, ":", 2)

		// Find line with given resource prefix
		if len(lineSplit) == 2 && strings.HasPrefix(lineSplit[0], prefix) {
			schema := strings.Split(lineSplit[1], ";")
			ids = make([]uint64, len(schema))

			// Get individual cache configurations from the schema
			for idx, definition := range schema {
				split := strings.Split(definition, "=")
				if len(split) != 2 {
					return ids, fmt.Errorf("looks like an invalid schema %q", trimmed)
				}
				ids[idx], err = strconv.ParseUint(split[0], 10, 64)
				if err != nil {
					return ids, fmt.Errorf("failed to parse cache id in %q: %v", trimmed, err)
				}
			}
			return ids, nil
		}
	}
	return ids, fmt.Errorf("no %s resources in root schemata", prefix)
}

func getResctrlMountInfo() (string, map[string]struct{}, error) {
	mountOptions := map[string]struct{}{}

	f, err := os.Open(mountInfoPath)
	if err != nil {
		return "", mountOptions, err
	}
	defer f.Close() // nolint:errcheck

	s := bufio.NewScanner(f)
	for s.Scan() {
		split := strings.Split(s.Text(), " ")
		if len(split) > 3 && split[2] == "resctrl" {
			opts := strings.Split(split[3], ",")
			for _, opt := range opts {
				if opt != "" {
					mountOptions[opt] = struct{}{}
				}
			}
			return split[1], mountOptions, nil
		}
	}
	return "", mountOptions, fmt.Errorf("resctrl not found in " + mountInfoPath)
}

func readFileUint64(path string) (uint64, error) {
	data, err := readFileString(path)
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(data, 10, 64)
}

func readFileBitmask(path string) (bitmask, error) {
	data, err := readFileString(path)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseUint(data, 16, 64)
	return bitmask(value), err
}

func readFileString(path string) (string, error) {
	data, err := os.ReadFile(path)
	return strings.TrimSpace(string(data)), err
}
