package checksum

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"strconv"
	"strings"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const (
	contentMD5Header                           = "Content-Md5"
	streamingUnsignedPayloadTrailerPayloadHash = "STREAMING-UNSIGNED-PAYLOAD-TRAILER"
)

// computedInputChecksumsKey is the metadata key for recording the algorithm the
// checksum was computed for and the checksum value.
type computedInputChecksumsKey struct{}

// GetComputedInputChecksums returns the map of checksum algorithm to their
// computed value stored in the middleware Metadata. Returns false if no values
// were stored in the Metadata.
func GetComputedInputChecksums(m middleware.Metadata) (map[string]string, bool) {
	vs, ok := m.Get(computedInputChecksumsKey{}).(map[string]string)
	return vs, ok
}

// SetComputedInputChecksums stores the map of checksum algorithm to their
// computed value in the middleware Metadata. Overwrites any values that
// currently exist in the metadata.
func SetComputedInputChecksums(m *middleware.Metadata, vs map[string]string) {
	m.Set(computedInputChecksumsKey{}, vs)
}

// computeInputPayloadChecksum middleware computes payload checksum
type computeInputPayloadChecksum struct {
	// Enables support for wrapping the serialized input payload with a
	// content-encoding: aws-check wrapper, and including a trailer for the
	// algorithm's checksum value.
	//
	// The checksum will not be computed, nor added as trailing checksum, if
	// the Algorithm's header is already set on the request.
	EnableTrailingChecksum bool

	// States that a checksum is required to be included for the operation. If
	// Input does not specify a checksum, fallback to built in MD5 checksum is
	// used.
	//
	// Replaces smithy-go's ContentChecksum middleware.
	RequireChecksum bool

	// Enables support for computing the SHA256 checksum of input payloads
	// along with the algorithm specified checksum. Prevents downstream
	// middleware handlers (computePayloadSHA256) re-reading the payload.
	//
	// The SHA256 payload hash will only be used for computed for requests
	// that are not TLS, or do not enable trailing checksums.
	//
	// The SHA256 payload hash will not be computed, if the Algorithm's header
	// is already set on the request.
	EnableComputePayloadHash bool

	// Enables support for setting the aws-chunked decoded content length
	// header for the decoded length of the underlying stream. Will only be set
	// when used with trailing checksums, and aws-chunked content-encoding.
	EnableDecodedContentLengthHeader bool

	buildHandlerRun        bool
	deferToFinalizeHandler bool
}

// ID provides the middleware's identifier.
func (m *computeInputPayloadChecksum) ID() string {
	return "AWSChecksum:ComputeInputPayloadChecksum"
}

type computeInputHeaderChecksumError struct {
	Msg string
	Err error
}

func (e computeInputHeaderChecksumError) Error() string {
	const intro = "compute input header checksum failed"

	if e.Err != nil {
		return fmt.Sprintf("%s, %s, %v", intro, e.Msg, e.Err)
	}

	return fmt.Sprintf("%s, %s", intro, e.Msg)
}
func (e computeInputHeaderChecksumError) Unwrap() error { return e.Err }

// HandleBuild handles computing the payload's checksum, in the following cases:
//   * Is HTTP, not HTTPS
//   * RequireChecksum is true, and no checksums were specified via the Input
//   * Trailing checksums are not supported
//
// The build handler must be inserted in the stack before ContentPayloadHash
// and after ComputeContentLength.
func (m *computeInputPayloadChecksum) HandleBuild(
	ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	m.buildHandlerRun = true

	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, computeInputHeaderChecksumError{
			Msg: fmt.Sprintf("unknown request type %T", req),
		}
	}

	var algorithm Algorithm
	var checksum string
	defer func() {
		if algorithm == "" || checksum == "" || err != nil {
			return
		}

		// Record the checksum and algorithm that was computed
		SetComputedInputChecksums(&metadata, map[string]string{
			string(algorithm): checksum,
		})
	}()

	// If no algorithm was specified, and the operation requires a checksum,
	// fallback to the legacy content MD5 checksum.
	algorithm, ok, err = getInputAlgorithm(ctx)
	if err != nil {
		return out, metadata, err
	} else if !ok {
		if m.RequireChecksum {
			checksum, err = setMD5Checksum(ctx, req)
			if err != nil {
				return out, metadata, computeInputHeaderChecksumError{
					Msg: "failed to compute stream's MD5 checksum",
					Err: err,
				}
			}
			algorithm = Algorithm("MD5")
		}
		return next.HandleBuild(ctx, in)
	}

	// If the checksum header is already set nothing to do.
	checksumHeader := AlgorithmHTTPHeader(algorithm)
	if checksum = req.Header.Get(checksumHeader); checksum != "" {
		return next.HandleBuild(ctx, in)
	}

	computePayloadHash := m.EnableComputePayloadHash
	if v := v4.GetPayloadHash(ctx); v != "" {
		computePayloadHash = false
	}

	stream := req.GetStream()
	streamLength, err := getRequestStreamLength(req)
	if err != nil {
		return out, metadata, computeInputHeaderChecksumError{
			Msg: "failed to determine stream length",
			Err: err,
		}
	}

	// If trailing checksums are supported, the request is HTTPS, and the
	// stream is not nil or empty, there is nothing to do in the build stage.
	// The checksum will be added to the request as a trailing checksum in the
	// finalize handler.
	//
	// Nil and empty streams will always be handled as a request header,
	// regardless if the operation supports trailing checksums or not.
	if strings.EqualFold(req.URL.Scheme, "https") {
		if stream != nil && streamLength != 0 && m.EnableTrailingChecksum {
			if m.EnableComputePayloadHash {
				// payload hash is set as header in Build middleware handler,
				// ContentSHA256Header.
				ctx = v4.SetPayloadHash(ctx, streamingUnsignedPayloadTrailerPayloadHash)
			}

			m.deferToFinalizeHandler = true
			return next.HandleBuild(ctx, in)
		}

		// If trailing checksums are not enabled but protocol is still HTTPS
		// disabling computing the payload hash. Downstream middleware  handler
		// (ComputetPayloadHash) will set the payload hash to unsigned payload,
		// if signing was used.
		computePayloadHash = false
	}

	// Only seekable streams are supported for non-trailing checksums, because
	// the stream needs to be rewound before the handler can continue.
	if stream != nil && !req.IsStreamSeekable() {
		return out, metadata, computeInputHeaderChecksumError{
			Msg: "unseekable stream is not supported without TLS and trailing checksum",
		}
	}

	var sha256Checksum string
	checksum, sha256Checksum, err = computeStreamChecksum(
		algorithm, stream, computePayloadHash)
	if err != nil {
		return out, metadata, computeInputHeaderChecksumError{
			Msg: "failed to compute stream checksum",
			Err: err,
		}
	}

	if err := req.RewindStream(); err != nil {
		return out, metadata, computeInputHeaderChecksumError{
			Msg: "failed to rewind stream",
			Err: err,
		}
	}

	req.Header.Set(checksumHeader, checksum)

	if computePayloadHash {
		ctx = v4.SetPayloadHash(ctx, sha256Checksum)
	}

	return next.HandleBuild(ctx, in)
}

type computeInputTrailingChecksumError struct {
	Msg string
	Err error
}

func (e computeInputTrailingChecksumError) Error() string {
	const intro = "compute input trailing checksum failed"

	if e.Err != nil {
		return fmt.Sprintf("%s, %s, %v", intro, e.Msg, e.Err)
	}

	return fmt.Sprintf("%s, %s", intro, e.Msg)
}
func (e computeInputTrailingChecksumError) Unwrap() error { return e.Err }

// HandleFinalize handles computing the payload's checksum, in the following cases:
//   * Is HTTPS, not HTTP
//   * A checksum was specified via the Input
//   * Trailing checksums are supported.
//
// The finalize handler must be inserted in the stack before Signing, and after Retry.
func (m *computeInputPayloadChecksum) HandleFinalize(
	ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
) (
	out middleware.FinalizeOutput, metadata middleware.Metadata, err error,
) {
	if !m.deferToFinalizeHandler {
		if !m.buildHandlerRun {
			return out, metadata, computeInputTrailingChecksumError{
				Msg: "build handler was removed without also removing finalize handler",
			}
		}
		return next.HandleFinalize(ctx, in)
	}

	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, computeInputTrailingChecksumError{
			Msg: fmt.Sprintf("unknown request type %T", req),
		}
	}

	// Trailing checksums are only supported when TLS is enabled.
	if !strings.EqualFold(req.URL.Scheme, "https") {
		return out, metadata, computeInputTrailingChecksumError{
			Msg: "HTTPS required",
		}
	}

	// If no algorithm was specified, there is nothing to do.
	algorithm, ok, err := getInputAlgorithm(ctx)
	if err != nil {
		return out, metadata, computeInputTrailingChecksumError{
			Msg: "failed to get algorithm",
			Err: err,
		}
	} else if !ok {
		return out, metadata, computeInputTrailingChecksumError{
			Msg: "no algorithm specified",
		}
	}

	// If the checksum header is already set before finalize could run, there
	// is nothing to do.
	checksumHeader := AlgorithmHTTPHeader(algorithm)
	if req.Header.Get(checksumHeader) != "" {
		return next.HandleFinalize(ctx, in)
	}

	stream := req.GetStream()
	streamLength, err := getRequestStreamLength(req)
	if err != nil {
		return out, metadata, computeInputTrailingChecksumError{
			Msg: "failed to determine stream length",
			Err: err,
		}
	}

	if stream == nil || streamLength == 0 {
		// Nil and empty streams are handled by the Build handler. They are not
		// supported by the trailing checksums finalize handler. There is no
		// benefit to sending them as trailers compared to headers.
		return out, metadata, computeInputTrailingChecksumError{
			Msg: "nil or empty streams are not supported",
		}
	}

	checksumReader, err := newComputeChecksumReader(stream, algorithm)
	if err != nil {
		return out, metadata, computeInputTrailingChecksumError{
			Msg: "failed to created checksum reader",
			Err: err,
		}
	}

	awsChunkedReader := newUnsignedAWSChunkedEncoding(checksumReader,
		func(o *awsChunkedEncodingOptions) {
			o.Trailers[AlgorithmHTTPHeader(checksumReader.Algorithm())] = awsChunkedTrailerValue{
				Get:    checksumReader.Base64Checksum,
				Length: checksumReader.Base64ChecksumLength(),
			}
			o.StreamLength = streamLength
		})

	for key, values := range awsChunkedReader.HTTPHeaders() {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Setting the stream on the request will create a copy. The content length
	// is not updated until after the request is copied to prevent impacting
	// upstream middleware.
	req, err = req.SetStream(awsChunkedReader)
	if err != nil {
		return out, metadata, computeInputTrailingChecksumError{
			Msg: "failed updating request to trailing checksum wrapped stream",
			Err: err,
		}
	}
	req.ContentLength = awsChunkedReader.EncodedLength()
	in.Request = req

	// Add decoded content length header if original stream's content length is known.
	if streamLength != -1 && m.EnableDecodedContentLengthHeader {
		req.Header.Set(decodedContentLengthHeaderName, strconv.FormatInt(streamLength, 10))
	}

	out, metadata, err = next.HandleFinalize(ctx, in)
	if err == nil {
		checksum, err := checksumReader.Base64Checksum()
		if err != nil {
			return out, metadata, fmt.Errorf("failed to get computed checksum, %w", err)
		}

		// Record the checksum and algorithm that was computed
		SetComputedInputChecksums(&metadata, map[string]string{
			string(algorithm): checksum,
		})
	}

	return out, metadata, err
}

func getInputAlgorithm(ctx context.Context) (Algorithm, bool, error) {
	ctxAlgorithm := getContextInputAlgorithm(ctx)
	if ctxAlgorithm == "" {
		return "", false, nil
	}

	algorithm, err := ParseAlgorithm(ctxAlgorithm)
	if err != nil {
		return "", false, fmt.Errorf(
			"failed to parse algorithm, %w", err)
	}

	return algorithm, true, nil
}

func computeStreamChecksum(algorithm Algorithm, stream io.Reader, computePayloadHash bool) (
	checksum string, sha256Checksum string, err error,
) {
	hasher, err := NewAlgorithmHash(algorithm)
	if err != nil {
		return "", "", fmt.Errorf(
			"failed to get hasher for checksum algorithm, %w", err)
	}

	var sha256Hasher hash.Hash
	var batchHasher io.Writer = hasher

	// Compute payload hash for the protocol. To prevent another handler
	// (computePayloadSHA256) re-reading body also compute the SHA256 for
	// request signing. If configured checksum algorithm is SHA256, don't
	// double wrap stream with another SHA256 hasher.
	if computePayloadHash && algorithm != AlgorithmSHA256 {
		sha256Hasher = sha256.New()
		batchHasher = io.MultiWriter(hasher, sha256Hasher)
	}

	if stream != nil {
		if _, err = io.Copy(batchHasher, stream); err != nil {
			return "", "", fmt.Errorf(
				"failed to read stream to compute hash, %w", err)
		}
	}

	checksum = string(base64EncodeHashSum(hasher))
	if computePayloadHash {
		if algorithm != AlgorithmSHA256 {
			sha256Checksum = string(hexEncodeHashSum(sha256Hasher))
		} else {
			sha256Checksum = string(hexEncodeHashSum(hasher))
		}
	}

	return checksum, sha256Checksum, nil
}

func getRequestStreamLength(req *smithyhttp.Request) (int64, error) {
	if v := req.ContentLength; v > 0 {
		return v, nil
	}

	if length, ok, err := req.StreamLength(); err != nil {
		return 0, fmt.Errorf("failed getting request stream's length, %w", err)
	} else if ok {
		return length, nil
	}

	return -1, nil
}

// setMD5Checksum computes the MD5 of the request payload and sets it to the
// Content-MD5 header. Returning the MD5 base64 encoded string or error.
//
// If the MD5 is already set as the Content-MD5 header, that value will be
// returned, and nothing else will be done.
//
// If the payload is empty, no MD5 will be computed. No error will be returned.
// Empty payloads do not have an MD5 value.
//
// Replaces the smithy-go middleware for httpChecksum trait.
func setMD5Checksum(ctx context.Context, req *smithyhttp.Request) (string, error) {
	if v := req.Header.Get(contentMD5Header); len(v) != 0 {
		return v, nil
	}
	stream := req.GetStream()
	if stream == nil {
		return "", nil
	}

	if !req.IsStreamSeekable() {
		return "", fmt.Errorf(
			"unseekable stream is not supported for computing md5 checksum")
	}

	v, err := computeMD5Checksum(stream)
	if err != nil {
		return "", err
	}
	if err := req.RewindStream(); err != nil {
		return "", fmt.Errorf("failed to rewind stream after computing MD5 checksum, %w", err)
	}
	// set the 'Content-MD5' header
	req.Header.Set(contentMD5Header, string(v))
	return string(v), nil
}
