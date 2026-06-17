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
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"sort"
	"strconv"
	"strings"

	"github.com/intel/goresctrl/pkg/utils"
)

// Config is the user-specified RDT configuration.
type Config struct {
	Options    Options `json:"options"`
	Partitions map[string]struct {
		L2Allocation CatConfig `json:"l2Allocation"`
		L3Allocation CatConfig `json:"l3Allocation"`
		MBAllocation MbaConfig `json:"mbAllocation"`
		Classes      map[string]struct {
			L2Allocation CatConfig         `json:"l2Allocation"`
			L3Allocation CatConfig         `json:"l3Allocation"`
			MBAllocation MbaConfig         `json:"mbAllocation"`
			Kubernetes   KubernetesOptions `json:"kubernetes"`
		} `json:"classes"`
	} `json:"partitions"`
}

// CatConfig contains the L2 or L3 cache allocation configuration for one partition or class.
type CatConfig map[string]CacheIdCatConfig

// MbaConfig contains the memory bandwidth configuration for one partition or class.
type MbaConfig map[string]CacheIdMbaConfig

// CacheIdCatConfig is the cache allocation configuration for one cache id.
// Code and Data represent an optional configuration for separate code and data
// paths and only have effect when RDT CDP (Code and Data Prioritization) is
// enabled in the system. Code and Data go in tandem so that both or neither
// must be specified - only specifying the other is considered a configuration
// error.
type CacheIdCatConfig struct {
	Unified CacheProportion
	Code    CacheProportion
	Data    CacheProportion
}

// CacheIdMbaConfig is the memory bandwidth configuration for one cache id.
// It's an array of at most two values, specifying separate values to be used
// for percentage based and MBps based memory bandwidth allocation. For
// example, `{"80%", "1000MBps"}` would allocate 80% if percentage based
// allocation is used by the Linux kernel, or 1000 MBps in case MBps based
// allocation is in use.
type CacheIdMbaConfig []MbProportion

// MbProportion specifies a share of available memory bandwidth. It's an
// integer value followed by a unit. Two units are supported:
//
// - percentage, e.g. `80%`
// - MBps, e.g. `1000MBps`
type MbProportion string

// CacheProportion specifies a share of the available cache lines.
// Supported formats:
//
// - percentage, e.g. `50%`
// - percentage range, e.g. `50-60%`
// - bit numbers, e.g. `0-5`, `2,3`, must contain one contiguous block of bits set
// - hex bitmask, e.g. `0xff0`, must contain one contiguous block of bits set
type CacheProportion string

// CacheIdAll is a special cache id used to denote a default, used as a
// fallback for all cache ids that are not explicitly specified.
const CacheIdAll = "all"

// config represents the final (parsed and resolved) runtime configuration of
// RDT Control
type config struct {
	Options    Options
	Partitions partitionSet
	Classes    classSet
}

// partitionSet represents the pool of rdt partitions
type partitionSet map[string]*partitionConfig

// classSet represents the pool of rdt classes
type classSet map[string]*classConfig

// partitionConfig is the final configuration of one partition
type partitionConfig struct {
	CAT map[cacheLevel]catSchema
	MB  mbSchema
}

// classConfig represents configuration of one class, i.e. one CTRL group in
// the Linux resctrl interface
type classConfig struct {
	Partition  string
	CATSchema  map[cacheLevel]catSchema
	MBSchema   mbSchema
	Kubernetes KubernetesOptions
}

// Options contains common settings.
type Options struct {
	L2 CatOptions `json:"l2"`
	L3 CatOptions `json:"l3"`
	MB MbOptions  `json:"mb"`
}

// CatOptions contains the common settings for cache allocation.
type CatOptions struct {
	Optional bool
}

// MbOptions contains the common settings for memory bandwidth allocation.
type MbOptions struct {
	Optional bool
}

// KubernetesOptions contains per-class settings for the Kubernetes-related functionality.
type KubernetesOptions struct {
	DenyPodAnnotation       bool `json:"denyPodAnnotation"`
	DenyContainerAnnotation bool `json:"denyContainerAnnotation"`
}

// catSchema represents a cache part of the schemata of a class (i.e. resctrl group)
type catSchema struct {
	Lvl   cacheLevel
	Alloc catSchemaRaw
}

// catSchemaRaw is the cache schemata without the information about cache level
type catSchemaRaw map[uint64]catAllocation

// mbSchema represents the MB part of the schemata of a class (i.e. resctrl group)
type mbSchema map[uint64]uint64

// catAllocation describes the allocation configuration for one cache id
type catAllocation struct {
	Unified cacheAllocation
	Code    cacheAllocation `json:",omitempty"`
	Data    cacheAllocation `json:",omitempty"`
}

// cacheAllocation is the basic interface for handling cache allocations of one
// type (unified, code, data)
type cacheAllocation interface {
	Overlay(bitmask, uint64) (bitmask, error)
}

// catAbsoluteAllocation represents an explicitly specified cache allocation
// bitmask
type catAbsoluteAllocation bitmask

// catPctAllocation represents a relative (percentage) share of the available
// bitmask
type catPctAllocation uint64

// catPctRangeAllocation represents a percentage range of the available bitmask
type catPctRangeAllocation struct {
	lowPct  uint64
	highPct uint64
}

// catSchemaType represents different L3 cache allocation schemes
type catSchemaType string

const (
	// catSchemaTypeUnified is the schema type when CDP is not enabled
	catSchemaTypeUnified catSchemaType = "unified"
	// catSchemaTypeCode is the 'code' part of CDP schema
	catSchemaTypeCode catSchemaType = "code"
	// catSchemaTypeData is the 'data' part of CDP schema
	catSchemaTypeData catSchemaType = "data"
)

// cat returns CAT options for the specified cache level.
func (o Options) cat(lvl cacheLevel) CatOptions {
	switch lvl {
	case L2:
		return o.L2
	case L3:
		return o.L3
	}
	return CatOptions{}
}

func (t catSchemaType) toResctrlStr() string {
	if t == catSchemaTypeUnified {
		return ""
	}
	return strings.ToUpper(string(t))
}

const (
	mbSuffixPct  = "%"
	mbSuffixMbps = "MBps"
)

func newCatSchema(typ cacheLevel) catSchema {
	return catSchema{
		Lvl:   typ,
		Alloc: make(map[uint64]catAllocation),
	}
}

// toStr returns the CAT schema in a format accepted by the Linux kernel
// resctrl (schemata) interface
func (s catSchema) toStr(typ catSchemaType, baseSchema catSchema) (string, error) {
	schema := string(s.Lvl) + typ.toResctrlStr() + ":"
	sep := ""

	// Get a sorted slice of cache ids for deterministic output
	ids := append([]uint64{}, info.cat[s.Lvl].cacheIds...)
	utils.SortUint64s(ids)

	minBits := info.cat[s.Lvl].minCbmBits()
	for _, id := range ids {
		// Default to 100%
		bmask := info.cat[s.Lvl].cbmMask()

		if base, ok := baseSchema.Alloc[id]; ok {
			baseMask, ok := base.getEffective(typ).(catAbsoluteAllocation)
			if !ok {
				return "", fmt.Errorf("BUG: basemask not of type catAbsoluteAllocation")
			}
			bmask = bitmask(baseMask)
		}

		if s.Alloc != nil {
			var err error

			masks := s.Alloc[id]
			overlayMask := masks.getEffective(typ)

			bmask, err = overlayMask.Overlay(bmask, minBits)
			if err != nil {
				return "", err
			}
		}
		schema += fmt.Sprintf("%s%d=%x", sep, id, bmask)
		sep = ";"
	}

	return schema + "\n", nil
}

func (a catAllocation) get(typ catSchemaType) cacheAllocation {
	switch typ {
	case catSchemaTypeCode:
		return a.Code
	case catSchemaTypeData:
		return a.Data
	}
	return a.Unified
}

func (a catAllocation) set(typ catSchemaType, v cacheAllocation) catAllocation {
	switch typ {
	case catSchemaTypeCode:
		a.Code = v
	case catSchemaTypeData:
		a.Data = v
	default:
		a.Unified = v
	}

	return a
}

func (a catAllocation) getEffective(typ catSchemaType) cacheAllocation {
	switch typ {
	case catSchemaTypeCode:
		if a.Code != nil {
			return a.Code
		}
	case catSchemaTypeData:
		if a.Data != nil {
			return a.Data
		}
	}
	// Use Unified as the default/fallback for Code and Data
	return a.Unified
}

// Overlay function of the cacheAllocation interface
func (a catAbsoluteAllocation) Overlay(baseMask bitmask, minBits uint64) (bitmask, error) {
	if err := verifyCatBaseMask(baseMask, minBits); err != nil {
		return 0, err
	}

	shiftWidth := baseMask.lsbOne()

	// Treat our bitmask relative to the basemask
	bmask := bitmask(a) << shiftWidth

	// Do bounds checking that we're "inside" the base mask
	if bmask|baseMask != baseMask {
		return 0, fmt.Errorf("bitmask %#x (%#x << %d) does not fit basemask %#x", bmask, a, shiftWidth, baseMask)
	}

	return bmask, nil
}

// MarshalJSON implements the Marshaler interface of "encoding/json"
func (a catAbsoluteAllocation) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%#x\"", a)), nil
}

// Overlay function of the cacheAllocation interface
func (a catPctAllocation) Overlay(baseMask bitmask, minBits uint64) (bitmask, error) {
	return catPctRangeAllocation{highPct: uint64(a)}.Overlay(baseMask, minBits)
}

// Overlay function of the cacheAllocation interface
func (a catPctRangeAllocation) Overlay(baseMask bitmask, minBits uint64) (bitmask, error) {
	if err := verifyCatBaseMask(baseMask, minBits); err != nil {
		return 0, err
	}

	baseMaskMsb := uint64(baseMask.msbOne())
	baseMaskLsb := uint64(baseMask.lsbOne())
	baseMaskNumBits := baseMaskMsb - baseMaskLsb + 1

	low, high := a.lowPct, a.highPct
	if low == 0 {
		low = 1
	}
	if low > high || low > 100 || high > 100 {
		return 0, fmt.Errorf("invalid percentage range in %v", a)
	}

	// Convert percentage limits to bit numbers
	// Our effective range is 1%-100%, use substraction (-1) because of
	// arithmetics, so that we don't overflow on 100%
	lsb := (low - 1) * baseMaskNumBits / 100
	msb := (high - 1) * baseMaskNumBits / 100

	// Make sure the number of bits set satisfies the minimum requirement
	numBits := msb - lsb + 1
	if numBits < minBits {
		gap := minBits - numBits

		// First, widen the mask from the "lsb end"
		if gap <= lsb {
			lsb -= gap
			gap = 0
		} else {
			gap -= lsb
			lsb = 0
		}
		// If needed, widen the mask from the "msb end"
		msbAvailable := baseMaskNumBits - msb - 1
		if gap <= msbAvailable {
			msb += gap
		} else {
			return 0, fmt.Errorf("BUG: not enough bits available for cache bitmask (%v applied on basemask %#x)", a, baseMask)
		}
	}

	value := ((1 << (msb - lsb + 1)) - 1) << (lsb + baseMaskLsb)

	return bitmask(value), nil
}

func verifyCatBaseMask(baseMask bitmask, minBits uint64) error {
	if baseMask == 0 {
		return fmt.Errorf("empty basemask not allowed")
	}

	// Check that the basemask contains one (and only one) contiguous block of
	// (enough) bits set
	baseMaskWidth := baseMask.msbOne() - baseMask.lsbOne() + 1
	if bits.OnesCount64(uint64(baseMask)) != baseMaskWidth {
		return fmt.Errorf("invalid basemask %#x: more than one block of bits set", baseMask)
	}
	if uint64(bits.OnesCount64(uint64(baseMask))) < minBits {
		return fmt.Errorf("invalid basemask %#x: fewer than %d bits set", baseMask, minBits)
	}

	return nil
}

// MarshalJSON implements the Marshaler interface of "encoding/json"
func (a catPctAllocation) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%d%%\"", a)), nil
}

// MarshalJSON implements the Marshaler interface of "encoding/json"
func (a catPctRangeAllocation) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%d-%d%%\"", a.lowPct, a.highPct)), nil
}

// toStr returns the MB schema in a format accepted by the Linux kernel
// resctrl (schemata) interface
func (s mbSchema) toStr(base map[uint64]uint64) string {
	schema := "MB:"
	sep := ""

	// Get a sorted slice of cache ids for deterministic output
	ids := append([]uint64{}, info.mb.cacheIds...)
	utils.SortUint64s(ids)

	for _, id := range ids {
		baseAllocation, ok := base[id]
		if !ok {
			if info.mb.mbpsEnabled {
				baseAllocation = math.MaxUint32
			} else {
				baseAllocation = 100
			}
		}

		value := uint64(0)
		if info.mb.mbpsEnabled {
			value = math.MaxUint32
			if s != nil {
				value = s[id]
			}
			// Limit to given base value
			if value > baseAllocation {
				value = baseAllocation
			}
		} else {
			allocation := uint64(100)
			if s != nil {
				allocation = s[id]
			}
			value = allocation * baseAllocation / 100
			// Guarantee minimum bw so that writing out the schemata does not fail
			if value < info.mb.minBandwidth {
				value = info.mb.minBandwidth
			}
		}

		schema += fmt.Sprintf("%s%d=%d", sep, id, value)
		sep = ";"
	}

	return schema + "\n"
}

// listStrToArray parses a string containing a human-readable list of numbers
// into an integer array
func listStrToArray(str string) ([]int, error) {
	a := []int{}

	// Empty list
	if len(str) == 0 {
		return a, nil
	}

	ranges := strings.Split(str, ",")
	for _, ran := range ranges {
		split := strings.SplitN(ran, "-", 2)

		// We limit to 8 bits in order to avoid accidental super long slices
		num, err := strconv.ParseInt(split[0], 10, 8)
		if err != nil {
			return a, fmt.Errorf("invalid integer %q: %v", str, err)
		}

		if len(split) == 1 {
			a = append(a, int(num))
		} else {
			endNum, err := strconv.ParseInt(split[1], 10, 8)
			if err != nil {
				return a, fmt.Errorf("invalid integer in range %q: %v", str, err)
			}
			if endNum <= num {
				return a, fmt.Errorf("invalid integer range %q in %q", ran, str)
			}
			for i := num; i <= endNum; i++ {
				a = append(a, int(i))
			}
		}
	}
	sort.Ints(a)
	return a, nil
}

// resolve tries to resolve the requested configuration into a working
// configuration
func (c *Config) resolve() (config, error) {
	var err error
	conf := config{Options: c.Options}

	// TODO: Think a better (more structured) way to log this
	log.Debug("resolving configuration:\n" + utils.DumpJSON(c))

	conf.Partitions, err = c.resolvePartitions()
	if err != nil {
		return conf, err
	}

	conf.Classes, err = c.resolveClasses()

	return conf, err
}

// resolvePartitions tries to resolve the requested resource allocations of
// partitions
func (c *Config) resolvePartitions() (partitionSet, error) {
	// Initialize empty partition configuration
	conf := make(partitionSet, len(c.Partitions))
	for name := range c.Partitions {
		conf[name] = &partitionConfig{
			CAT: map[cacheLevel]catSchema{
				L2: newCatSchema(L2),
				L3: newCatSchema(L3),
			},
			MB: make(mbSchema, len(info.mb.cacheIds))}
	}

	// Resolve L2 partition allocations
	err := c.resolveCatPartitions(L2, conf)
	if err != nil {
		return nil, err
	}

	// Try to resolve L3 partition allocations
	err = c.resolveCatPartitions(L3, conf)
	if err != nil {
		return nil, err
	}

	// Try to resolve MB partition allocations
	err = c.resolveMBPartitions(conf)
	if err != nil {
		return nil, err
	}

	return conf, nil
}

// resolveCatPartitions tries to resolve requested cache allocations between partitions
func (c *Config) resolveCatPartitions(lvl cacheLevel, conf partitionSet) error {
	if len(c.Partitions) == 0 {
		return nil
	}

	// Resolve partitions in sorted order for reproducibility
	names := make([]string, 0, len(c.Partitions))
	for name := range c.Partitions {
		names = append(names, name)
	}
	sort.Strings(names)

	resolver := newCacheResolver(lvl, names)

	// Parse requested allocations from user config and load the resolver
	for _, name := range names {
		var allocations catSchema
		var err error
		switch lvl {
		case L2:
			allocations, err = c.Partitions[name].L2Allocation.toSchema(L2)
		case L3:
			allocations, err = c.Partitions[name].L3Allocation.toSchema(L3)
		}

		if err != nil {
			return fmt.Errorf("failed to parse %s allocation request for partition %q: %v", lvl, name, err)
		}

		resolver.requests[name] = allocations.Alloc
	}

	// Run resolver fo partition allocations
	grants, err := resolver.resolve()
	if err != nil {
		return err
	}
	if grants == nil {
		log.Debug("cache allocation disabled for all partitions", "cacheLevel", lvl)
		return nil
	}

	for name, grant := range grants {
		conf[name].CAT[lvl] = grant
	}

	infoStr := fmt.Sprintf("actual (and requested) %s allocations per partition and cache id:\n", lvl)
	for name, partition := range resolver.requests {
		infoStr += name + "\n"
		for _, id := range resolver.ids {
			infoStr += fmt.Sprintf("  %2d: ", id)
			allocationReq := partition[id]
			for _, typ := range []catSchemaType{catSchemaTypeUnified, catSchemaTypeCode, catSchemaTypeData} {
				infoStr += string(typ) + " "
				requested := allocationReq.get(typ)
				switch v := requested.(type) {
				case catAbsoluteAllocation:
					infoStr += fmt.Sprintf("<absolute %#x>  ", v)
				case catPctAllocation:
					granted := grants[name].Alloc[id].get(typ).(catAbsoluteAllocation)
					requestedPct := fmt.Sprintf("(%d%%)", v)
					truePct := float64(bits.OnesCount64(uint64(granted))) * 100 / float64(resolver.bitsTotal)
					infoStr += fmt.Sprintf("%5.1f%% %-6s ", truePct, requestedPct)
				case nil:
					infoStr += "<not specified>  "
				}
			}
			infoStr += "\n"
		}
	}
	// TODO: Think a better (more structured) way to log this
	log.Debug("actual (and requested) allocations per partition and cache id\n" + infoStr)

	return nil
}

// cacheResolver is a helper for resolving exclusive (partition) cache // allocation requests
type cacheResolver struct {
	lvl        cacheLevel
	ids        []uint64
	minBits    uint64
	bitsTotal  uint64
	partitions []string
	requests   map[string]catSchemaRaw
	grants     map[string]catSchema
}

func newCacheResolver(lvl cacheLevel, partitions []string) *cacheResolver {
	r := &cacheResolver{
		lvl:        lvl,
		ids:        info.cat[lvl].cacheIds,
		minBits:    info.cat[lvl].minCbmBits(),
		bitsTotal:  uint64(info.cat[lvl].cbmMask().lsbZero()),
		partitions: partitions,
		requests:   make(map[string]catSchemaRaw, len(partitions)),
		grants:     make(map[string]catSchema, len(partitions))}

	for _, p := range partitions {
		r.grants[p] = catSchema{Lvl: lvl, Alloc: make(catSchemaRaw, len(r.ids))}
	}

	return r
}

func (r *cacheResolver) resolve() (map[string]catSchema, error) {
	for _, id := range r.ids {
		err := r.resolveID(id)
		if err != nil {
			return nil, err
		}
	}
	return r.grants, nil
}

// resolveCacheID resolves the partition allocations for one cache id
func (r *cacheResolver) resolveID(id uint64) error {
	for _, typ := range []catSchemaType{catSchemaTypeUnified, catSchemaTypeCode, catSchemaTypeData} {
		err := r.resolveType(id, typ)
		if err != nil {
			return err
		}
	}
	return nil
}

// resolveType resolve one schema type for one cache id
func (r *cacheResolver) resolveType(id uint64, typ catSchemaType) error {
	// Sanity check: if any partition has l3 allocation of this schema type
	// configured check that all other partitions have it, too
	nils := []string{}
	for _, partition := range r.partitions {
		if r.requests[partition][id].get(typ) == nil {
			nils = append(nils, partition)
		}
	}
	if len(nils) > 0 && len(nils) != len(r.partitions) {
		return fmt.Errorf("some partitions (%s) missing %s %q allocation request for cache id %d",
			strings.Join(nils, ", "), r.lvl, typ, id)
	}

	// Act depending on the type of the first request in the list
	a := r.requests[r.partitions[0]][id].get(typ)
	switch a.(type) {
	case catAbsoluteAllocation:
		return r.resolveAbsolute(id, typ)
	case nil:
	default:
		return r.resolveRelative(id, typ)
	}
	return nil
}

func (r *cacheResolver) resolveRelative(id uint64, typ catSchemaType) error {
	type reqHelper struct {
		name string
		req  uint64
	}

	// Sanity check:
	// 1. allocation requests are of the same type (relative)
	// 2. total allocation requested for this cache id does not exceed 100 percent
	// Additionally fill a helper structure for sorting partitions
	percentageTotal := uint64(0)
	reqs := make([]reqHelper, 0, len(r.partitions))
	for _, partition := range r.partitions {
		switch a := r.requests[partition][id].get(typ).(type) {
		case catPctAllocation:
			percentageTotal += uint64(a)
			reqs = append(reqs, reqHelper{name: partition, req: uint64(a)})
		case catAbsoluteAllocation:
			return fmt.Errorf("error resolving %s allocation for cache id %d: mixing "+
				"relative and absolute allocations between partitions not supported", r.lvl, id)
		case catPctRangeAllocation:
			return fmt.Errorf("percentage ranges in partition allocation not supported")
		default:
			return fmt.Errorf("BUG: unknown cacheAllocation type %T", a)
		}
	}
	if percentageTotal < 100 {
		log.Info("requested total partition allocation <100%%", "cacheLevel", r.lvl, "cacheID", id, "schemaType", typ, "percentage", percentageTotal)
	} else if percentageTotal > 100 {
		return fmt.Errorf("accumulated %s %q partition allocation requests for cache id %d exceeds 100%% (%d%%)", r.lvl, typ, id, percentageTotal)
	}

	// Sort partition allocations. We want to resolve smallest allocations
	// first in order to try to ensure that all allocations can be satisfied
	// because small percentages might need to be rounded up
	sort.Slice(reqs, func(i, j int) bool {
		return reqs[i].req < reqs[j].req
	})

	// Calculate number of bits granted to each partition.
	grants := make(map[string]uint64, len(r.partitions))
	bitsTotal := percentageTotal * uint64(r.bitsTotal) / 100
	bitsAvailable := bitsTotal
	for i, req := range reqs {
		percentageAvailable := bitsAvailable * percentageTotal / bitsTotal

		// This might happen e.g. if number of partitions would be greater
		// than the total number of bits
		if bitsAvailable < r.minBits {
			return fmt.Errorf("unable to resolve %s allocation for cache id %d, not enough exlusive bits available", r.lvl, id)
		}

		// Use integer arithmetics, effectively always rounding down
		// fractional allocations i.e. trying to avoid over-allocation
		numBits := req.req * bitsAvailable / percentageAvailable

		// Guarantee a non-zero allocation
		if numBits < r.minBits {
			numBits = r.minBits
		}
		// Don't overflow, allocate all remaining bits to the last partition
		if numBits > bitsAvailable || i == len(reqs)-1 {
			numBits = bitsAvailable
		}

		grants[req.name] = numBits
		bitsAvailable -= numBits
	}

	// Construct the actual bitmasks for each partition
	lsbID := uint64(0)
	for _, partition := range r.partitions {
		// Compose the actual bitmask
		v := r.grants[partition].Alloc[id].set(typ, catAbsoluteAllocation(bitmask(((1<<grants[partition])-1)<<lsbID)))
		r.grants[partition].Alloc[id] = v

		lsbID += grants[partition]
	}

	return nil
}

func (r *cacheResolver) resolveAbsolute(id uint64, typ catSchemaType) error {
	// Just sanity check:
	// 1. allocation requests of the correct type (absolute)
	// 2. allocations do not overlap
	mask := bitmask(0)
	for _, partition := range r.partitions {
		a, ok := r.requests[partition][id].get(typ).(catAbsoluteAllocation)
		if !ok {
			return fmt.Errorf("error resolving %s allocation for cache id %d: mixing absolute and relative allocations between partitions not supported", r.lvl, id)
		}
		if bitmask(a)&mask > 0 {
			return fmt.Errorf("overlapping %s partition allocation requests for cache id %d", r.lvl, id)
		}
		mask |= bitmask(a)

		r.grants[partition].Alloc[id] = r.grants[partition].Alloc[id].set(typ, a)
	}

	return nil
}

// resolveMBPartitions tries to resolve requested MB allocations between partitions
func (c *Config) resolveMBPartitions(conf partitionSet) error {
	// We use percentage values directly from the user conf
	for name, partition := range c.Partitions {
		allocations, err := partition.MBAllocation.toSchema()
		if err != nil {
			return fmt.Errorf("failed to resolve MB allocation for partition %q: %v", name, err)
		}
		for id, allocation := range allocations {
			conf[name].MB[id] = allocation
			// Check that we don't go under the minimum allowed bandwidth setting
			if !info.mb.mbpsEnabled && allocation < info.mb.minBandwidth {
				conf[name].MB[id] = info.mb.minBandwidth
			}
		}
	}

	return nil
}

// resolveClasses tries to resolve class allocations of all partitions
func (c *Config) resolveClasses() (classSet, error) {
	classes := make(classSet)

	for bname, partition := range c.Partitions {
		for gname, class := range partition.Classes {
			gname = unaliasClassName(gname)

			if !IsQualifiedClassName(gname) {
				return classes, fmt.Errorf("unqualified class name %q (must not be '.' or '..' and must not contain '/' or newline)", gname)
			}
			if _, ok := classes[gname]; ok {
				return classes, fmt.Errorf("class names must be unique, %q defined multiple times", gname)
			}

			var err error
			gc := &classConfig{Partition: bname,
				CATSchema:  make(map[cacheLevel]catSchema),
				Kubernetes: class.Kubernetes}

			gc.CATSchema[L2], err = class.L2Allocation.toSchema(L2)
			if err != nil {
				return classes, fmt.Errorf("failed to resolve L2 allocation for class %q: %v", gname, err)
			}
			if gc.CATSchema[L2].Alloc != nil && partition.L2Allocation == nil {
				return classes, fmt.Errorf("L2 allocation missing from partition %q but class %q specifies L2 schema", bname, gname)
			}

			gc.CATSchema[L3], err = class.L3Allocation.toSchema(L3)
			if err != nil {
				return classes, fmt.Errorf("failed to resolve L3 allocation for class %q: %v", gname, err)
			}
			if gc.CATSchema[L3].Alloc != nil && partition.L3Allocation == nil {
				return classes, fmt.Errorf("L3 allocation missing from partition %q but class %q specifies L3 schema", bname, gname)
			}

			gc.MBSchema, err = class.MBAllocation.toSchema()
			if err != nil {
				return classes, fmt.Errorf("failed to resolve MB allocation for class %q: %v", gname, err)
			}
			if gc.MBSchema != nil && partition.MBAllocation == nil {
				return classes, fmt.Errorf("MB allocation missing from partition %q but class %q specifies MB schema", bname, gname)
			}

			classes[gname] = gc
		}
	}

	return classes, nil
}

// toSchema converts a cache allocation config to effective allocation schema covering all cache IDs
func (c CatConfig) toSchema(lvl cacheLevel) (catSchema, error) {
	if c == nil {
		return catSchema{Lvl: lvl}, nil
	}

	allocations := newCatSchema(lvl)
	minBits := info.cat[lvl].minCbmBits()

	d, ok := c[CacheIdAll]
	if !ok {
		d = CacheIdCatConfig{Unified: "100%"}
	}
	defaultVal, err := d.parse(minBits)
	if err != nil {
		return allocations, err
	}

	// Pre-fill with defaults
	for _, i := range info.cat[lvl].cacheIds {
		allocations.Alloc[i] = defaultVal
	}

	for key, val := range c {
		if key == CacheIdAll {
			continue
		}

		ids, err := listStrToArray(key)
		if err != nil {
			return allocations, err
		}

		schemaVal, err := val.parse(minBits)
		if err != nil {
			return allocations, err
		}

		for _, id := range ids {
			if _, ok := allocations.Alloc[uint64(id)]; ok {
				allocations.Alloc[uint64(id)] = schemaVal
			}
		}
	}

	return allocations, nil
}

// catConfig is a helper for unmarshalling CatConfig
type catConfig CatConfig

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json"
func (c *CatConfig) UnmarshalJSON(data []byte) error {
	raw := new(interface{})

	err := json.Unmarshal(data, raw)
	if err != nil {
		return err
	}

	conf := CatConfig{}
	switch v := (*raw).(type) {
	case string:
		conf[CacheIdAll] = CacheIdCatConfig{Unified: CacheProportion(v)}
	default:
		// Use the helper type to avoid infinite recursion
		helper := catConfig{}
		if err := json.Unmarshal(data, &helper); err != nil {
			return err
		}
		for k, v := range helper {
			conf[k] = v
		}
	}
	*c = conf
	return nil
}

// toSchema converts an MB allocation config to effective allocation schema covering all cache IDs
func (c MbaConfig) toSchema() (mbSchema, error) {
	if c == nil {
		return nil, nil
	}

	d, ok := c[CacheIdAll]
	if !ok {
		d = CacheIdMbaConfig{"100" + mbSuffixPct, "4294967295" + mbSuffixMbps}
	}
	defaultVal, err := d.parse()
	if err != nil {
		return nil, err
	}

	allocations := make(mbSchema, len(info.mb.cacheIds))
	// Pre-fill with defaults
	for _, i := range info.mb.cacheIds {
		allocations[i] = defaultVal
	}

	for key, val := range c {
		if key == CacheIdAll {
			continue
		}

		ids, err := listStrToArray(key)
		if err != nil {
			return nil, err
		}

		schemaVal, err := val.parse()
		if err != nil {
			return nil, err
		}

		for _, id := range ids {
			if _, ok := allocations[uint64(id)]; ok {
				allocations[uint64(id)] = schemaVal
			}
		}
	}

	return allocations, nil
}

// mbaConfig is a helper for unmarshalling MbaConfig
type mbaConfig MbaConfig

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json"
func (c *MbaConfig) UnmarshalJSON(data []byte) error {
	raw := new(interface{})

	err := json.Unmarshal(data, raw)
	if err != nil {
		return err
	}

	conf := MbaConfig{}
	switch (*raw).(type) {
	case []interface{}:
		helper := CacheIdMbaConfig{}
		if err := json.Unmarshal(data, &helper); err != nil {
			return err
		}
		conf[CacheIdAll] = helper
	default:
		// Use the helper type to avoid infinite recursion
		helper := mbaConfig{}
		if err := json.Unmarshal(data, &helper); err != nil {
			return err
		}
		for k, v := range helper {
			conf[k] = v
		}
	}
	*c = conf
	return nil
}

// parse per cache-id CAT configuration into an effective allocation to be used
// in the CAT schema
func (c *CacheIdCatConfig) parse(minBits uint64) (catAllocation, error) {
	var err error
	allocation := catAllocation{}

	allocation.Unified, err = c.Unified.parse(minBits)
	if err != nil {
		return allocation, err
	}
	allocation.Code, err = c.Code.parse(minBits)
	if err != nil {
		return allocation, err
	}
	allocation.Data, err = c.Data.parse(minBits)
	if err != nil {
		return allocation, err
	}

	// Sanity check for the configuration
	if allocation.Unified == nil {
		return allocation, fmt.Errorf("'unified' not specified in cache schema %s", *c)
	}
	if allocation.Code != nil && allocation.Data == nil {
		return allocation, fmt.Errorf("'code' specified but missing 'data' from cache schema %s", *c)
	}
	if allocation.Code == nil && allocation.Data != nil {
		return allocation, fmt.Errorf("'data' specified but missing 'code' from cache schema %s", *c)
	}

	return allocation, nil
}

// cacheIdCatConfig is a helper for unmarshalling CacheIdCatConfig
type cacheIdCatConfig CacheIdCatConfig

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json"
func (c *CacheIdCatConfig) UnmarshalJSON(data []byte) error {
	raw := new(interface{})

	err := json.Unmarshal(data, raw)
	if err != nil {
		return err
	}

	conf := CacheIdCatConfig{}
	switch v := (*raw).(type) {
	case string:
		conf.Unified = CacheProportion(v)
	default:
		// Use the helper type to avoid infinite recursion
		helper := cacheIdCatConfig{}
		if err := json.Unmarshal(data, &helper); err != nil {
			return err
		}
		conf.Unified = helper.Unified
		conf.Code = helper.Code
		conf.Data = helper.Data
	}
	*c = conf
	return nil
}

// parse converts a per cache-id MBA configuration into effective value
// to be used in the MBA schema
func (c *CacheIdMbaConfig) parse() (uint64, error) {
	for _, v := range *c {
		str := string(v)
		if strings.HasSuffix(str, mbSuffixPct) {
			if !info.mb.mbpsEnabled {
				value, err := strconv.ParseUint(strings.TrimSuffix(str, mbSuffixPct), 10, 7)
				if err != nil {
					return 0, err
				}
				return value, nil
			}
		} else if strings.HasSuffix(str, mbSuffixMbps) {
			if info.mb.mbpsEnabled {
				value, err := strconv.ParseUint(strings.TrimSuffix(str, mbSuffixMbps), 10, 32)
				if err != nil {
					return 0, err
				}
				return value, nil
			}
		} else {
			log.Warn("unrecognized MBA allocation unit", "value", str)
		}
	}

	// No value for the active mode was specified
	if info.mb.mbpsEnabled {
		return 0, fmt.Errorf("missing 'MBps' value from mbSchema; required because 'mba_MBps' is enabled in the system")
	}
	return 0, fmt.Errorf("missing '%%' value from mbSchema; required because percentage-based MBA allocation is enabled in the system")
}

// parse converts a string value into cacheAllocation type
func (c CacheProportion) parse(minBits uint64) (cacheAllocation, error) {
	if c == "" {
		return nil, nil
	}

	if c[len(c)-1] == '%' {
		// Percentages of the max number of bits
		split := strings.SplitN(string(c)[0:len(c)-1], "-", 2)
		var allocation cacheAllocation

		if len(split) == 1 {
			pct, err := strconv.ParseUint(split[0], 10, 7)
			if err != nil {
				return allocation, err
			}
			if pct > 100 {
				return allocation, fmt.Errorf("invalid percentage value %q", c)
			}
			allocation = catPctAllocation(pct)
		} else {
			low, err := strconv.ParseUint(split[0], 10, 7)
			if err != nil {
				return allocation, err
			}
			high, err := strconv.ParseUint(split[1], 10, 7)
			if err != nil {
				return allocation, err
			}
			if low > high || low > 100 || high > 100 {
				return allocation, fmt.Errorf("invalid percentage range %q", c)
			}
			allocation = catPctRangeAllocation{lowPct: low, highPct: high}
		}

		return allocation, nil
	}

	// Absolute allocation
	var value uint64
	var err error
	if strings.HasPrefix(string(c), "0x") {
		// Hex value
		value, err = strconv.ParseUint(string(c[2:]), 16, 64)
		if err != nil {
			return nil, err
		}
	} else {
		// Last, try "list" format (i.e. smthg like 0,2,5-9,...)
		tmp, err := listStrToBitmask(string(c))
		value = uint64(tmp)
		if err != nil {
			return nil, err
		}
	}

	// Sanity check of absolute allocation: bitmask must (only) contain one
	// contiguous block of ones wide enough
	numOnes := bits.OnesCount64(value)
	if numOnes != bits.Len64(value)-bits.TrailingZeros64(value) {
		return nil, fmt.Errorf("invalid cache bitmask %q: more than one continuous block of ones", c)
	}
	if uint64(numOnes) < minBits {
		return nil, fmt.Errorf("invalid cache bitmask %q: number of bits less than %d", c, minBits)
	}

	return catAbsoluteAllocation(value), nil
}
