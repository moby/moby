package wasm

import (
	"errors"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/wasmdebug"
)

type HostFuncExporter interface {
	ExportHostFunc(*HostFunc)
}

// HostFunc is a function with an inlined type, used for NewHostModule.
// Any corresponding FunctionType will be reused or added to the Module.
type HostFunc struct {
	// ExportName is the only value returned by api.FunctionDefinition.
	ExportName string

	// Name is equivalent to the same method on api.FunctionDefinition.
	Name string

	// ParamTypes is equivalent to the same method on api.FunctionDefinition.
	ParamTypes []ValueType

	// ParamNames is equivalent to the same method on api.FunctionDefinition.
	ParamNames []string

	// ResultTypes is equivalent to the same method on api.FunctionDefinition.
	ResultTypes []ValueType

	// ResultNames is equivalent to the same method on api.FunctionDefinition.
	ResultNames []string

	// Code is the equivalent function in the SectionIDCode.
	Code Code
}

// WithGoModuleFunc returns a copy of the function, replacing its Code.GoFunc.
func (f *HostFunc) WithGoModuleFunc(fn api.GoModuleFunc) *HostFunc {
	ret := *f
	ret.Code.GoFunc = fn
	return &ret
}

// NewHostModule is defined internally for use in WASI tests and to keep the code size in the root directory small.
func NewHostModule(
	moduleName string,
	exportNames []string,
	nameToHostFunc map[string]*HostFunc,
	enabledFeatures api.CoreFeatures,
) (m *Module, err error) {
	if moduleName != "" {
		m = &Module{NameSection: &NameSection{ModuleName: moduleName}}
	} else {
		return nil, errors.New("a module name must not be empty")
	}

	if exportCount := uint32(len(nameToHostFunc)); exportCount > 0 {
		m.ExportSection = make([]Export, 0, exportCount)
		m.Exports = make(map[string]*Export, exportCount)
		if err = addFuncs(m, exportNames, nameToHostFunc, enabledFeatures); err != nil {
			return
		}
	}

	m.IsHostModule = true
	// Uses the address of *wasm.Module as the module ID so that host functions can have each state per compilation.
	// Downside of this is that compilation cache on host functions (trampoline codes for Go functions and
	// Wasm codes for Wasm-implemented host functions) are not available and compiles each time. On the other hand,
	// compilation of host modules is not costly as it's merely small trampolines vs the real-world native Wasm binary.
	// TODO: refactor engines so that we can properly cache compiled machine codes for host modules.
	m.AssignModuleID([]byte(fmt.Sprintf("@@@@@@@@%p", m)), // @@@@@@@@ = any 8 bytes different from Wasm header.
		nil, false)
	return
}

func addFuncs(
	m *Module,
	exportNames []string,
	nameToHostFunc map[string]*HostFunc,
	enabledFeatures api.CoreFeatures,
) (err error) {
	if m.NameSection == nil {
		m.NameSection = &NameSection{}
	}
	moduleName := m.NameSection.ModuleName

	for _, k := range exportNames {
		hf := nameToHostFunc[k]
		if hf.Name == "" {
			hf.Name = k // default name to export name
		}
		switch hf.Code.GoFunc.(type) {
		case api.GoModuleFunction, api.GoFunction:
			continue // already parsed
		}

		// Resolve the code using reflection
		hf.ParamTypes, hf.ResultTypes, hf.Code, err = parseGoReflectFunc(hf.Code.GoFunc)
		if err != nil {
			return fmt.Errorf("func[%s.%s] %w", moduleName, k, err)
		}

		// Assign names to the function, if they exist.
		params := hf.ParamTypes
		if paramNames := hf.ParamNames; paramNames != nil {
			if paramNamesLen := len(paramNames); paramNamesLen != len(params) {
				return fmt.Errorf("func[%s.%s] has %d params, but %d params names", moduleName, k, paramNamesLen, len(params))
			}
		}

		results := hf.ResultTypes
		if resultNames := hf.ResultNames; resultNames != nil {
			if resultNamesLen := len(resultNames); resultNamesLen != len(results) {
				return fmt.Errorf("func[%s.%s] has %d results, but %d results names", moduleName, k, resultNamesLen, len(results))
			}
		}
	}

	funcCount := uint32(len(exportNames))
	m.NameSection.FunctionNames = make([]NameAssoc, 0, funcCount)
	m.FunctionSection = make([]Index, 0, funcCount)
	m.CodeSection = make([]Code, 0, funcCount)

	idx := Index(0)
	for _, name := range exportNames {
		hf := nameToHostFunc[name]
		debugName := wasmdebug.FuncName(moduleName, name, idx)
		typeIdx, typeErr := m.maybeAddType(hf.ParamTypes, hf.ResultTypes, enabledFeatures)
		if typeErr != nil {
			return fmt.Errorf("func[%s] %v", debugName, typeErr)
		}
		m.FunctionSection = append(m.FunctionSection, typeIdx)
		m.CodeSection = append(m.CodeSection, hf.Code)

		export := hf.ExportName
		m.ExportSection = append(m.ExportSection, Export{Type: ExternTypeFunc, Name: export, Index: idx})
		m.Exports[export] = &m.ExportSection[len(m.ExportSection)-1]
		m.NameSection.FunctionNames = append(m.NameSection.FunctionNames, NameAssoc{Index: idx, Name: hf.Name})

		if len(hf.ParamNames) > 0 {
			localNames := NameMapAssoc{Index: idx}
			for i, n := range hf.ParamNames {
				localNames.NameMap = append(localNames.NameMap, NameAssoc{Index: Index(i), Name: n})
			}
			m.NameSection.LocalNames = append(m.NameSection.LocalNames, localNames)
		}
		if len(hf.ResultNames) > 0 {
			resultNames := NameMapAssoc{Index: idx}
			for i, n := range hf.ResultNames {
				resultNames.NameMap = append(resultNames.NameMap, NameAssoc{Index: Index(i), Name: n})
			}
			m.NameSection.ResultNames = append(m.NameSection.ResultNames, resultNames)
		}
		idx++
	}
	return nil
}

func (m *Module) maybeAddType(params, results []ValueType, enabledFeatures api.CoreFeatures) (Index, error) {
	if len(results) > 1 {
		// Guard >1.0 feature multi-value
		if err := enabledFeatures.RequireEnabled(api.CoreFeatureMultiValue); err != nil {
			return 0, fmt.Errorf("multiple result types invalid as %v", err)
		}
	}
	for i := range m.TypeSection {
		t := &m.TypeSection[i]
		if t.EqualsSignature(params, results) {
			return Index(i), nil
		}
	}

	result := m.SectionElementCount(SectionIDType)
	m.TypeSection = append(m.TypeSection, FunctionType{Params: params, Results: results})
	return result, nil
}
