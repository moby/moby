package jsonstream

import "encoding/json"

// Message defines a message struct. It describes
// the created time, where it from, status, ID of the
// message.
type Message struct {
	Stream   string           `json:"stream,omitempty"`
	Status   string           `json:"status,omitempty"`
	Progress *Progress        `json:"progressDetail,omitempty"`
	ID       string           `json:"id,omitempty"`
	Error    *Error           `json:"errorDetail,omitempty"`
	Push     *PushResult      `json:"push,omitempty"`
	Aux      *json.RawMessage `json:"aux,omitempty"` // Aux contains out-of-band data, such as legacy digests for push signing and image id after building.
}
