package btf

import (
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"math"
	"os"
	"reflect"
	"slices"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/sys"
)

const btfMagic = 0xeB9F

// Errors returned by BTF functions.
var (
	ErrNotSupported    = internal.ErrNotSupported
	ErrNotFound        = errors.New("not found")
	ErrNoExtendedInfo  = errors.New("no extended info")
	ErrMultipleMatches = errors.New("multiple matching types")
)

// ID represents the unique ID of a BTF object.
type ID = sys.BTFID

type elfData struct {
	sectionSizes  map[string]uint32
	symbolOffsets map[elfSymbol]uint32
	fixups        map[Type]bool
}

type elfSymbol struct {
	section string
	name    string
}

// Spec allows querying a set of Types and loading the set into the
// kernel.
type Spec struct {
	*decoder

	// Additional data from ELF, may be nil.
	elf *elfData
}

// LoadSpec opens file and calls LoadSpecFromReader on it.
func LoadSpec(file string) (*Spec, error) {
	fh, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	return LoadSpecFromReader(fh)
}

// LoadSpecFromReader reads from an ELF or a raw BTF blob.
//
// Returns ErrNotFound if reading from an ELF which contains no BTF. ExtInfos
// may be nil.
func LoadSpecFromReader(rd io.ReaderAt) (*Spec, error) {
	file, err := internal.NewSafeELFFile(rd)
	if err != nil {
		raw, err := io.ReadAll(io.NewSectionReader(rd, 0, math.MaxInt64))
		if err != nil {
			return nil, fmt.Errorf("read raw BTF: %w", err)
		}

		return loadRawSpec(raw, nil)
	}

	return loadSpecFromELF(file)
}

// LoadSpecAndExtInfosFromReader reads from an ELF.
//
// ExtInfos may be nil if the ELF doesn't contain section metadata.
// Returns ErrNotFound if the ELF contains no BTF.
func LoadSpecAndExtInfosFromReader(rd io.ReaderAt) (*Spec, *ExtInfos, error) {
	file, err := internal.NewSafeELFFile(rd)
	if err != nil {
		return nil, nil, err
	}

	spec, err := loadSpecFromELF(file)
	if err != nil {
		return nil, nil, err
	}

	extInfos, err := loadExtInfosFromELF(file, spec)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, nil, err
	}

	return spec, extInfos, nil
}

// symbolOffsets extracts all symbols offsets from an ELF and indexes them by
// section and variable name.
//
// References to variables in BTF data sections carry unsigned 32-bit offsets.
// Some ELF symbols (e.g. in vmlinux) may point to virtual memory that is well
// beyond this range. Since these symbols cannot be described by BTF info,
// ignore them here.
func symbolOffsets(file *internal.SafeELFFile) (map[elfSymbol]uint32, error) {
	symbols, err := file.Symbols()
	if err != nil {
		return nil, fmt.Errorf("can't read symbols: %v", err)
	}

	offsets := make(map[elfSymbol]uint32)
	for _, sym := range symbols {
		if idx := sym.Section; idx >= elf.SHN_LORESERVE && idx <= elf.SHN_HIRESERVE {
			// Ignore things like SHN_ABS
			continue
		}

		if sym.Value > math.MaxUint32 {
			// VarSecinfo offset is u32, cannot reference symbols in higher regions.
			continue
		}

		if int(sym.Section) >= len(file.Sections) {
			return nil, fmt.Errorf("symbol %s: invalid section %d", sym.Name, sym.Section)
		}

		secName := file.Sections[sym.Section].Name
		offsets[elfSymbol{secName, sym.Name}] = uint32(sym.Value)
	}

	return offsets, nil
}

func loadSpecFromELF(file *internal.SafeELFFile) (*Spec, error) {
	var (
		btfSection   *elf.Section
		sectionSizes = make(map[string]uint32)
	)

	for _, sec := range file.Sections {
		switch sec.Name {
		case ".BTF":
			btfSection = sec
		default:
			if sec.Type != elf.SHT_PROGBITS && sec.Type != elf.SHT_NOBITS {
				break
			}

			if sec.Size > math.MaxUint32 {
				return nil, fmt.Errorf("section %s exceeds maximum size", sec.Name)
			}

			sectionSizes[sec.Name] = uint32(sec.Size)
		}
	}

	if btfSection == nil {
		return nil, fmt.Errorf("btf: %w", ErrNotFound)
	}

	offsets, err := symbolOffsets(file)
	if err != nil {
		return nil, err
	}

	rawBTF, err := btfSection.Data()
	if err != nil {
		return nil, fmt.Errorf("reading .BTF section: %w", err)
	}

	spec, err := loadRawSpec(rawBTF, nil)
	if err != nil {
		return nil, err
	}

	if spec.decoder.byteOrder != file.ByteOrder {
		return nil, fmt.Errorf("BTF byte order %s does not match ELF byte order %s", spec.decoder.byteOrder, file.ByteOrder)
	}

	spec.elf = &elfData{
		sectionSizes,
		offsets,
		make(map[Type]bool),
	}

	return spec, nil
}

func loadRawSpec(btf []byte, base *Spec) (*Spec, error) {
	var (
		baseDecoder *decoder
		baseStrings *stringTable
		err         error
	)

	if base != nil {
		baseDecoder = base.decoder
		baseStrings = base.strings
	}

	header, _, bo, err := parseBTFHeader(btf)
	if err != nil {
		return nil, fmt.Errorf("parsing .BTF header: %v", err)
	}

	if header.HdrLen > uint32(len(btf)) {
		return nil, fmt.Errorf("BTF header length is out of bounds")
	}
	btf = btf[header.HdrLen:]

	if uint64(header.StringOff)+uint64(header.StringLen) > uint64(len(btf)) {
		return nil, fmt.Errorf("string table is out of bounds")
	}
	stringsSection := btf[header.StringOff : header.StringOff+header.StringLen]

	rawStrings, err := newStringTable(stringsSection, baseStrings)
	if err != nil {
		return nil, fmt.Errorf("read string section: %w", err)
	}

	if uint64(header.TypeOff)+uint64(header.TypeLen) > uint64(len(btf)) {
		return nil, fmt.Errorf("types section is out of bounds")
	}
	typesSection := btf[header.TypeOff : header.TypeOff+header.TypeLen]

	decoder, err := newDecoder(typesSection, bo, rawStrings, baseDecoder)
	if err != nil {
		return nil, err
	}

	return &Spec{decoder, nil}, nil
}

// fixupDatasec attempts to patch up missing info in Datasecs and its members by
// supplementing them with information from the ELF headers and symbol table.
func (elf *elfData) fixupDatasec(typ Type) error {
	if elf == nil {
		return nil
	}

	if ds, ok := typ.(*Datasec); ok {
		if elf.fixups[ds] {
			return nil
		}
		elf.fixups[ds] = true

		name := ds.Name

		// Some Datasecs are virtual and don't have corresponding ELF sections.
		switch name {
		case ".ksyms":
			// .ksyms describes forward declarations of kfunc signatures, as well as
			// references to kernel symbols.
			// Nothing to fix up, all sizes and offsets are 0.
			for _, vsi := range ds.Vars {
				switch t := vsi.Type.(type) {
				case *Func:
					continue
				case *Var:
					if _, ok := t.Type.(*Void); !ok {
						return fmt.Errorf("data section %s: expected %s to be *Void, not %T: %w", name, vsi.Type.TypeName(), vsi.Type, ErrNotSupported)
					}
				default:
					return fmt.Errorf("data section %s: expected to be either *btf.Func or *btf.Var, not %T: %w", name, vsi.Type, ErrNotSupported)
				}
			}

			return nil
		case ".kconfig":
			// .kconfig has a size of 0 and has all members' offsets set to 0.
			// Fix up all offsets and set the Datasec's size.
			if err := fixupDatasecLayout(ds); err != nil {
				return err
			}

			// Fix up extern to global linkage to avoid a BTF verifier error.
			for _, vsi := range ds.Vars {
				vsi.Type.(*Var).Linkage = GlobalVar
			}

			return nil
		}

		if ds.Size != 0 {
			return nil
		}

		ds.Size, ok = elf.sectionSizes[name]
		if !ok {
			return fmt.Errorf("data section %s: missing size", name)
		}

		for i := range ds.Vars {
			symName := ds.Vars[i].Type.TypeName()
			ds.Vars[i].Offset, ok = elf.symbolOffsets[elfSymbol{name, symName}]
			if !ok {
				return fmt.Errorf("data section %s: missing offset for symbol %s", name, symName)
			}
		}
	}

	return nil
}

// fixupDatasecLayout populates ds.Vars[].Offset according to var sizes and
// alignment. Calculate and set ds.Size.
func fixupDatasecLayout(ds *Datasec) error {
	var off uint32

	for i, vsi := range ds.Vars {
		v, ok := vsi.Type.(*Var)
		if !ok {
			return fmt.Errorf("member %d: unsupported type %T", i, vsi.Type)
		}

		size, err := Sizeof(v.Type)
		if err != nil {
			return fmt.Errorf("variable %s: getting size: %w", v.Name, err)
		}
		align, err := alignof(v.Type)
		if err != nil {
			return fmt.Errorf("variable %s: getting alignment: %w", v.Name, err)
		}

		// Align the current member based on the offset of the end of the previous
		// member and the alignment of the current member.
		off = internal.Align(off, uint32(align))

		ds.Vars[i].Offset = off

		off += uint32(size)
	}

	ds.Size = off

	return nil
}

// Copy a Spec.
//
// All contained types are duplicated while preserving any modifications made
// to them.
func (s *Spec) Copy() *Spec {
	if s == nil {
		return nil
	}

	cpy := &Spec{
		s.decoder.Copy(),
		nil,
	}

	if s.elf != nil {
		cpy.elf = &elfData{
			s.elf.sectionSizes,
			s.elf.symbolOffsets,
			maps.Clone(s.elf.fixups),
		}
	}

	return cpy
}

// TypeByID returns the BTF Type with the given type ID.
//
// Returns an error wrapping ErrNotFound if a Type with the given ID
// does not exist in the Spec.
func (s *Spec) TypeByID(id TypeID) (Type, error) {
	typ, err := s.decoder.TypeByID(id)
	if err != nil {
		return nil, fmt.Errorf("inflate type: %w", err)
	}

	if err := s.elf.fixupDatasec(typ); err != nil {
		return nil, err
	}

	return typ, nil
}

// TypeID returns the ID for a given Type.
//
// Returns an error wrapping [ErrNotFound] if the type isn't part of the Spec.
func (s *Spec) TypeID(typ Type) (TypeID, error) {
	return s.decoder.TypeID(typ)
}

// AnyTypesByName returns a list of BTF Types with the given name.
//
// If the BTF blob describes multiple compilation units like vmlinux, multiple
// Types with the same name and kind can exist, but might not describe the same
// data structure.
//
// Returns an error wrapping ErrNotFound if no matching Type exists in the Spec.
func (s *Spec) AnyTypesByName(name string) ([]Type, error) {
	types, err := s.TypesByName(newEssentialName(name))
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(types); i++ {
		// Match against the full name, not just the essential one
		// in case the type being looked up is a struct flavor.
		if types[i].TypeName() != name {
			types = slices.Delete(types, i, i+1)
			continue
		}

		if err := s.elf.fixupDatasec(types[i]); err != nil {
			return nil, err
		}
	}

	return types, nil
}

// AnyTypeByName returns a Type with the given name.
//
// Returns an error if multiple types of that name exist.
func (s *Spec) AnyTypeByName(name string) (Type, error) {
	types, err := s.AnyTypesByName(name)
	if err != nil {
		return nil, err
	}

	if len(types) > 1 {
		return nil, fmt.Errorf("found multiple types: %v", types)
	}

	return types[0], nil
}

// TypeByName searches for a Type with a specific name. Since multiple Types
// with the same name can exist, the parameter typ is taken to narrow down the
// search in case of a clash.
//
// typ must be a non-nil pointer to an implementation of a Type. On success, the
// address of the found Type will be copied to typ.
//
// Returns an error wrapping ErrNotFound if no matching Type exists in the Spec.
// Returns an error wrapping ErrMultipleTypes if multiple candidates are found.
func (s *Spec) TypeByName(name string, typ any) error {
	if err := internal.IsNilPointer(typ); err != nil {
		return fmt.Errorf("type argument: %w", err)
	}

	typeInterface := reflect.TypeFor[Type]()

	// typ may be **T or *Type
	typValue := reflect.ValueOf(typ)
	if typValue.Kind() != reflect.Pointer {
		return fmt.Errorf("%T is not a pointer", typ)
	}
	typPtr := typValue.Elem()
	if !typPtr.CanSet() {
		return fmt.Errorf("%T cannot be set", typ)
	}

	wanted := typPtr.Type()
	if wanted == typeInterface {
		// This is *Type. Unwrap the value's type.
		if typPtr.IsNil() {
			return fmt.Errorf("%T points to a nil Type", typ)
		}
		wanted = typPtr.Elem().Type()
	}

	if !wanted.AssignableTo(typeInterface) {
		return fmt.Errorf("%T does not satisfy Type interface", typ)
	}

	types, err := s.AnyTypesByName(name)
	if err != nil {
		return err
	}

	var candidate Type
	for _, typ := range types {
		if reflect.TypeOf(typ) != wanted {
			continue
		}

		if candidate != nil {
			return fmt.Errorf("type %s(%T): %w", name, typ, ErrMultipleMatches)
		}

		candidate = typ
	}

	if candidate == nil {
		return fmt.Errorf("%s %s: %w", wanted, name, ErrNotFound)
	}

	typPtr.Set(reflect.ValueOf(candidate))

	return nil
}

// LoadSplitSpec loads split BTF from the given file.
//
// Types from base are used to resolve references in the split BTF.
// The returned Spec only contains types from the split BTF, not from the base.
func LoadSplitSpec(file string, base *Spec) (*Spec, error) {
	fh, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	return LoadSplitSpecFromReader(fh, base)
}

// LoadSplitSpecFromReader loads split BTF from a reader.
//
// Types from base are used to resolve references in the split BTF.
// The returned Spec only contains types from the split BTF, not from the base.
func LoadSplitSpecFromReader(r io.ReaderAt, base *Spec) (*Spec, error) {
	raw, err := io.ReadAll(io.NewSectionReader(r, 0, math.MaxInt64))
	if err != nil {
		return nil, fmt.Errorf("read raw BTF: %w", err)
	}

	return loadRawSpec(raw, base)
}

// All iterates over all types.
func (s *Spec) All() iter.Seq2[Type, error] {
	return func(yield func(Type, error) bool) {
		for id := s.firstTypeID; ; id++ {
			typ, err := s.TypeByID(id)
			if errors.Is(err, ErrNotFound) {
				return
			} else if err != nil {
				yield(nil, err)
				return
			}

			// Skip declTags, during unmarshaling declTags become `Tags` fields of other types.
			// We keep them in the spec to avoid holes in the ID space, but for the purposes of
			// iteration, they are not useful to the user.
			if _, ok := typ.(*declTag); ok {
				continue
			}

			if !yield(typ, nil) {
				return
			}
		}
	}
}
