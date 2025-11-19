// Code generated from OpenAPI definition. DO NOT EDIT.

package image

// HistoryResponseItem individual image layer information in response to ImageHistory operation
type HistoryResponseItem struct {
	//
	// Required: true
	Id string `json:"Id"`

	//
	// Required: true
	Created int64 `json:"Created"`

	//
	// Required: true
	CreatedBy string `json:"CreatedBy"`

	//
	// Required: true
	Tags []string `json:"Tags"`

	//
	// Required: true
	Size int64 `json:"Size"`

	//
	// Required: true
	Comment string `json:"Comment"`
}
