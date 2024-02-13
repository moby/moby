package retry

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddle "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/internal/sdk"
	"github.com/aws/smithy-go/logging"
	smithymiddle "github.com/aws/smithy-go/middleware"
	"github.com/aws/smithy-go/transport/http"
)

// RequestCloner is a function that can take an input request type and clone
// the request for use in a subsequent retry attempt.
type RequestCloner func(interface{}) interface{}

type retryMetadata struct {
	AttemptNum       int
	AttemptTime      time.Time
	MaxAttempts      int
	AttemptClockSkew time.Duration
}

// Attempt is a Smithy Finalize middleware that handles retry attempts using
// the provided Retryer implementation.
type Attempt struct {
	// Enable the logging of retry attempts performed by the SDK. This will
	// include logging retry attempts, unretryable errors, and when max
	// attempts are reached.
	LogAttempts bool

	retryer       aws.RetryerV2
	requestCloner RequestCloner
}

// NewAttemptMiddleware returns a new Attempt retry middleware.
func NewAttemptMiddleware(retryer aws.Retryer, requestCloner RequestCloner, optFns ...func(*Attempt)) *Attempt {
	m := &Attempt{
		retryer:       wrapAsRetryerV2(retryer),
		requestCloner: requestCloner,
	}
	for _, fn := range optFns {
		fn(m)
	}
	return m
}

// ID returns the middleware identifier
func (r *Attempt) ID() string { return "Retry" }

func (r Attempt) logf(logger logging.Logger, classification logging.Classification, format string, v ...interface{}) {
	if !r.LogAttempts {
		return
	}
	logger.Logf(classification, format, v...)
}

// HandleFinalize utilizes the provider Retryer implementation to attempt
// retries over the next handler
func (r *Attempt) HandleFinalize(ctx context.Context, in smithymiddle.FinalizeInput, next smithymiddle.FinalizeHandler) (
	out smithymiddle.FinalizeOutput, metadata smithymiddle.Metadata, err error,
) {
	var attemptNum int
	var attemptClockSkew time.Duration
	var attemptResults AttemptResults

	maxAttempts := r.retryer.MaxAttempts()
	releaseRetryToken := nopRelease

	for {
		attemptNum++
		attemptInput := in
		attemptInput.Request = r.requestCloner(attemptInput.Request)

		// Record the metadata for the for attempt being started.
		attemptCtx := setRetryMetadata(ctx, retryMetadata{
			AttemptNum:       attemptNum,
			AttemptTime:      sdk.NowTime().UTC(),
			MaxAttempts:      maxAttempts,
			AttemptClockSkew: attemptClockSkew,
		})

		var attemptResult AttemptResult
		out, attemptResult, releaseRetryToken, err = r.handleAttempt(attemptCtx, attemptInput, releaseRetryToken, next)
		attemptClockSkew, _ = awsmiddle.GetAttemptSkew(attemptResult.ResponseMetadata)

		// AttemptResult Retried states that the attempt was not successful, and
		// should be retried.
		shouldRetry := attemptResult.Retried

		// Add attempt metadata to list of all attempt metadata
		attemptResults.Results = append(attemptResults.Results, attemptResult)

		if !shouldRetry {
			// Ensure the last response's metadata is used as the bases for result
			// metadata returned by the stack. The Slice of attempt results
			// will be added to this cloned metadata.
			metadata = attemptResult.ResponseMetadata.Clone()

			break
		}
	}

	addAttemptResults(&metadata, attemptResults)
	return out, metadata, err
}

// handleAttempt handles an individual request attempt.
func (r *Attempt) handleAttempt(
	ctx context.Context, in smithymiddle.FinalizeInput, releaseRetryToken func(error) error, next smithymiddle.FinalizeHandler,
) (
	out smithymiddle.FinalizeOutput, attemptResult AttemptResult, _ func(error) error, err error,
) {
	defer func() {
		attemptResult.Err = err
	}()

	// Short circuit if this attempt never can succeed because the context is
	// canceled. This reduces the chance of token pools being modified for
	// attempts that will not be made
	select {
	case <-ctx.Done():
		return out, attemptResult, nopRelease, ctx.Err()
	default:
	}

	//------------------------------
	// Get Attempt Token
	//------------------------------
	releaseAttemptToken, err := r.retryer.GetAttemptToken(ctx)
	if err != nil {
		return out, attemptResult, nopRelease, fmt.Errorf(
			"failed to get retry Send token, %w", err)
	}

	//------------------------------
	// Send Attempt
	//------------------------------
	logger := smithymiddle.GetLogger(ctx)
	service, operation := awsmiddle.GetServiceID(ctx), awsmiddle.GetOperationName(ctx)
	retryMetadata, _ := getRetryMetadata(ctx)
	attemptNum := retryMetadata.AttemptNum
	maxAttempts := retryMetadata.MaxAttempts

	// Following attempts must ensure the request payload stream starts in a
	// rewound state.
	if attemptNum > 1 {
		if rewindable, ok := in.Request.(interface{ RewindStream() error }); ok {
			if rewindErr := rewindable.RewindStream(); rewindErr != nil {
				return out, attemptResult, nopRelease, fmt.Errorf(
					"failed to rewind transport stream for retry, %w", rewindErr)
			}
		}

		r.logf(logger, logging.Debug, "retrying request %s/%s, attempt %d",
			service, operation, attemptNum)
	}

	var metadata smithymiddle.Metadata
	out, metadata, err = next.HandleFinalize(ctx, in)
	attemptResult.ResponseMetadata = metadata

	//------------------------------
	// Bookkeeping
	//------------------------------
	// Release the retry token based on the state of the attempt's error (if any).
	if releaseError := releaseRetryToken(err); releaseError != nil && err != nil {
		return out, attemptResult, nopRelease, fmt.Errorf(
			"failed to release retry token after request error, %w", err)
	}
	// Release the attempt token based on the state of the attempt's error (if any).
	if releaseError := releaseAttemptToken(err); releaseError != nil && err != nil {
		return out, attemptResult, nopRelease, fmt.Errorf(
			"failed to release initial token after request error, %w", err)
	}
	// If there was no error making the attempt, nothing further to do. There
	// will be nothing to retry.
	if err == nil {
		return out, attemptResult, nopRelease, err
	}

	//------------------------------
	// Is Retryable and Should Retry
	//------------------------------
	// If the attempt failed with an unretryable error, nothing further to do
	// but return, and inform the caller about the terminal failure.
	retryable := r.retryer.IsErrorRetryable(err)
	if !retryable {
		r.logf(logger, logging.Debug, "request failed with unretryable error %v", err)
		return out, attemptResult, nopRelease, err
	}

	// set retryable to true
	attemptResult.Retryable = true

	// Once the maximum number of attempts have been exhausted there is nothing
	// further to do other than inform the caller about the terminal failure.
	if maxAttempts > 0 && attemptNum >= maxAttempts {
		r.logf(logger, logging.Debug, "max retry attempts exhausted, max %d", maxAttempts)
		err = &MaxAttemptsError{
			Attempt: attemptNum,
			Err:     err,
		}
		return out, attemptResult, nopRelease, err
	}

	//------------------------------
	// Get Retry (aka Retry Quota) Token
	//------------------------------
	// Get a retry token that will be released after the
	releaseRetryToken, retryTokenErr := r.retryer.GetRetryToken(ctx, err)
	if retryTokenErr != nil {
		return out, attemptResult, nopRelease, retryTokenErr
	}

	//------------------------------
	// Retry Delay and Sleep
	//------------------------------
	// Get the retry delay before another attempt can be made, and sleep for
	// that time. Potentially early exist if the sleep is canceled via the
	// context.
	retryDelay, reqErr := r.retryer.RetryDelay(attemptNum, err)
	if reqErr != nil {
		return out, attemptResult, releaseRetryToken, reqErr
	}
	if reqErr = sdk.SleepWithContext(ctx, retryDelay); reqErr != nil {
		err = &aws.RequestCanceledError{Err: reqErr}
		return out, attemptResult, releaseRetryToken, err
	}

	// The request should be re-attempted.
	attemptResult.Retried = true

	return out, attemptResult, releaseRetryToken, err
}

// MetricsHeader attaches SDK request metric header for retries to the transport
type MetricsHeader struct{}

// ID returns the middleware identifier
func (r *MetricsHeader) ID() string {
	return "RetryMetricsHeader"
}

// HandleFinalize attaches the SDK request metric header to the transport layer
func (r MetricsHeader) HandleFinalize(ctx context.Context, in smithymiddle.FinalizeInput, next smithymiddle.FinalizeHandler) (
	out smithymiddle.FinalizeOutput, metadata smithymiddle.Metadata, err error,
) {
	retryMetadata, _ := getRetryMetadata(ctx)

	const retryMetricHeader = "Amz-Sdk-Request"
	var parts []string

	parts = append(parts, "attempt="+strconv.Itoa(retryMetadata.AttemptNum))
	if retryMetadata.MaxAttempts != 0 {
		parts = append(parts, "max="+strconv.Itoa(retryMetadata.MaxAttempts))
	}

	var ttl time.Time
	if deadline, ok := ctx.Deadline(); ok {
		ttl = deadline
	}

	// Only append the TTL if it can be determined.
	if !ttl.IsZero() && retryMetadata.AttemptClockSkew > 0 {
		const unixTimeFormat = "20060102T150405Z"
		ttl = ttl.Add(retryMetadata.AttemptClockSkew)
		parts = append(parts, "ttl="+ttl.Format(unixTimeFormat))
	}

	switch req := in.Request.(type) {
	case *http.Request:
		req.Header[retryMetricHeader] = append(req.Header[retryMetricHeader][:0], strings.Join(parts, "; "))
	default:
		return out, metadata, fmt.Errorf("unknown transport type %T", req)
	}

	return next.HandleFinalize(ctx, in)
}

type retryMetadataKey struct{}

// getRetryMetadata retrieves retryMetadata from the context and a bool
// indicating if it was set.
//
// Scoped to stack values. Use github.com/aws/smithy-go/middleware#ClearStackValues
// to clear all stack values.
func getRetryMetadata(ctx context.Context) (metadata retryMetadata, ok bool) {
	metadata, ok = smithymiddle.GetStackValue(ctx, retryMetadataKey{}).(retryMetadata)
	return metadata, ok
}

// setRetryMetadata sets the retryMetadata on the context.
//
// Scoped to stack values. Use github.com/aws/smithy-go/middleware#ClearStackValues
// to clear all stack values.
func setRetryMetadata(ctx context.Context, metadata retryMetadata) context.Context {
	return smithymiddle.WithStackValue(ctx, retryMetadataKey{}, metadata)
}

// AddRetryMiddlewaresOptions is the set of options that can be passed to
// AddRetryMiddlewares for configuring retry associated middleware.
type AddRetryMiddlewaresOptions struct {
	Retryer aws.Retryer

	// Enable the logging of retry attempts performed by the SDK. This will
	// include logging retry attempts, unretryable errors, and when max
	// attempts are reached.
	LogRetryAttempts bool
}

// AddRetryMiddlewares adds retry middleware to operation middleware stack
func AddRetryMiddlewares(stack *smithymiddle.Stack, options AddRetryMiddlewaresOptions) error {
	attempt := NewAttemptMiddleware(options.Retryer, http.RequestCloner, func(middleware *Attempt) {
		middleware.LogAttempts = options.LogRetryAttempts
	})

	if err := stack.Finalize.Add(attempt, smithymiddle.After); err != nil {
		return err
	}
	if err := stack.Finalize.Add(&MetricsHeader{}, smithymiddle.After); err != nil {
		return err
	}
	return nil
}
