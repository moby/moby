package types

// MediaType represents an HTTP media type (MIME type) used in API
// Content-Type and Accept headers.
//
// In addition to standard media types (for example, "application/json"),
// this package defines vendor-specific vendor media types for streaming
// endpoints, such as raw TTY streams and multiplexed stdout/stderr streams.
type MediaType = string

const (
	// MediaTypeRawStream is a vendor-specific media type for raw TTY streams.
	MediaTypeRawStream MediaType = "application/vnd.docker.raw-stream"

	// MediaTypeMultiplexedStream is a vendor-specific media type for streams
	// where stdin, stdout, and stderr are multiplexed into a single byte stream.
	//
	// Use stdcopy.StdCopy (https://pkg.go.dev/github.com/moby/moby/api/pkg/stdcopy)
	// to demultiplex the stream.
	MediaTypeMultiplexedStream MediaType = "application/vnd.docker.multiplexed-stream"

	// MediaTypeJSON is the media type for JSON objects.
	MediaTypeJSON MediaType = "application/json"

	// MediaTypeNDJSON is the media type for newline-delimited JSON streams (https://github.com/ndjson/ndjson-spec).
	MediaTypeNDJSON MediaType = "application/x-ndjson"

	// MediaTypeJSONLines is the media type for JSON Lines streams (https://jsonlines.org/).
	MediaTypeJSONLines MediaType = "application/jsonl"

	// MediaTypeJSONSequence is the media type for JSON text sequences (RFC 7464).
	MediaTypeJSONSequence MediaType = "application/json-seq"
)
