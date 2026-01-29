package wasm

import (
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
	"github.com/tetratelabs/wazero/internal/wasmdebug"
)

// ImportedFunctions returns the definitions of each imported function.
//
// Note: Unlike ExportedFunctions, there is no unique constraint on imports.
func (m *Module) ImportedFunctions() (ret []api.FunctionDefinition) {
	for i := uint32(0); i < m.ImportFunctionCount; i++ {
		ret = append(ret, m.FunctionDefinition(i))
	}
	return
}

// ExportedFunctions returns the definitions of each exported function.
func (m *Module) ExportedFunctions() map[string]api.FunctionDefinition {
	ret := map[string]api.FunctionDefinition{}
	for i := range m.ExportSection {
		exp := &m.ExportSection[i]
		if exp.Type == ExternTypeFunc {
			d := m.FunctionDefinition(exp.Index)
			ret[exp.Name] = d
		}
	}
	return ret
}

// FunctionDefinition returns the FunctionDefinition for the given `index`.
func (m *Module) FunctionDefinition(index Index) *FunctionDefinition {
	// TODO: function initialization is lazy, but bulk. Make it per function.
	m.buildFunctionDefinitions()
	return &m.FunctionDefinitionSection[index]
}

// buildFunctionDefinitions generates function metadata that can be parsed from
// the module. This must be called after all validation.
func (m *Module) buildFunctionDefinitions() {
	m.functionDefinitionSectionInitOnce.Do(m.buildFunctionDefinitionsOnce)
}

func (m *Module) buildFunctionDefinitionsOnce() {
	var moduleName string
	var functionNames NameMap
	var localNames, resultNames IndirectNameMap
	if m.NameSection != nil {
		moduleName = m.NameSection.ModuleName
		functionNames = m.NameSection.FunctionNames
		localNames = m.NameSection.LocalNames
		resultNames = m.NameSection.ResultNames
	}

	importCount := m.ImportFunctionCount
	m.FunctionDefinitionSection = make([]FunctionDefinition, importCount+uint32(len(m.FunctionSection)))

	importFuncIdx := Index(0)
	for i := range m.ImportSection {
		imp := &m.ImportSection[i]
		if imp.Type != ExternTypeFunc {
			continue
		}

		def := &m.FunctionDefinitionSection[importFuncIdx]
		def.importDesc = imp
		def.index = importFuncIdx
		def.Functype = &m.TypeSection[imp.DescFunc]
		importFuncIdx++
	}

	for codeIndex, typeIndex := range m.FunctionSection {
		code := &m.CodeSection[codeIndex]
		idx := importFuncIdx + Index(codeIndex)
		def := &m.FunctionDefinitionSection[idx]
		def.index = idx
		def.Functype = &m.TypeSection[typeIndex]
		def.goFunc = code.GoFunc
	}

	n, nLen := 0, len(functionNames)
	for i := range m.FunctionDefinitionSection {
		d := &m.FunctionDefinitionSection[i]
		// The function name section begins with imports, but can be sparse.
		// This keeps track of how far in the name section we've searched.
		funcIdx := d.index
		var funcName string
		for ; n < nLen; n++ {
			next := &functionNames[n]
			if next.Index > funcIdx {
				break // we have function names, but starting at a later index.
			} else if next.Index == funcIdx {
				funcName = next.Name
				break
			}
		}

		d.moduleName = moduleName
		d.name = funcName
		d.Debugname = wasmdebug.FuncName(moduleName, funcName, funcIdx)
		d.paramNames = paramNames(localNames, funcIdx, len(d.Functype.Params))
		d.resultNames = paramNames(resultNames, funcIdx, len(d.Functype.Results))

		for i := range m.ExportSection {
			e := &m.ExportSection[i]
			if e.Type == ExternTypeFunc && e.Index == funcIdx {
				d.exportNames = append(d.exportNames, e.Name)
			}
		}
	}
}

// FunctionDefinition implements api.FunctionDefinition
type FunctionDefinition struct {
	internalapi.WazeroOnlyType
	moduleName string
	index      Index
	name       string
	// Debugname is exported for testing purpose.
	Debugname string
	goFunc    interface{}
	// Functype is exported for testing purpose.
	Functype    *FunctionType
	importDesc  *Import
	exportNames []string
	paramNames  []string
	resultNames []string
}

// ModuleName implements the same method as documented on api.FunctionDefinition.
func (f *FunctionDefinition) ModuleName() string {
	return f.moduleName
}

// Index implements the same method as documented on api.FunctionDefinition.
func (f *FunctionDefinition) Index() uint32 {
	return f.index
}

// Name implements the same method as documented on api.FunctionDefinition.
func (f *FunctionDefinition) Name() string {
	return f.name
}

// DebugName implements the same method as documented on api.FunctionDefinition.
func (f *FunctionDefinition) DebugName() string {
	return f.Debugname
}

// Import implements the same method as documented on api.FunctionDefinition.
func (f *FunctionDefinition) Import() (moduleName, name string, isImport bool) {
	if f.importDesc != nil {
		importDesc := f.importDesc
		moduleName, name, isImport = importDesc.Module, importDesc.Name, true
	}
	return
}

// ExportNames implements the same method as documented on api.FunctionDefinition.
func (f *FunctionDefinition) ExportNames() []string {
	return f.exportNames
}

// GoFunction implements the same method as documented on api.FunctionDefinition.
func (f *FunctionDefinition) GoFunction() interface{} {
	return f.goFunc
}

// ParamTypes implements api.FunctionDefinition ParamTypes.
func (f *FunctionDefinition) ParamTypes() []ValueType {
	return f.Functype.Params
}

// ParamNames implements the same method as documented on api.FunctionDefinition.
func (f *FunctionDefinition) ParamNames() []string {
	return f.paramNames
}

// ResultTypes implements api.FunctionDefinition ResultTypes.
func (f *FunctionDefinition) ResultTypes() []ValueType {
	return f.Functype.Results
}

// ResultNames implements the same method as documented on api.FunctionDefinition.
func (f *FunctionDefinition) ResultNames() []string {
	return f.resultNames
}
