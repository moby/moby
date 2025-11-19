// Code generated from OpenAPI definition. DO NOT EDIT.

package registry

// AuthResponse An identity token was generated successfully.
type AuthResponse struct {
	// The status of the authentication
	// Example: Login Succeeded
	// Required: true
	Status string `json:"Status"`

	// An opaque token used to authenticate a user after a successful login
	// Example: 9cbaf023786cd7...
	IdentityToken string `json:"IdentityToken,omitempty"`
}
