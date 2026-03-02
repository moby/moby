package binary

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func decodeTypeSection(enabledFeatures api.CoreFeatures, r *bytes.Reader) ([]wasm.FunctionType, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	result := make([]wasm.FunctionType, vs)
	for i := uint32(0); i < vs; i++ {
		if err = decodeFunctionType(enabledFeatures, r, &result[i]); err != nil {
			return nil, fmt.Errorf("read %d-th type: %v", i, err)
		}
	}
	return result, nil
}

// decodeImportSection decodes the decoded import segments plus the count per wasm.ExternType.
func decodeImportSection(
	r *bytes.Reader,
	memorySizer memorySizer,
	memoryLimitPages uint32,
	enabledFeatures api.CoreFeatures,
) (result []wasm.Import,
	perModule map[string][]*wasm.Import,
	funcCount, globalCount, memoryCount, tableCount wasm.Index, err error,
) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		err = fmt.Errorf("get size of vector: %w", err)
		return
	}

	perModule = make(map[string][]*wasm.Import)
	result = make([]wasm.Import, vs)
	for i := uint32(0); i < vs; i++ {
		imp := &result[i]
		if err = decodeImport(r, i, memorySizer, memoryLimitPages, enabledFeatures, imp); err != nil {
			return
		}
		switch imp.Type {
		case wasm.ExternTypeFunc:
			imp.IndexPerType = funcCount
			funcCount++
		case wasm.ExternTypeGlobal:
			imp.IndexPerType = globalCount
			globalCount++
		case wasm.ExternTypeMemory:
			imp.IndexPerType = memoryCount
			memoryCount++
		case wasm.ExternTypeTable:
			imp.IndexPerType = tableCount
			tableCount++
		}
		perModule[imp.Module] = append(perModule[imp.Module], imp)
	}
	return
}

func decodeFunctionSection(r *bytes.Reader) ([]uint32, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	result := make([]uint32, vs)
	for i := uint32(0); i < vs; i++ {
		if result[i], _, err = leb128.DecodeUint32(r); err != nil {
			return nil, fmt.Errorf("get type index: %w", err)
		}
	}
	return result, err
}

func decodeTableSection(r *bytes.Reader, enabledFeatures api.CoreFeatures) ([]wasm.Table, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("error reading size")
	}
	if vs > 1 {
		if err := enabledFeatures.RequireEnabled(api.CoreFeatureReferenceTypes); err != nil {
			return nil, fmt.Errorf("at most one table allowed in module as %w", err)
		}
	}

	ret := make([]wasm.Table, vs)
	for i := range ret {
		err = decodeTable(r, enabledFeatures, &ret[i])
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func decodeMemorySection(
	r *bytes.Reader,
	enabledFeatures api.CoreFeatures,
	memorySizer memorySizer,
	memoryLimitPages uint32,
) (*wasm.Memory, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("error reading size")
	}
	if vs > 1 {
		return nil, fmt.Errorf("at most one memory allowed in module, but read %d", vs)
	} else if vs == 0 {
		// memory count can be zero.
		return nil, nil
	}

	return decodeMemory(r, enabledFeatures, memorySizer, memoryLimitPages)
}

func decodeGlobalSection(r *bytes.Reader, enabledFeatures api.CoreFeatures) ([]wasm.Global, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	result := make([]wasm.Global, vs)
	for i := uint32(0); i < vs; i++ {
		if err = decodeGlobal(r, enabledFeatures, &result[i]); err != nil {
			return nil, fmt.Errorf("global[%d]: %w", i, err)
		}
	}
	return result, nil
}

func decodeExportSection(r *bytes.Reader) ([]wasm.Export, map[string]*wasm.Export, error) {
	vs, _, sizeErr := leb128.DecodeUint32(r)
	if sizeErr != nil {
		return nil, nil, fmt.Errorf("get size of vector: %v", sizeErr)
	}

	exportMap := make(map[string]*wasm.Export, vs)
	exportSection := make([]wasm.Export, vs)
	for i := wasm.Index(0); i < vs; i++ {
		export := &exportSection[i]
		err := decodeExport(r, export)
		if err != nil {
			return nil, nil, fmt.Errorf("read export: %w", err)
		}
		if _, ok := exportMap[export.Name]; ok {
			return nil, nil, fmt.Errorf("export[%d] duplicates name %q", i, export.Name)
		} else {
			exportMap[export.Name] = export
		}
	}
	return exportSection, exportMap, nil
}

func decodeStartSection(r *bytes.Reader) (*wasm.Index, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get function index: %w", err)
	}
	return &vs, nil
}

func decodeElementSection(r *bytes.Reader, enabledFeatures api.CoreFeatures) ([]wasm.ElementSegment, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	result := make([]wasm.ElementSegment, vs)
	for i := uint32(0); i < vs; i++ {
		if err = decodeElementSegment(r, enabledFeatures, &result[i]); err != nil {
			return nil, fmt.Errorf("read element: %w", err)
		}
	}
	return result, nil
}

func decodeCodeSection(r *bytes.Reader) ([]wasm.Code, error) {
	codeSectionStart := uint64(r.Len())
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	result := make([]wasm.Code, vs)
	for i := uint32(0); i < vs; i++ {
		err = decodeCode(r, codeSectionStart, &result[i])
		if err != nil {
			return nil, fmt.Errorf("read %d-th code segment: %v", i, err)
		}
	}
	return result, nil
}

func decodeDataSection(r *bytes.Reader, enabledFeatures api.CoreFeatures) ([]wasm.DataSegment, error) {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}

	result := make([]wasm.DataSegment, vs)
	for i := uint32(0); i < vs; i++ {
		if err = decodeDataSegment(r, enabledFeatures, &result[i]); err != nil {
			return nil, fmt.Errorf("read data segment: %w", err)
		}
	}
	return result, nil
}

func decodeDataCountSection(r *bytes.Reader) (count *uint32, err error) {
	v, _, err := leb128.DecodeUint32(r)
	if err != nil && err != io.EOF {
		// data count is optional, so EOF is fine.
		return nil, err
	}
	return &v, nil
}
