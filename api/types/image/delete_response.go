// Code generated from OpenAPI definition. DO NOT EDIT.

package image

// DeleteResponse
type DeleteResponse struct {
	// The image ID of an image that was deleted
	Deleted string `json:"Deleted,omitempty"`

	// The image ID of an image that was untagged
	Untagged string `json:"Untagged,omitempty"`
}
