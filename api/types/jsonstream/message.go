package jsonstream

import "encoding/json"

// JSONMessage defines a message struct. It describes
// the created time, where it from, status, ID of the
// message. It's used for docker events.
type Message struct {
	Stream   string           `json:"stream,omitempty"`
	Status   string           `json:"status,omitempty"`
	Progress *Progress        `json:"progressDetail,omitempty"`
	ID       string           `json:"id,omitempty"`
	Error    *Error           `json:"errorDetail,omitempty"`
	Aux      *json.RawMessage `json:"aux,omitempty"` // Aux contains out-of-band data, such as digests for push signing and image id after building.
}
