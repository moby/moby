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

// ComponentVersion describes the version information for a specific component.
type ComponentVersion struct {
	Name    string
	Version string
	Details map[string]string `json:",omitempty"`
}

// Version contains response of Engine API:
// GET "/version"
type Version struct {
	Platform   struct{ Name string } `json:",omitempty"`
	Components []ComponentVersion    `json:",omitempty"`

	// The following fields are deprecated, they relate to the Engine component and are kept for backwards compatibility

	Version       string
	APIVersion    string `json:"ApiVersion"`
	MinAPIVersion string `json:"MinAPIVersion,omitempty"`
	GitCommit     string
	GoVersion     string
	Os            string
	Arch          string
	KernelVersion string `json:",omitempty"`
	Experimental  bool   `json:",omitempty"`
	BuildTime     string `json:",omitempty"`
}
