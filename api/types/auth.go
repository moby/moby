package types

// AuthConfig contains authorization information for connecting to a Registry
type AuthConfig struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`
	Email    string `json:"email,omitempty"`

	ServerAddress string `json:"serveraddress,omitempty"`

	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `json:"identitytoken,omitempty"`

	// RegistryToken is a bearer token to be sent to a registry
	RegistryToken string `json:"registrytoken,omitempty"`
}

// AuthResponse is the response of a remote authentication
type AuthResponse struct {
	// Status is the authentication status
	Status string `json:"Status"`

	// IdentityToken is an opaque token used for authenticating
	// a user after a successful login.
	IdentityToken string `json:"IdentityToken,omitempty"`
}

// AuthTokenDebugResponse is the response of a token debugging request
type AuthTokenDebugResponse struct {
	// Claims contains the JWT claims from the token
	Claims map[string]interface{} `json:"claims"`
}
