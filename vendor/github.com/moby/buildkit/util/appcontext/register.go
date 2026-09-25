package appcontext

import (
	"context"
)

type Initializer func(context.Context) context.Context

var inits []Initializer

// Register stores a new context initializer that runs on app context creation
func Register(f Initializer) {
	inits = append(inits, f)
}
