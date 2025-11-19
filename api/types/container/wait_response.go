// Code generated from OpenAPI definition. DO NOT EDIT.

package container

// WaitResponse OK response to ContainerWait operation
type WaitResponse struct {
	// container waiting error, if any
	Error *WaitExitError `json:"Error,omitempty"`

	// Exit code of the container
	// Required: true
	StatusCode int64 `json:"StatusCode"`
}
