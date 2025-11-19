// Code generated from OpenAPI definition. DO NOT EDIT.

package swarm

// ServiceUpdateResponse
//
//	Example : {
//	  "Warnings": [
//	    "unable to pin image doesnotexist:latest to digest: image library/doesnotexist:latest not found"
//	  ]
//	}
type ServiceUpdateResponse struct {
	// Optional warning messages
	Warnings []string `json:"Warnings,omitempty"`
}
