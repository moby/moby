package jsonstream

// Progress describes a progress message in a JSON stream.
type Progress struct {
	Current    int64  `json:"current,omitempty"`    // Current is the current status and value of the progress made towards Total.
	Total      int64  `json:"total,omitempty"`      // Total is the end value describing when we made 100% progress for an operation.
	Start      int64  `json:"start,omitempty"`      // Start is the initial value for the operation.
	HideCounts bool   `json:"hidecounts,omitempty"` // HideCounts. if true, hides the progress count indicator (xB/yB).
	Units      string `json:"units,omitempty"`      // Units is the unit to print for progress. It defaults to "bytes" if empty.
}
