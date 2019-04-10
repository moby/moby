package types // import "github.com/docker/docker/api/types"

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

// IDResponse Response to an API call that returns just an Id
// swagger:model IdResponse
type IDResponse struct {

	// The id of the newly created object.
	// Required: true
	ID string `json:"Id"`
}
