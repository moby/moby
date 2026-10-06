// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag/pools"
)

// validatorPools recycles the objects allocated while validating.
//
// Validation allocates a validator per schema node and a result per check, so
// the same handful of types are built and thrown away constantly. Recycling
// them keeps validating a large specification affordable.
//
// Build with the "poolsdebug" tag to have every borrow and redeem tracked:
// misuse then panics where it happens rather than corrupting a pool, and
// [pools.AssertNoLeaks] reports what was borrowed and never given back.
var validatorPools allPools

func init() {
	resetPools()
}

// resetPools builds a fresh set of pools.
//
// Recycling an object twice leaves a pool holding it twice, and it would then
// be handed to two borrowers at once. A test that provokes such misuse has to
// start the next one from clean pools.
func resetPools() {
	validatorPools = allPools{
		schemaValidators:      pools.New[SchemaValidator](),
		objectValidators:      pools.New[objectValidator](),
		sliceValidators:       pools.New[schemaSliceValidator](),
		itemsValidators:       pools.New[itemsValidator](),
		basicCommonValidators: pools.New[basicCommonValidator](),
		headerValidators:      pools.New[HeaderValidator](),
		paramValidators:       pools.New[ParamValidator](),
		basicSliceValidators:  pools.New[basicSliceValidator](),
		numberValidators:      pools.New[numberValidator](),
		stringValidators:      pools.New[stringValidator](),
		schemaPropsValidators: pools.New[schemaPropsValidator](),
		formatValidators:      pools.New[formatValidator](),
		typeValidators:        pools.New[typeValidator](),
		schemas:               pools.New[spec.Schema](),
		results:               pools.New[Result](),
	}
}

// allPools is the set of pools shared by the validators of this package.
type allPools struct {
	schemaValidators      *pools.Pool[SchemaValidator]
	objectValidators      *pools.Pool[objectValidator]
	sliceValidators       *pools.Pool[schemaSliceValidator]
	itemsValidators       *pools.Pool[itemsValidator]
	basicCommonValidators *pools.Pool[basicCommonValidator]
	headerValidators      *pools.Pool[HeaderValidator]
	paramValidators       *pools.Pool[ParamValidator]
	basicSliceValidators  *pools.Pool[basicSliceValidator]
	numberValidators      *pools.Pool[numberValidator]
	stringValidators      *pools.Pool[stringValidator]
	schemaPropsValidators *pools.Pool[schemaPropsValidator]
	formatValidators      *pools.Pool[formatValidator]
	typeValidators        *pools.Pool[typeValidator]
	schemas               *pools.Pool[spec.Schema]
	results               *pools.Pool[Result]
}

// redeemResult returns a result to the pool.
//
// emptyResult is a shared value that was never borrowed, so it is not the
// pool's to take back: handing it over would be reported as a foreign redeem,
// rightly.
//
// Results are borrowed straight from the pool rather than through a helper,
// so that the instrumented build attributes a leak to the code that borrowed it.
//
// This wrapper costs that attribution on redeem, where a double redeem still
// names the offending call site in the panic it raises.
func redeemResult(r *Result) {
	if r == emptyResult {
		return
	}

	validatorPools.results.Redeem(r)
}
