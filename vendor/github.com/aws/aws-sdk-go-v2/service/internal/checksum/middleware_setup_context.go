package checksum

import (
	"context"

	"github.com/aws/smithy-go/middleware"
)

// setupChecksumContext is the initial middleware that looks up the input
// used to configure checksum behavior. This middleware must be executed before
// input validation step or any other checksum middleware.
type setupInputContext struct {
	// GetAlgorithm is a function to get the checksum algorithm of the
	// input payload from the input parameters.
	//
	// Given the input parameter value, the function must return the algorithm
	// and true, or false if no algorithm is specified.
	GetAlgorithm func(interface{}) (string, bool)
}

// ID for the middleware
func (m *setupInputContext) ID() string {
	return "AWSChecksum:SetupInputContext"
}

// HandleInitialize initialization middleware that setups up the checksum
// context based on the input parameters provided in the stack.
func (m *setupInputContext) HandleInitialize(
	ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler,
) (
	out middleware.InitializeOutput, metadata middleware.Metadata, err error,
) {
	// Check if validation algorithm is specified.
	if m.GetAlgorithm != nil {
		// check is input resource has a checksum algorithm
		algorithm, ok := m.GetAlgorithm(in.Parameters)
		if ok && len(algorithm) != 0 {
			ctx = setContextInputAlgorithm(ctx, algorithm)
		}
	}

	return next.HandleInitialize(ctx, in)
}

// inputAlgorithmKey is the key set on context used to identify, retrieves the
// request checksum algorithm if present on the context.
type inputAlgorithmKey struct{}

// setContextInputAlgorithm sets the request checksum algorithm on the context.
//
// Scoped to stack values.
func setContextInputAlgorithm(ctx context.Context, value string) context.Context {
	return middleware.WithStackValue(ctx, inputAlgorithmKey{}, value)
}

// getContextInputAlgorithm returns the checksum algorithm from the context if
// one was specified. Empty string is returned if one is not specified.
//
// Scoped to stack values.
func getContextInputAlgorithm(ctx context.Context) (v string) {
	v, _ = middleware.GetStackValue(ctx, inputAlgorithmKey{}).(string)
	return v
}

type setupOutputContext struct {
	// GetValidationMode is a function to get the checksum validation
	// mode of the output payload from the input parameters.
	//
	// Given the input parameter value, the function must return the validation
	// mode and true, or false if no mode is specified.
	GetValidationMode func(interface{}) (string, bool)
}

// ID for the middleware
func (m *setupOutputContext) ID() string {
	return "AWSChecksum:SetupOutputContext"
}

// HandleInitialize initialization middleware that setups up the checksum
// context based on the input parameters provided in the stack.
func (m *setupOutputContext) HandleInitialize(
	ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler,
) (
	out middleware.InitializeOutput, metadata middleware.Metadata, err error,
) {
	// Check if validation mode is specified.
	if m.GetValidationMode != nil {
		// check is input resource has a checksum algorithm
		mode, ok := m.GetValidationMode(in.Parameters)
		if ok && len(mode) != 0 {
			ctx = setContextOutputValidationMode(ctx, mode)
		}
	}

	return next.HandleInitialize(ctx, in)
}

// outputValidationModeKey is the key set on context used to identify if
// output checksum validation is enabled.
type outputValidationModeKey struct{}

// setContextOutputValidationMode sets the request checksum
// algorithm on the context.
//
// Scoped to stack values.
func setContextOutputValidationMode(ctx context.Context, value string) context.Context {
	return middleware.WithStackValue(ctx, outputValidationModeKey{}, value)
}

// getContextOutputValidationMode returns response checksum validation state,
// if one was specified. Empty string is returned if one is not specified.
//
// Scoped to stack values.
func getContextOutputValidationMode(ctx context.Context) (v string) {
	v, _ = middleware.GetStackValue(ctx, outputValidationModeKey{}).(string)
	return v
}
