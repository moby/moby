package wazevoapi

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"time"
)

// These consts are used various places in the wazevo implementations.
// Instead of defining them in each file, we define them here so that we can quickly iterate on
// debugging without spending "where do we have debug logging?" time.

// ----- Debug logging -----
// These consts must be disabled by default. Enable them only when debugging.

const (
	FrontEndLoggingEnabled = false
	SSALoggingEnabled      = false
	RegAllocLoggingEnabled = false
)

// ----- Output prints -----
// These consts must be disabled by default. Enable them only when debugging.

const (
	PrintSSA                                 = false
	PrintOptimizedSSA                        = false
	PrintSSAToBackendIRLowering              = false
	PrintRegisterAllocated                   = false
	PrintFinalizedMachineCode                = false
	PrintMachineCodeHexPerFunction           = printMachineCodeHexPerFunctionUnmodified || PrintMachineCodeHexPerFunctionDisassemblable //nolint
	printMachineCodeHexPerFunctionUnmodified = false
	// PrintMachineCodeHexPerFunctionDisassemblable prints the machine code while modifying the actual result
	// to make it disassemblable. This is useful when debugging the final machine code. See the places where this is used for detail.
	// When this is enabled, functions must not be called.
	PrintMachineCodeHexPerFunctionDisassemblable = false
)

// printTarget is the function index to print the machine code. This is used for debugging to print the machine code
// of a specific function.
const printTarget = -1

// PrintEnabledIndex returns true if the current function index is the print target.
func PrintEnabledIndex(ctx context.Context) bool {
	if printTarget == -1 {
		return true
	}
	return GetCurrentFunctionIndex(ctx) == printTarget
}

// ----- Validations -----
const (
	// SSAValidationEnabled enables the SSA validation. This is disabled by default since the operation is expensive.
	SSAValidationEnabled = false
)

// ----- Stack Guard Check -----
const (
	// StackGuardCheckEnabled enables the stack guard check to ensure that our stack bounds check works correctly.
	StackGuardCheckEnabled       = false
	StackGuardCheckGuardPageSize = 8096
)

// CheckStackGuardPage checks the given stack guard page is not corrupted.
func CheckStackGuardPage(s []byte) {
	for i := 0; i < StackGuardCheckGuardPageSize; i++ {
		if s[i] != 0 {
			panic(
				fmt.Sprintf("BUG: stack guard page is corrupted:\n\tguard_page=%s\n\tstack=%s",
					hex.EncodeToString(s[:StackGuardCheckGuardPageSize]),
					hex.EncodeToString(s[StackGuardCheckGuardPageSize:]),
				))
		}
	}
}

// ----- Deterministic compilation verifier -----

const (
	// DeterministicCompilationVerifierEnabled enables the deterministic compilation verifier. This is disabled by default
	// since the operation is expensive. But when in doubt, enable this to make sure the compilation is deterministic.
	DeterministicCompilationVerifierEnabled = false
	DeterministicCompilationVerifyingIter   = 5
)

type (
	verifierState struct {
		initialCompilationDone bool
		maybeRandomizedIndexes []int
		r                      *rand.Rand
		values                 map[string]string
	}
	verifierStateContextKey struct{}
	currentFunctionNameKey  struct{}
	currentFunctionIndexKey struct{}
)

// NewDeterministicCompilationVerifierContext creates a new context with the deterministic compilation verifier used per wasm.Module.
func NewDeterministicCompilationVerifierContext(ctx context.Context, localFunctions int) context.Context {
	maybeRandomizedIndexes := make([]int, localFunctions)
	for i := range maybeRandomizedIndexes {
		maybeRandomizedIndexes[i] = i
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return context.WithValue(ctx, verifierStateContextKey{}, &verifierState{
		r: r, maybeRandomizedIndexes: maybeRandomizedIndexes, values: map[string]string{},
	})
}

// DeterministicCompilationVerifierRandomizeIndexes randomizes the indexes for the deterministic compilation verifier.
// To get the randomized index, use DeterministicCompilationVerifierGetRandomizedLocalFunctionIndex.
func DeterministicCompilationVerifierRandomizeIndexes(ctx context.Context) {
	state := ctx.Value(verifierStateContextKey{}).(*verifierState)
	if !state.initialCompilationDone {
		// If this is the first attempt, we use the index as-is order.
		state.initialCompilationDone = true
		return
	}
	r := state.r
	r.Shuffle(len(state.maybeRandomizedIndexes), func(i, j int) {
		state.maybeRandomizedIndexes[i], state.maybeRandomizedIndexes[j] = state.maybeRandomizedIndexes[j], state.maybeRandomizedIndexes[i]
	})
}

// DeterministicCompilationVerifierGetRandomizedLocalFunctionIndex returns the randomized index for the given `index`
// which is assigned by DeterministicCompilationVerifierRandomizeIndexes.
func DeterministicCompilationVerifierGetRandomizedLocalFunctionIndex(ctx context.Context, index int) int {
	state := ctx.Value(verifierStateContextKey{}).(*verifierState)
	ret := state.maybeRandomizedIndexes[index]
	return ret
}

// VerifyOrSetDeterministicCompilationContextValue verifies that the `newValue` is the same as the previous value for the given `scope`
// and the current function name. If the previous value doesn't exist, it sets the value to the given `newValue`.
//
// If the verification fails, this prints the diff and exits the process.
func VerifyOrSetDeterministicCompilationContextValue(ctx context.Context, scope string, newValue string) {
	fn := ctx.Value(currentFunctionNameKey{}).(string)
	key := fn + ": " + scope
	verifierCtx := ctx.Value(verifierStateContextKey{}).(*verifierState)
	oldValue, ok := verifierCtx.values[key]
	if !ok {
		verifierCtx.values[key] = newValue
		return
	}
	if oldValue != newValue {
		fmt.Printf(
			`BUG: Deterministic compilation failed for function%s at scope="%s".

This is mostly due to (but might not be limited to):
	* Resetting ssa.Builder, backend.Compiler or frontend.Compiler, etc doens't work as expected, and the compilation has been affected by the previous iterations.
	* Using a map with non-deterministic iteration order.

---------- [old] ----------
%s

---------- [new] ----------
%s
`,
			fn, scope, oldValue, newValue,
		)
		os.Exit(1)
	}
}

// nolint
const NeedFunctionNameInContext = PrintSSA ||
	PrintOptimizedSSA ||
	PrintSSAToBackendIRLowering ||
	PrintRegisterAllocated ||
	PrintFinalizedMachineCode ||
	PrintMachineCodeHexPerFunction ||
	DeterministicCompilationVerifierEnabled ||
	PerfMapEnabled

// SetCurrentFunctionName sets the current function name to the given `functionName`.
func SetCurrentFunctionName(ctx context.Context, index int, functionName string) context.Context {
	ctx = context.WithValue(ctx, currentFunctionNameKey{}, functionName)
	ctx = context.WithValue(ctx, currentFunctionIndexKey{}, index)
	return ctx
}

// GetCurrentFunctionName returns the current function name.
func GetCurrentFunctionName(ctx context.Context) string {
	ret, _ := ctx.Value(currentFunctionNameKey{}).(string)
	return ret
}

// GetCurrentFunctionIndex returns the current function index.
func GetCurrentFunctionIndex(ctx context.Context) int {
	ret, _ := ctx.Value(currentFunctionIndexKey{}).(int)
	return ret
}
