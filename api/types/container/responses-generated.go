// Code generated by swagger-gen. DO NOT EDIT

package container

type ContainerChangesOKResponse []ContainerChangeResponseItem

// change item in response to ContainerChanges operation
type ContainerChangeResponseItem struct {
	// Kind of change
	Kind uint8 `json:"Kind"`
	// Path to file that has changed
	Path string `json:"Path"`
}

// OK response to ContainerCreate operation
type ContainerCreateResponse struct {
	// The ID of the created container
	ID string `json:"Id"`
	// Warnings encountered when creating the container
	Warnings []string `json:"Warnings"`
}

// OK response to ContainerTop operation
type ContainerTopResponse struct {
	// Each process running in the container, where each is process is an array of values corresponding to the titles
	Processes [][]string `json:"Processes,omitempty"`
	// The ps column titles
	Titles []string `json:"Titles,omitempty"`
}

// OK response to ContainerUpdate operation
type ContainerUpdateResponse struct {
	Warnings []string `json:"Warnings,omitempty"`
}

// OK response to ContainerWait operation
type ContainerWaitResponse struct {
	// container waiting error, if any
	Error *ContainerWaitResponseError `json:"Error,omitempty"`
	// Exit code of the container
	StatusCode int64 `json:"StatusCode"`
}

// container waiting error, if any
type ContainerWaitResponseError struct {
	// Details of an error
	Message string `json:"Message,omitempty"`
}
