package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/internal/rand"
	"github.com/aws/aws-sdk-go-v2/internal/sdk"
	"github.com/aws/smithy-go/logging"
	"github.com/aws/smithy-go/middleware"
	smithyrand "github.com/aws/smithy-go/rand"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// ClientRequestID is a Smithy BuildMiddleware that will generate a unique ID for logical API operation
// invocation.
type ClientRequestID struct{}

// ID the identifier for the ClientRequestID
func (r *ClientRequestID) ID() string {
	return "ClientRequestID"
}

// HandleBuild attaches a unique operation invocation id for the operation to the request
func (r ClientRequestID) HandleBuild(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, fmt.Errorf("unknown transport type %T", req)
	}

	invocationID, err := smithyrand.NewUUID(rand.Reader).GetUUID()
	if err != nil {
		return out, metadata, err
	}

	const invocationIDHeader = "Amz-Sdk-Invocation-Id"
	req.Header[invocationIDHeader] = append(req.Header[invocationIDHeader][:0], invocationID)

	return next.HandleBuild(ctx, in)
}

// RecordResponseTiming records the response timing for the SDK client requests.
type RecordResponseTiming struct {
	// DisableClockSkewCorrection suppresses recording of clock skew observed
	// from the response, per the Clock Skew Correction SEP. Response timing is
	// still recorded.
	DisableClockSkewCorrection bool
}

// ID is the middleware identifier
func (a *RecordResponseTiming) ID() string {
	return "RecordResponseTiming"
}

// HandleDeserialize calculates response metadata and clock skew
func (a RecordResponseTiming) HandleDeserialize(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (
	out middleware.DeserializeOutput, metadata middleware.Metadata, err error,
) {
	requestAt := sdk.NowTime()
	out, metadata, err = next.HandleDeserialize(ctx, in)
	responseAt := sdk.NowTime()
	setResponseAt(&metadata, responseAt)

	var serverTime time.Time
	var hasAgeHeader bool

	switch resp := out.RawResponse.(type) {
	case *smithyhttp.Response:
		hasAgeHeader = len(resp.Header.Get("Age")) > 0
		respDateHeader := resp.Header.Get("Date")
		if len(respDateHeader) == 0 {
			break
		}
		var parseErr error
		serverTime, parseErr = smithyhttp.ParseTime(respDateHeader)
		if parseErr != nil {
			logger := middleware.GetLogger(ctx)
			logger.Logf(logging.Warn, "failed to parse response Date header value, got %v",
				parseErr.Error())
			break
		}
		setServerTime(&metadata, serverTime)
	}

	if !a.DisableClockSkewCorrection {
		if skew, ok := computeClockSkew(serverTime, requestAt, responseAt, hasAgeHeader); ok {
			setAttemptSkew(&metadata, skew)
		}
	}

	return out, metadata, err
}

// maxTrustedRequestDuration bounds how long a request may take before the SDK
// discards the skew measurement derived from its response. A slower round trip
// could only produce a signing failure if it pushed the timestamp outside the
// SigV4 validity window. See the Clock Skew Correction SEP.
const maxTrustedRequestDuration = 15 * time.Minute

// computeClockSkew derives a clock skew candidate from a response per the Clock
// Skew Correction SEP. It returns ok=false (no candidate) when the Date header
// was absent/unparseable (serverTime zero), the round trip exceeded the maximum
// trusted request duration, or the response was served from a cache (Age
// header present). Otherwise the skew is the difference between the server's
// Date and the midpoint of the request round trip.
func computeClockSkew(serverTime, requestAt, responseAt time.Time, hasAgeHeader bool) (time.Duration, bool) {
	if serverTime.IsZero() {
		return 0, false
	}

	if hasAgeHeader {
		return 0, false
	}

	elapsed := responseAt.Sub(requestAt)
	if elapsed > maxTrustedRequestDuration {
		return 0, false
	}

	midpoint := requestAt.Add(elapsed / 2)
	return serverTime.Sub(midpoint), true
}

type responseAtKey struct{}

// GetResponseAt returns the time response was received at.
func GetResponseAt(metadata middleware.Metadata) (v time.Time, ok bool) {
	v, ok = metadata.Get(responseAtKey{}).(time.Time)
	return v, ok
}

// setResponseAt sets the response time on the metadata.
func setResponseAt(metadata *middleware.Metadata, v time.Time) {
	metadata.Set(responseAtKey{}, v)
}

type serverTimeKey struct{}

// GetServerTime returns the server time for response.
func GetServerTime(metadata middleware.Metadata) (v time.Time, ok bool) {
	v, ok = metadata.Get(serverTimeKey{}).(time.Time)
	return v, ok
}

// setServerTime sets the server time on the metadata.
func setServerTime(metadata *middleware.Metadata, v time.Time) {
	metadata.Set(serverTimeKey{}, v)
}

type attemptSkewKey struct{}

// GetAttemptSkew returns Attempt clock skew for response from metadata.
func GetAttemptSkew(metadata middleware.Metadata) (v time.Duration, ok bool) {
	v, ok = metadata.Get(attemptSkewKey{}).(time.Duration)
	return v, ok
}

// setAttemptSkew sets the attempt clock skew on the metadata.
func setAttemptSkew(metadata *middleware.Metadata, v time.Duration) {
	metadata.Set(attemptSkewKey{}, v)
}

// AddClientRequestIDMiddleware adds ClientRequestID to the middleware stack
func AddClientRequestIDMiddleware(stack *middleware.Stack) error {
	return stack.Build.Add(&ClientRequestID{}, middleware.After)
}

// AddRecordResponseTiming adds RecordResponseTiming middleware to the
// middleware stack.
func AddRecordResponseTiming(stack *middleware.Stack) error {
	return stack.Deserialize.Add(&RecordResponseTiming{}, middleware.After)
}

// rawResponseKey is the accessor key used to store and access the
// raw response within the response metadata.
type rawResponseKey struct{}

// AddRawResponse middleware adds raw response on to the metadata
type AddRawResponse struct{}

// ID the identifier for the ClientRequestID
func (m *AddRawResponse) ID() string {
	return "AddRawResponseToMetadata"
}

// HandleDeserialize adds raw response on the middleware metadata
func (m AddRawResponse) HandleDeserialize(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (
	out middleware.DeserializeOutput, metadata middleware.Metadata, err error,
) {
	out, metadata, err = next.HandleDeserialize(ctx, in)
	metadata.Set(rawResponseKey{}, out.RawResponse)
	return out, metadata, err
}

// AddRawResponseToMetadata adds middleware to the middleware stack that
// store raw response on to the metadata.
func AddRawResponseToMetadata(stack *middleware.Stack) error {
	return stack.Deserialize.Add(&AddRawResponse{}, middleware.Before)
}

// GetRawResponse returns raw response set on metadata
func GetRawResponse(metadata middleware.Metadata) interface{} {
	return metadata.Get(rawResponseKey{})
}
