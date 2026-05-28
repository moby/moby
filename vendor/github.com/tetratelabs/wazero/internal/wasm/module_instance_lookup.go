package wasm

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
)

// LookupFunction looks up the table by the given index, and returns the api.Function implementation if found,
// otherwise this panics according to the same semantics as call_indirect instruction.
// Currently, this is only used by emscripten which needs to do call_indirect-like operation in the host function.
func (m *ModuleInstance) LookupFunction(t *TableInstance, typeId FunctionTypeID, tableOffset Index) api.Function {
	fm, index := m.Engine.LookupFunction(t, typeId, tableOffset)
	if source := fm.Source; source.IsHostModule {
		// This case, the found function is a host function stored in the table. Generally, Engine.NewFunction are only
		// responsible for calling Wasm-defined functions (not designed for calling Go functions!). Hence we need to wrap
		// the host function as a special case.
		def := &source.FunctionDefinitionSection[index]
		goF := source.CodeSection[index].GoFunc
		switch typed := goF.(type) {
		case api.GoFunction:
			// GoFunction doesn't need looked up module.
			return &lookedUpGoFunction{def: def, g: goFunctionAsGoModuleFunction(typed)}
		case api.GoModuleFunction:
			return &lookedUpGoFunction{def: def, lookedUpModule: m, g: typed}
		default:
			panic(fmt.Sprintf("unexpected GoFunc type: %T", goF))
		}
	} else {
		return fm.Engine.NewFunction(index)
	}
}

// lookedUpGoFunction implements lookedUpGoModuleFunction.
type lookedUpGoFunction struct {
	internalapi.WazeroOnly
	def *FunctionDefinition
	// lookedUpModule is the *ModuleInstance from which this Go function is looked up, i.e. owner of the table.
	lookedUpModule *ModuleInstance
	g              api.GoModuleFunction
}

// goFunctionAsGoModuleFunction converts api.GoFunction to api.GoModuleFunction which ignores the api.Module argument.
func goFunctionAsGoModuleFunction(g api.GoFunction) api.GoModuleFunction {
	return api.GoModuleFunc(func(ctx context.Context, _ api.Module, stack []uint64) {
		g.Call(ctx, stack)
	})
}

// Definition implements api.Function.
func (l *lookedUpGoFunction) Definition() api.FunctionDefinition { return l.def }

// Call implements api.Function.
func (l *lookedUpGoFunction) Call(ctx context.Context, params ...uint64) ([]uint64, error) {
	typ := l.def.Functype
	stackSize := typ.ParamNumInUint64
	rn := typ.ResultNumInUint64
	if rn > stackSize {
		stackSize = rn
	}
	stack := make([]uint64, stackSize)
	copy(stack, params)
	return stack[:rn], l.CallWithStack(ctx, stack)
}

// CallWithStack implements api.Function.
func (l *lookedUpGoFunction) CallWithStack(ctx context.Context, stack []uint64) error {
	// The Go host function always needs to access caller's module, in this case the one holding the table.
	l.g.Call(ctx, l.lookedUpModule, stack)
	return nil
}
