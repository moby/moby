// Code generated from OpenAPI definition. DO NOT EDIT.

package storage

// RootFSStorage Information about the storage used for the container's root filesystem.
type RootFSStorage struct {
	// Information about a snapshot backend of the container's root filesystem.
	//
	Snapshot *RootFSStorageSnapshot `json:"Snapshot,omitempty"`
}
