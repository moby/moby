package jsonstream

// PushResult contains the result of an image push.
type PushResult struct {
	Tag    string `json:"tag,omitempty"`
	Digest string `json:"digest,omitempty"`
	Size   int    `json:"size,omitempty"`
}
