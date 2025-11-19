// Code generated from OpenAPI definition. DO NOT EDIT.

package plugin

// RootFS
type RootFS struct {
	//
	// Example: layers
	Type string `json:"type,omitempty"`

	//
	// Example: sha256:675532206fbf3030b8458f88d6e26d4eb1577688a25efec97154c94e8b6b4887
	// sha256:e216a057b1cb1efc11f8a268f37ef62083e70b1b38323ba252e25ac88904a7e8
	Diff_ids []string `json:"diff_ids,omitempty"`
}
