// Code generated from OpenAPI definition. DO NOT EDIT.

package common

// ErrorResponse Represents an error.
//
//	Example : {
//	  "message": "Something went wrong."
//	}
type ErrorResponse struct {
	// The error message.
	// Required: true
	Message string `json:"message"`
}

func (e ErrorResponse) Error() string {
	return e.Message
}
