package typeutil

import (
	"go/types"
)

// Identical reports whether x and y are identical types.
// Unlike types.Identical, receivers of Signature types are not ignored.
func Identical(x, y types.Type) (ret bool) {
	if !types.Identical(x, y) {
		return false
	}
	sigX, ok := x.(*types.Signature)
	if !ok {
		return true
	}
	sigY, ok := y.(*types.Signature)
	if !ok {
		// should be impossible
		return true
	}
	if sigX.Recv() == sigY.Recv() {
		return true
	}
	if sigX.Recv() == nil || sigY.Recv() == nil {
		return false
	}
	return Identical(sigX.Recv().Type(), sigY.Recv().Type())
}
