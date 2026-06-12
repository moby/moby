package client

import (
	"context"
	"errors"
	"slices"

	"github.com/moby/moby/api/types/registry"
)

// staticAuth creates a privilegeFn from the given registryAuth.
func staticAuth(registryAuth string) registry.RequestAuthConfig {
	return func(ctx context.Context) (string, error) {
		return registryAuth, nil
	}
}

var (
	// errNoMorePrivilegeFuncs is a sentinel error to detect when the list of
	// chained privilege-funcs is exhausted. It is returned by [chainPrivilegeFuncs].
	//
	// This error is not currently exported as we don't want to expose this feature
	// yet for alternative implementations.
	errNoMorePrivilegeFuncs = errors.New("no more privilege functions")

	// errTryNextPrivilegeFunc is a sentinel error to detect whether the same
	// privilegeFunc can be re-tried. It is returned by [chainPrivilegeFuncs].
	//
	// This error is not currently exported as we don't want to expose this feature
	// yet for alternative implementations.
	errTryNextPrivilegeFunc = errors.New("try next privilege function")
)

// ChainPrivilegeFuncs returns a PrivilegeFunc that wraps the given funcs.
// Each call tries the next func in order, returning its result if successful.
//
// It returns errTryNextPrivilegeFunc if more funcs are available; errNoMorePrivilegeFuncs
// when exhausted.
func ChainPrivilegeFuncs(funcs ...registry.RequestAuthConfig) registry.RequestAuthConfig {
	acFuncs := slices.DeleteFunc(slices.Clone(funcs), func(f registry.RequestAuthConfig) bool { return f == nil })

	var i int
	return func(ctx context.Context) (string, error) {
		if i >= len(acFuncs) {
			return "", errNoMorePrivilegeFuncs
		}
		ac, err := acFuncs[i](ctx)
		switch {
		case err == nil:
			// success; allow caller to retry if the list is not exhausted.
		case errors.Is(err, errTryNextPrivilegeFunc):
			// PrivilegeFunc is a chain; return without incrementing.
			return ac, err
		case errors.Is(err, errNoMorePrivilegeFuncs):
			// PrivilegeFunc is a chain and the list is exhausted.
			// Continue as if no error occurred.
		default:
			// terminal error; stop chain
			acFuncs = nil
			return ac, err
		}
		i++
		if i >= len(acFuncs) {
			return ac, nil
		}
		return ac, errTryNextPrivilegeFunc
	}
}

func getAuth(ctx context.Context, fn registry.RequestAuthConfig) (auth string, tryNext bool, _ error) {
	if fn == nil {
		return "", false, nil
	}
	auth, err := fn(ctx)
	if errors.Is(err, errTryNextPrivilegeFunc) {
		return auth, true, nil
	}
	// No error, errNoMorePrivilegeFuncs and any hard failure.
	return auth, false, err
}
