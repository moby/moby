package ebpf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/kallsyms"
	"github.com/cilium/ebpf/internal/kconfig"
	"github.com/cilium/ebpf/internal/linux"
	"github.com/cilium/ebpf/internal/platform"
	"github.com/cilium/ebpf/internal/sys"
)

// CollectionOptions control loading a collection into the kernel.
//
// Maps and Programs are passed to NewMapWithOptions and NewProgramsWithOptions.
type CollectionOptions struct {
	Maps     MapOptions
	Programs ProgramOptions

	// MapReplacements takes a set of Maps that will be used instead of
	// creating new ones when loading the CollectionSpec.
	//
	// For each given Map, there must be a corresponding MapSpec in
	// CollectionSpec.Maps, and its type, key/value size, max entries and flags
	// must match the values of the MapSpec.
	//
	// The given Maps are Clone()d before being used in the Collection, so the
	// caller can Close() them freely when they are no longer needed.
	MapReplacements map[string]*Map

	// Cache amortises the cost of decoding kernel BTF across multiple
	// Collection loads. Callers loading more than one Collection should
	// allocate a Cache via [btf.NewCache] and share it across loads.
	// If nil, a fresh cache is allocated per load and discarded.
	Cache *btf.Cache
}

// CollectionSpec describes a collection.
type CollectionSpec struct {
	Maps     map[string]*MapSpec
	Programs map[string]*ProgramSpec

	// Variables refer to global variables declared in the ELF. They can be read
	// and modified freely before loading the Collection. Modifying them after
	// loading has no effect on a running eBPF program.
	Variables map[string]*VariableSpec

	// Types holds type information about Maps and Programs.
	// Modifications to Types are currently undefined behaviour.
	Types *btf.Spec

	// ByteOrder specifies whether the ELF was compiled for
	// big-endian or little-endian architectures.
	ByteOrder binary.ByteOrder
}

// Copy returns a recursive copy of the spec.
func (cs *CollectionSpec) Copy() *CollectionSpec {
	if cs == nil {
		return nil
	}

	cpy := CollectionSpec{
		Maps:      copyMapOfSpecs(cs.Maps),
		Programs:  copyMapOfSpecs(cs.Programs),
		Variables: make(map[string]*VariableSpec, len(cs.Variables)),
		ByteOrder: cs.ByteOrder,
		Types:     cs.Types.Copy(),
	}

	for name, spec := range cs.Variables {
		cpy.Variables[name] = spec.Copy()
	}
	if cs.Variables == nil {
		cpy.Variables = nil
	}

	return &cpy
}

func copyMapOfSpecs[T interface{ Copy() T }](m map[string]T) map[string]T {
	if m == nil {
		return nil
	}

	cpy := make(map[string]T, len(m))
	for k, v := range m {
		cpy[k] = v.Copy()
	}

	return cpy
}

// Assign the contents of a CollectionSpec to a struct.
//
// This function is a shortcut to manually checking the presence
// of maps and programs in a CollectionSpec. Consider using bpf2go
// if this sounds useful.
//
// 'to' must be a pointer to a struct. A field of the
// struct is updated with values from Programs, Maps or Variables if it
// has an `ebpf` tag and its type is *ProgramSpec, *MapSpec or *VariableSpec.
// The tag's value specifies the name of the program or map as
// found in the CollectionSpec.
//
//	struct {
//	    Foo     *ebpf.ProgramSpec  `ebpf:"xdp_foo"`
//	    Bar     *ebpf.MapSpec      `ebpf:"bar_map"`
//	    Var     *ebpf.VariableSpec `ebpf:"some_var"`
//	    Ignored int
//	}
//
// Returns an error if any of the eBPF objects can't be found, or
// if the same Spec is assigned multiple times.
func (cs *CollectionSpec) Assign(to any) error {
	getValue := func(typ reflect.Type, name string) (any, error) {
		switch typ {
		case reflect.TypeFor[*ProgramSpec]():
			if p := cs.Programs[name]; p != nil {
				return p, nil
			}
			return nil, fmt.Errorf("missing program %q", name)

		case reflect.TypeFor[*MapSpec]():
			if m := cs.Maps[name]; m != nil {
				return m, nil
			}
			return nil, fmt.Errorf("missing map %q", name)

		case reflect.TypeFor[*VariableSpec]():
			if v := cs.Variables[name]; v != nil {
				return v, nil
			}
			return nil, fmt.Errorf("missing variable %q", name)

		default:
			return nil, fmt.Errorf("unsupported type %s", typ)
		}
	}

	return assignValues(to, getValue)
}

// LoadAndAssign loads Maps and Programs into the kernel and assigns them
// to a struct.
//
// Omitting Map/Program.Close() during application shutdown is an error.
// See the package documentation for details around Map and Program lifecycle.
//
// This function is a shortcut to manually checking the presence
// of maps and programs in a CollectionSpec. Consider using bpf2go
// if this sounds useful.
//
// 'to' must be a pointer to a struct. A field of the struct is updated with
// a Program or Map if it has an `ebpf` tag and its type is *Program or *Map.
// The tag's value specifies the name of the program or map as found in the
// CollectionSpec. Before updating the struct, the requested objects and their
// dependent resources are loaded into the kernel and populated with values if
// specified.
//
//	struct {
//	    Foo     *ebpf.Program `ebpf:"xdp_foo"`
//	    Bar     *ebpf.Map     `ebpf:"bar_map"`
//	    Ignored int
//	}
//
// opts may be nil.
//
// Returns an error if any of the fields can't be found, or
// if the same Map or Program is assigned multiple times.
func (cs *CollectionSpec) LoadAndAssign(to any, opts *CollectionOptions) error {
	loader, err := newCollectionLoader(cs, opts)
	if err != nil {
		return err
	}
	defer loader.close()

	// Support assigning Programs and Maps, lazy-loading the required objects.
	assignedMaps := make(map[string]bool)
	assignedProgs := make(map[string]bool)
	assignedVars := make(map[string]bool)

	getValue := func(typ reflect.Type, name string) (any, error) {
		switch typ {

		case reflect.TypeFor[*Program]():
			assignedProgs[name] = true
			return loader.loadProgram(name)

		case reflect.TypeFor[*Map]():
			assignedMaps[name] = true
			return loader.loadMap(name)

		case reflect.TypeFor[*Variable]():
			assignedVars[name] = true
			return loader.loadVariable(name)

		default:
			return nil, fmt.Errorf("unsupported type %s", typ)
		}
	}

	// Load the Maps and Programs requested by the annotated struct.
	if err := assignValues(to, getValue); err != nil {
		return fmt.Errorf("assign values: %w", err)
	}

	// Populate the requested maps. Has a chance of lazy-loading other dependent maps.
	if err := loader.populateDeferredMaps(); err != nil {
		return fmt.Errorf("populate deferred maps: %w", err)
	}

	// Evaluate the loader's objects after all (lazy)loading has taken place.
	for n, m := range loader.maps {
		if m.typ.canStoreProgram() {
			// Require all lazy-loaded ProgramArrays to be assigned to the given object.
			// The kernel empties a ProgramArray once the last user space reference
			// to it closes, which leads to failed tail calls. Combined with the library
			// closing map fds via GC finalizers this can lead to surprising behaviour.
			// Only allow unassigned ProgramArrays when the library hasn't pre-populated
			// any entries from static value declarations. At this point, we know the map
			// is empty and there's no way for the caller to interact with the map going
			// forward.
			if !assignedMaps[n] && len(cs.Maps[n].Contents) > 0 {
				return fmt.Errorf("ProgramArray %s must be assigned to prevent missed tail calls", n)
			}
		}
	}

	// Prevent loader.cleanup() from closing assigned Maps and Programs.
	for m := range assignedMaps {
		delete(loader.maps, m)
	}
	for p := range assignedProgs {
		delete(loader.programs, p)
	}
	for p := range assignedVars {
		delete(loader.vars, p)
	}

	return nil
}

// Collection is a collection of live BPF resources present in the kernel.
type Collection struct {
	Programs map[string]*Program
	Maps     map[string]*Map

	// Variables contains global variables used by the Collection's program(s). On
	// kernels older than 5.5, most interactions with Variables return
	// [ErrNotSupported].
	Variables map[string]*Variable
}

// NewCollection creates a Collection from the given spec, creating and
// loading its declared resources into the kernel.
//
// Omitting Collection.Close() during application shutdown is an error.
// See the package documentation for details around Map and Program lifecycle.
func NewCollection(spec *CollectionSpec) (*Collection, error) {
	return NewCollectionWithOptions(spec, CollectionOptions{})
}

// NewCollectionWithOptions creates a Collection from the given spec using
// options, creating and loading its declared resources into the kernel.
//
// Omitting Collection.Close() during application shutdown is an error.
// See the package documentation for details around Map and Program lifecycle.
func NewCollectionWithOptions(spec *CollectionSpec, opts CollectionOptions) (*Collection, error) {
	loader, err := newCollectionLoader(spec, &opts)
	if err != nil {
		return nil, err
	}
	defer loader.close()

	// Create maps first, as their fds need to be linked into programs.
	for mapName := range spec.Maps {
		if _, err := loader.loadMap(mapName); err != nil {
			return nil, err
		}
	}

	for progName, prog := range spec.Programs {
		if prog.Type == UnspecifiedProgram {
			continue
		}

		if _, err := loader.loadProgram(progName); err != nil {
			return nil, err
		}
	}

	for varName := range spec.Variables {
		if _, err := loader.loadVariable(varName); err != nil {
			return nil, err
		}
	}

	// Maps can contain Program and Map stubs, so populate them after
	// all Maps and Programs have been successfully loaded.
	if err := loader.populateDeferredMaps(); err != nil {
		return nil, err
	}

	// Prevent loader.cleanup from closing maps, programs and vars.
	maps, progs, vars := loader.maps, loader.programs, loader.vars
	loader.maps, loader.programs, loader.vars = nil, nil, nil

	return &Collection{
		progs,
		maps,
		vars,
	}, nil
}

type collectionLoader struct {
	coll     *CollectionSpec
	opts     *CollectionOptions
	maps     map[string]*Map
	programs map[string]*Program
	vars     map[string]*Variable
	types    *btf.Cache
}

func newCollectionLoader(coll *CollectionSpec, opts *CollectionOptions) (*collectionLoader, error) {
	if opts == nil {
		opts = &CollectionOptions{}
	}

	// Check for existing MapSpecs in the CollectionSpec for all provided replacement maps.
	for name := range opts.MapReplacements {
		if _, ok := coll.Maps[name]; !ok {
			return nil, fmt.Errorf("replacement map %s not found in CollectionSpec", name)
		}
	}

	if err := populateKallsyms(coll.Programs); err != nil {
		return nil, fmt.Errorf("populating kallsyms caches: %w", err)
	}

	cache := opts.Cache
	if cache == nil {
		cache = btf.NewCache()
	}

	return &collectionLoader{
		coll,
		opts,
		make(map[string]*Map),
		make(map[string]*Program),
		make(map[string]*Variable),
		cache,
	}, nil
}

// populateKallsyms populates kallsyms caches, making lookups cheaper later on
// during individual program loading. Since we have less context available
// at those stages, we batch the lookups here instead to avoid redundant work.
func populateKallsyms(progs map[string]*ProgramSpec) error {
	// Look up addresses of all kernel symbols referenced by all programs.
	addrs := make(map[string]uint64)
	for _, p := range progs {
		iter := p.Instructions.Iterate()
		for iter.Next() {
			ins := iter.Ins
			meta, _ := ins.Metadata.Get(ksymMetaKey{}).(*ksymMeta)
			if meta != nil {
				addrs[meta.Name] = 0
			}
		}
	}
	if len(addrs) != 0 {
		if err := kallsyms.AssignAddresses(addrs); err != nil {
			return fmt.Errorf("getting addresses from kallsyms: %w", err)
		}
	}

	return nil
}

// close all resources left over in the collectionLoader.
func (cl *collectionLoader) close() {
	for _, m := range cl.maps {
		m.Close()
	}
	for _, p := range cl.programs {
		p.Close()
	}
}

func (cl *collectionLoader) loadMap(mapName string) (*Map, error) {
	if m := cl.maps[mapName]; m != nil {
		return m, nil
	}

	mapSpec := cl.coll.Maps[mapName]
	if mapSpec == nil {
		return nil, fmt.Errorf("missing map %s", mapName)
	}

	mapSpec = mapSpec.Copy()

	// Defer setting the mmapable flag on maps until load time. This avoids the
	// MapSpec having different flags on some kernel versions. Also avoid running
	// syscalls during ELF loading, so platforms like wasm can also parse an ELF.
	if isDataSection(mapSpec.Name) && haveMmapableMaps() == nil {
		mapSpec.Flags |= sys.BPF_F_MMAPABLE
	}

	if replaceMap, ok := cl.opts.MapReplacements[mapName]; ok {
		// Check compatibility with the replacement map after setting
		// feature-dependent map flags.
		if err := mapSpec.Compatible(replaceMap); err != nil {
			return nil, fmt.Errorf("using replacement map %s: %w", mapSpec.Name, err)
		}

		// Clone the map to avoid closing user's map later on.
		m, err := replaceMap.Clone()
		if err != nil {
			return nil, err
		}

		cl.maps[mapName] = m
		return m, nil
	}

	if err := mapSpec.updateDataSection(cl.coll.Variables, mapName); err != nil {
		return nil, fmt.Errorf("assembling contents of map %s: %w", mapName, err)
	}

	m, err := newMapWithOptions(mapSpec, cl.opts.Maps, cl.types)
	if err != nil {
		return nil, fmt.Errorf("map %s: %w", mapName, err)
	}

	// Finalize 'scalar' maps that don't refer to any other eBPF resources
	// potentially pending creation. This is needed for frozen maps like .rodata
	// that need to be finalized before invoking the verifier.
	if !mapSpec.Type.canStoreMapOrProgram() {
		if err := m.finalize(mapSpec); err != nil {
			_ = m.Close()
			return nil, fmt.Errorf("finalizing map %s: %w", mapName, err)
		}
	}

	cl.maps[mapName] = m
	return m, nil
}

func (cl *collectionLoader) loadProgram(progName string) (*Program, error) {
	if prog := cl.programs[progName]; prog != nil {
		return prog, nil
	}

	progSpec := cl.coll.Programs[progName]
	if progSpec == nil {
		return nil, fmt.Errorf("unknown program %s", progName)
	}

	// Bail out early if we know the kernel is going to reject the program.
	// This skips loading map dependencies, saving some cleanup work later.
	if progSpec.Type == UnspecifiedProgram {
		return nil, fmt.Errorf("cannot load program %s: program type is unspecified", progName)
	}

	progSpec = progSpec.Copy()

	// Rewrite any reference to a valid map in the program's instructions,
	// which includes all of its dependencies.
	for i := range progSpec.Instructions {
		ins := &progSpec.Instructions[i]

		if !ins.IsLoadFromMap() || ins.Reference() == "" {
			continue
		}

		// Don't overwrite map loads containing non-zero map fd's,
		// they can be manually included by the caller.
		// Map FDs/IDs are placed in the lower 32 bits of Constant.
		if int32(ins.Constant) > 0 {
			continue
		}

		m, err := cl.loadMap(ins.Reference())
		if err != nil {
			return nil, fmt.Errorf("program %s: %w", progName, err)
		}

		if err := ins.AssociateMap(m); err != nil {
			return nil, fmt.Errorf("program %s: map %s: %w", progName, ins.Reference(), err)
		}
	}

	prog, err := newProgramWithOptions(progSpec, cl.opts.Programs, cl.types)
	if err != nil {
		return nil, fmt.Errorf("program %s: %w", progName, err)
	}

	cl.programs[progName] = prog

	return prog, nil
}

func (cl *collectionLoader) loadVariable(varName string) (*Variable, error) {
	if v := cl.vars[varName]; v != nil {
		return v, nil
	}

	varSpec := cl.coll.Variables[varName]
	if varSpec == nil {
		return nil, fmt.Errorf("unknown variable %s", varName)
	}

	m, err := cl.loadMap(varSpec.SectionName)
	if err != nil {
		return nil, fmt.Errorf("variable %s: %w", varName, err)
	}

	// If the kernel is too old or the underlying map was created without
	// BPF_F_MMAPABLE, [Map.Memory] will return ErrNotSupported. In this case,
	// emit a Variable with a nil Memory. This keeps Collection{Spec}.Variables
	// consistent across systems with different feature sets without breaking
	// LoadAndAssign.
	var mm *Memory
	if unsafeMemory {
		mm, err = m.unsafeMemory()
	} else {
		mm, err = m.Memory()
	}
	if err != nil && !errors.Is(err, ErrNotSupported) {
		return nil, fmt.Errorf("variable %s: getting memory for map %s: %w", varName, varSpec.SectionName, err)
	}

	v, err := newVariable(
		varSpec.Name,
		varSpec.Offset,
		varSpec.Size(),
		varSpec.Type,
		mm,
	)
	if err != nil {
		return nil, fmt.Errorf("variable %s: %w", varName, err)
	}

	cl.vars[varName] = v
	return v, nil
}

// populateDeferredMaps iterates maps holding programs or other maps and loads
// any dependencies. Populates all maps in cl and freezes them if specified.
func (cl *collectionLoader) populateDeferredMaps() error {
	for mapName, m := range cl.maps {
		mapSpec, ok := cl.coll.Maps[mapName]
		if !ok {
			return fmt.Errorf("missing map spec %s", mapName)
		}

		// Scalar maps without Map or Program references are finalized during
		// creation. Don't finalize them again.
		if !mapSpec.Type.canStoreMapOrProgram() {
			continue
		}

		mapSpec = mapSpec.Copy()

		// MapSpecs that refer to inner maps or programs within the same
		// CollectionSpec do so using strings. These strings are used as the key
		// to look up the respective object in the Maps or Programs fields.
		// Resolve those references to actual Map or Program resources that
		// have been loaded into the kernel.
		for i, kv := range mapSpec.Contents {
			objName, ok := kv.Value.(string)
			if !ok {
				continue
			}

			switch t := mapSpec.Type; {
			case t.canStoreProgram():
				// loadProgram is idempotent and could return an existing Program.
				prog, err := cl.loadProgram(objName)
				if err != nil {
					return fmt.Errorf("loading program %s, for map %s: %w", objName, mapName, err)
				}
				mapSpec.Contents[i] = MapKV{kv.Key, prog}

			case t.canStoreMap():
				// loadMap is idempotent and could return an existing Map.
				innerMap, err := cl.loadMap(objName)
				if err != nil {
					return fmt.Errorf("loading inner map %s, for map %s: %w", objName, mapName, err)
				}
				mapSpec.Contents[i] = MapKV{kv.Key, innerMap}
			}
		}

		if mapSpec.Type == StructOpsMap {
			// populate StructOps data into `kernVData`
			if err := cl.populateStructOps(m, mapSpec); err != nil {
				return err
			}
		}

		// Populate and freeze the map if specified.
		if err := m.finalize(mapSpec); err != nil {
			return fmt.Errorf("populating map %s: %w", mapName, err)
		}
	}

	return nil
}

// populateStructOps translates the user struct bytes into the kernel value struct
// layout for a struct_ops map and writes the result back to mapSpec.Contents[0].
func (cl *collectionLoader) populateStructOps(m *Map, mapSpec *MapSpec) error {
	userType, ok := btf.As[*btf.Struct](mapSpec.Value)
	if !ok {
		return fmt.Errorf("value should be a *Struct")
	}

	userData, err := mapSpec.dataSection()
	if err != nil {
		return fmt.Errorf("getting data section: %w", err)
	}
	if len(userData) < int(userType.Size) {
		return fmt.Errorf("user data too short: have %d, need at least %d", len(userData), userType.Size)
	}

	vType, _, module, err := structOpsFindTarget(userType, cl.types)
	if err != nil {
		return fmt.Errorf("struct_ops value type %q: %w", userType.Name, err)
	}
	defer module.Close()

	// Find the inner ops struct embedded in the value struct.
	kType, kTypeOff, err := structOpsFindInnerType(vType)
	if err != nil {
		return err
	}

	kernVData := make([]byte, int(vType.Size))
	for _, m := range userType.Members {
		i := slices.IndexFunc(kType.Members, func(km btf.Member) bool {
			return km.Name == m.Name
		})

		// Allow field to not exist in target as long as the source is zero.
		if i == -1 {
			mSize, err := btf.Sizeof(m.Type)
			if err != nil {
				return fmt.Errorf("sizeof(user.%s): %w", m.Name, err)
			}
			srcOff := int(m.Offset.Bytes())
			if srcOff < 0 || srcOff+mSize > len(userData) {
				return fmt.Errorf("member %q: userdata is too small", m.Name)
			}

			// let fail if the field in type user type is missing in type kern type
			if !structOpsIsMemZeroed(userData[srcOff : srcOff+mSize]) {
				return fmt.Errorf("%s doesn't exist in %s, but it has non-zero value", m.Name, kType.Name)
			}

			continue
		}

		km := kType.Members[i]

		switch btf.UnderlyingType(m.Type).(type) {
		case *btf.Pointer:
			// If this is a pointer → resolve struct_ops program.
			psKey := kType.Name + ":" + m.Name
			for k, ps := range cl.coll.Programs {
				if ps.AttachTo == psKey {
					p, ok := cl.programs[k]
					if !ok || p == nil {
						return nil
					}
					if err := structOpsPopulateValue(km, kernVData[kTypeOff:], p); err != nil {
						return err
					}
				}
			}

		default:
			// Otherwise → memcpy the field contents.
			if err := structOpsCopyMember(m, km, userData, kernVData[kTypeOff:]); err != nil {
				return fmt.Errorf("field %s: %w", kType.Name, err)
			}
		}
	}

	// Populate the map explicitly and keep a reference on cl.programs.
	// This is necessary because we may inline fds into kernVData which
	// may become invalid if the GC frees them.
	if err := m.Put(uint32(0), kernVData); err != nil {
		return err
	}
	mapSpec.Contents = nil
	runtime.KeepAlive(cl.programs)

	return nil
}

// resolveKconfig resolves all variables declared in .kconfig and populates
// m.Contents. Does nothing if the given m.Contents is non-empty.
func resolveKconfig(m *MapSpec) error {
	ds, ok := m.Value.(*btf.Datasec)
	if !ok {
		return errors.New("map value is not a Datasec")
	}

	if platform.IsWindows {
		return fmt.Errorf(".kconfig: %w", internal.ErrNotSupportedOnOS)
	}

	type configInfo struct {
		offset uint32
		size   uint32
		typ    btf.Type
	}

	configs := make(map[string]configInfo)

	data := make([]byte, ds.Size)
	for _, vsi := range ds.Vars {
		v := vsi.Type.(*btf.Var)
		n := v.TypeName()

		switch n {
		case "LINUX_KERNEL_VERSION":
			if integer, ok := v.Type.(*btf.Int); !ok || integer.Size != 4 {
				return fmt.Errorf("variable %s must be a 32 bits integer, got %s", n, v.Type)
			}

			kv, err := linux.KernelVersion()
			if err != nil {
				return fmt.Errorf("getting kernel version: %w", err)
			}
			internal.NativeEndian.PutUint32(data[vsi.Offset:], kv.Kernel())

		case "LINUX_HAS_SYSCALL_WRAPPER":
			integer, ok := v.Type.(*btf.Int)
			if !ok {
				return fmt.Errorf("variable %s must be an integer, got %s", n, v.Type)
			}
			var value uint64 = 1
			if err := haveSyscallWrapper(); errors.Is(err, ErrNotSupported) {
				value = 0
			} else if err != nil {
				return fmt.Errorf("unable to derive a value for LINUX_HAS_SYSCALL_WRAPPER: %w", err)
			}

			if err := kconfig.PutInteger(data[vsi.Offset:], integer, value); err != nil {
				return fmt.Errorf("set LINUX_HAS_SYSCALL_WRAPPER: %w", err)
			}

		default: // Catch CONFIG_*.
			configs[n] = configInfo{
				offset: vsi.Offset,
				size:   vsi.Size,
				typ:    v.Type,
			}
		}
	}

	// We only parse kconfig file if a CONFIG_* variable was found.
	if len(configs) > 0 {
		f, err := linux.FindKConfig()
		if err != nil {
			return fmt.Errorf("cannot find a kconfig file: %w", err)
		}
		defer f.Close()

		filter := make(map[string]struct{}, len(configs))
		for config := range configs {
			filter[config] = struct{}{}
		}

		kernelConfig, err := kconfig.Parse(f, filter)
		if err != nil {
			return fmt.Errorf("cannot parse kconfig file: %w", err)
		}

		for n, info := range configs {
			value, ok := kernelConfig[n]
			if !ok {
				return fmt.Errorf("config option %q does not exist on this kernel", n)
			}

			err := kconfig.PutValue(data[info.offset:info.offset+info.size], info.typ, value)
			if err != nil {
				return fmt.Errorf("problem adding value for %s: %w", n, err)
			}
		}
	}

	m.Contents = []MapKV{{uint32(0), data}}

	return nil
}

// LoadCollection reads an object file and creates and loads its declared
// resources into the kernel.
//
// Omitting Collection.Close() during application shutdown is an error.
// See the package documentation for details around Map and Program lifecycle.
func LoadCollection(file string) (*Collection, error) {
	if platform.IsWindows {
		// This mirrors a check in efW.
		if ext := filepath.Ext(file); ext == ".sys" {
			return loadCollectionFromNativeImage(file)
		}
	}

	spec, err := LoadCollectionSpec(file)
	if err != nil {
		return nil, err
	}
	return NewCollection(spec)
}

// Assign the contents of a Collection to a struct.
//
// This function bridges functionality between bpf2go generated
// code and any functionality better implemented in Collection.
//
// 'to' must be a pointer to a struct. A field of the
// struct is updated with values from Programs or Maps if it
// has an `ebpf` tag and its type is *Program or *Map.
// The tag's value specifies the name of the program or map as
// found in the CollectionSpec.
//
//	struct {
//	    Foo     *ebpf.Program `ebpf:"xdp_foo"`
//	    Bar     *ebpf.Map     `ebpf:"bar_map"`
//	    Ignored int
//	}
//
// Returns an error if any of the eBPF objects can't be found, or
// if the same Map or Program is assigned multiple times.
//
// Ownership and Close()ing responsibility is transferred to `to`
// for any successful assigns. On error `to` is left in an undefined state.
func (coll *Collection) Assign(to any) error {
	assignedMaps := make(map[string]bool)
	assignedProgs := make(map[string]bool)
	assignedVars := make(map[string]bool)

	// Assign() only transfers already-loaded Maps and Programs. No extra
	// loading is done.
	getValue := func(typ reflect.Type, name string) (any, error) {
		switch typ {

		case reflect.TypeFor[*Program]():
			if p := coll.Programs[name]; p != nil {
				assignedProgs[name] = true
				return p, nil
			}
			return nil, fmt.Errorf("missing program %q", name)

		case reflect.TypeFor[*Map]():
			if m := coll.Maps[name]; m != nil {
				assignedMaps[name] = true
				return m, nil
			}
			return nil, fmt.Errorf("missing map %q", name)

		case reflect.TypeFor[*Variable]():
			if v := coll.Variables[name]; v != nil {
				assignedVars[name] = true
				return v, nil
			}
			return nil, fmt.Errorf("missing variable %q", name)

		default:
			return nil, fmt.Errorf("unsupported type %s", typ)
		}
	}

	if err := assignValues(to, getValue); err != nil {
		return fmt.Errorf("assign values: %w", err)
	}

	// Finalize ownership transfer
	for p := range assignedProgs {
		delete(coll.Programs, p)
	}
	for m := range assignedMaps {
		delete(coll.Maps, m)
	}
	for s := range assignedVars {
		delete(coll.Variables, s)
	}

	return nil
}

// Close frees all maps and programs associated with the collection.
//
// The collection mustn't be used afterwards.
func (coll *Collection) Close() {
	for _, prog := range coll.Programs {
		prog.Close()
	}
	for _, m := range coll.Maps {
		m.Close()
	}
}

// DetachMap removes the named map from the Collection.
//
// This means that a later call to Close() will not affect this map.
//
// Returns nil if no map of that name exists.
func (coll *Collection) DetachMap(name string) *Map {
	m := coll.Maps[name]
	delete(coll.Maps, name)
	return m
}

// DetachProgram removes the named program from the Collection.
//
// This means that a later call to Close() will not affect this program.
//
// Returns nil if no program of that name exists.
func (coll *Collection) DetachProgram(name string) *Program {
	p := coll.Programs[name]
	delete(coll.Programs, name)
	return p
}

// structField represents a struct field containing the ebpf struct tag.
type structField struct {
	reflect.StructField
	value reflect.Value
}

// ebpfFields extracts field names tagged with 'ebpf' from a struct type.
// Keep track of visited types to avoid infinite recursion.
func ebpfFields(structVal reflect.Value, visited map[reflect.Type]bool) ([]structField, error) {
	if visited == nil {
		visited = make(map[reflect.Type]bool)
	}

	structType := structVal.Type()
	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s is not a struct", structType)
	}

	if visited[structType] {
		return nil, fmt.Errorf("recursion on type %s", structType)
	}

	fields := make([]structField, 0, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		field := structField{structType.Field(i), structVal.Field(i)}

		// If the field is tagged, gather it and move on.
		name := field.Tag.Get("ebpf")
		if name != "" {
			fields = append(fields, field)
			continue
		}

		// If the field does not have an ebpf tag, but is a struct or a pointer
		// to a struct, attempt to gather its fields as well.
		var v reflect.Value
		switch field.Type.Kind() {
		case reflect.Pointer:
			if field.Type.Elem().Kind() != reflect.Struct {
				continue
			}

			if field.value.IsNil() {
				return nil, fmt.Errorf("nil pointer to %s", structType)
			}

			// Obtain the destination type of the pointer.
			v = field.value.Elem()

		case reflect.Struct:
			// Reference the value's type directly.
			v = field.value

		default:
			continue
		}

		inner, err := ebpfFields(v, visited)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		fields = append(fields, inner...)
	}

	return fields, nil
}

// assignValues attempts to populate all fields of 'to' tagged with 'ebpf'.
//
// getValue is called for every tagged field of 'to' and must return the value
// to be assigned to the field with the given typ and name.
func assignValues(to any, getValue func(typ reflect.Type, name string) (any, error)) error {
	if err := internal.IsNilPointer(to); err != nil {
		return err
	}

	toValue := reflect.ValueOf(to)
	if toValue.Type().Kind() != reflect.Pointer {
		return fmt.Errorf("%T is not a pointer to struct", to)
	}

	fields, err := ebpfFields(toValue.Elem(), nil)
	if err != nil {
		return err
	}

	type elem struct {
		// Either *Map or *Program
		typ  reflect.Type
		name string
	}

	assigned := make(map[elem]string)
	for _, field := range fields {
		// Get string value the field is tagged with.
		tag := field.Tag.Get("ebpf")
		if strings.Contains(tag, ",") {
			return fmt.Errorf("field %s: ebpf tag contains a comma", field.Name)
		}

		// Check if the eBPF object with the requested
		// type and tag was already assigned elsewhere.
		e := elem{field.Type, tag}
		if af := assigned[e]; af != "" {
			return fmt.Errorf("field %s: object %q was already assigned to %s", field.Name, tag, af)
		}

		// Get the eBPF object referred to by the tag.
		value, err := getValue(field.Type, tag)
		if err != nil {
			return fmt.Errorf("field %s: %w", field.Name, err)
		}

		if !field.value.CanSet() {
			return fmt.Errorf("field %s: can't set value", field.Name)
		}
		field.value.Set(reflect.ValueOf(value))

		assigned[e] = field.Name
	}

	return nil
}
