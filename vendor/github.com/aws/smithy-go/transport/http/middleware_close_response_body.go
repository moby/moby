package http

import (
	"context"
	"io"

	"github.com/aws/smithy-go/logging"
	"github.com/aws/smithy-go/middleware"
)

// CloseResponseBody closes the HTTP response body. It leaves the body open only
// for a successful response whose payload is a caller-owned stream (isStreaming
// with a nil opErr); on error, or for a non-streaming response, it closes the
// body — an error response body is diagnostic, not a caller-owned stream.
func CloseResponseBody(ctx context.Context, resp *Response, isStreaming bool, opErr error) {
	if resp == nil || resp.Body == nil {
		return
	}
	if isStreaming && opErr == nil {
		return
	}

	// Drain to EOF before closing; a body closed while unread prevents
	// connection reuse.
	if _, copyErr := io.Copy(io.Discard, resp.Body); copyErr != nil {
		middleware.GetLogger(ctx).Logf(logging.Warn, "failed to discard remaining HTTP response body, this may affect connection reuse")
	}

	if closeErr := resp.Body.Close(); closeErr != nil {
		middleware.GetLogger(ctx).Logf(logging.Warn, "failed to close HTTP response body, this may affect connection reuse")
	}
}

// AddErrorCloseResponseBodyMiddleware adds the middleware to automatically
// close the response body of an operation request if the request response
// failed.
//
// Deprecated: generated operation deserializers now close the response body
// via CloseResponseBody, so this middleware is no longer used.
func AddErrorCloseResponseBodyMiddleware(stack *middleware.Stack) error {
	return stack.Deserialize.Insert(&errorCloseResponseBodyMiddleware{}, "OperationDeserializer", middleware.Before)
}

type errorCloseResponseBodyMiddleware struct{}

func (*errorCloseResponseBodyMiddleware) ID() string {
	return "ErrorCloseResponseBody"
}

func (m *errorCloseResponseBodyMiddleware) HandleDeserialize(
	ctx context.Context, input middleware.DeserializeInput, next middleware.DeserializeHandler,
) (
	output middleware.DeserializeOutput, metadata middleware.Metadata, err error,
) {
	out, metadata, err := next.HandleDeserialize(ctx, input)
	if err != nil {
		if resp, ok := out.RawResponse.(*Response); ok && resp != nil && resp.Body != nil {
			// Consume the full body to prevent TCP connection resets on some platforms
			_, _ = io.Copy(io.Discard, resp.Body)
			// Do not validate that the response closes successfully.
			resp.Body.Close()
		}
	}

	return out, metadata, err
}

// AddCloseResponseBodyMiddleware adds the middleware to automatically close
// the response body of an operation request, after the response had been
// deserialized.
//
// Deprecated: generated operation deserializers now close the response body
// via CloseResponseBody, so this middleware is no longer used.
func AddCloseResponseBodyMiddleware(stack *middleware.Stack) error {
	return stack.Deserialize.Insert(&closeResponseBody{}, "OperationDeserializer", middleware.Before)
}

type closeResponseBody struct{}

func (*closeResponseBody) ID() string {
	return "CloseResponseBody"
}

func (m *closeResponseBody) HandleDeserialize(
	ctx context.Context, input middleware.DeserializeInput, next middleware.DeserializeHandler,
) (
	output middleware.DeserializeOutput, metadata middleware.Metadata, err error,
) {
	out, metadata, err := next.HandleDeserialize(ctx, input)
	if err != nil {
		return out, metadata, err
	}

	if resp, ok := out.RawResponse.(*Response); ok {
		// Consume the full body to prevent TCP connection resets on some platforms
		_, copyErr := io.Copy(io.Discard, resp.Body)
		if copyErr != nil {
			middleware.GetLogger(ctx).Logf(logging.Warn, "failed to discard remaining HTTP response body, this may affect connection reuse")
		}

		closeErr := resp.Body.Close()
		if closeErr != nil {
			middleware.GetLogger(ctx).Logf(logging.Warn, "failed to close HTTP response body, this may affect connection reuse")
		}
	}

	return out, metadata, err
}
