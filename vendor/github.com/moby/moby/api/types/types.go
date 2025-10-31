package types

const (
	// MediaTypeRawStream is vendor specific MIME-Type set for raw TTY streams.
	MediaTypeRawStream = "application/vnd.docker.raw-stream"

	// MediaTypeMultiplexedStream is vendor specific MIME-Type set for stdin/stdout/stderr multiplexed streams.
	MediaTypeMultiplexedStream = "application/vnd.docker.multiplexed-stream"

	// MediaTypeJSON is the MIME-Type for JSON objects.
	MediaTypeJSON = "application/json"

	// MediaTypeNDJSON is the MIME-Type for Newline Delimited JSON objects streams.
	MediaTypeNDJSON = "application/x-ndjson"

	// MediaTypeJSONSequence is the MIME-Type for JSON Text Sequences (RFC7464).
	MediaTypeJSONSequence = "application/json-seq"
)
