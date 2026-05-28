package wasm

import (
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/internalapi"
)

// constantGlobal wraps GlobalInstance to implement api.Global.
type constantGlobal struct {
	internalapi.WazeroOnlyType
	g *GlobalInstance
}

// Type implements api.Global.
func (g constantGlobal) Type() api.ValueType {
	return g.g.Type.ValType
}

// Get implements api.Global.
func (g constantGlobal) Get() uint64 {
	ret, _ := g.g.Value()
	return ret
}

// String implements api.Global.
func (g constantGlobal) String() string {
	return g.g.String()
}

// mutableGlobal extends constantGlobal to allow updates.
type mutableGlobal struct {
	internalapi.WazeroOnlyType
	g *GlobalInstance
}

// Type implements api.Global.
func (g mutableGlobal) Type() api.ValueType {
	return g.g.Type.ValType
}

// Get implements api.Global.
func (g mutableGlobal) Get() uint64 {
	ret, _ := g.g.Value()
	return ret
}

// String implements api.Global.
func (g mutableGlobal) String() string {
	return g.g.String()
}

// Set implements the same method as documented on api.MutableGlobal.
func (g mutableGlobal) Set(v uint64) {
	g.g.SetValue(v, 0)
}
