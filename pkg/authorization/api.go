package authorization

const (
	// AuthZApiRequest is the url for daemon request authorization
	AuthZApiRequest = "AuthZPlugin.AuthZReq"

	// AuthZApiResponse is the url for daemon response authorization
	AuthZApiResponse = "AuthZPlugin.AuthZRes"

	// AuthZApiImplements is the name of the interface all AuthZ plugins implement
	AuthZApiImplements = "authz"
)

// Request holds data required for authZ plugins
type Request struct {
	// User holds the user extracted by AuthN mechanism
	User string `json:"User,omitempty"`

	// UserAuthNMethod holds the mechanism used to extract user details (e.g., krb)
	UserAuthNMethod string `json:"UserAuthNMethod,omitempty"`

	// RequestMethod holds the HTTP method (GET/POST/PUT)
	RequestMethod string `json:"RequestMethod,omitempty"`

	// RequestUri holds the full HTTP uri (e.g., /v1.21/version)
	RequestURI string `json:"RequestUri,omitempty"`

	// RequestBody stores the raw request body sent to the docker daemon
	RequestBody []byte `json:"RequestBody,omitempty"`

	// RequestHeaders stores the raw request headers sent to the docker daemon
	RequestHeaders map[string]string `json:"RequestHeaders,omitempty"`

	// ResponseStatusCode stores the status code returned from docker daemon
	ResponseStatusCode int `json:"ResponseStatusCode,omitempty"`

	// ResponseBody stores the raw response body sent from docker daemon
	ResponseBody []byte `json:"ResponseBody,omitempty"`

	// ResponseHeaders stores the response headers sent to the docker daemon
	ResponseHeaders map[string]string `json:"ResponseHeaders,omitempty"`
}

// Response represents authZ plugin response
type Response struct {
	// Allow indicating whether the user is allowed or not
	Allow bool `json:"Allow"`

	// Msg stores the authorization message
	Msg string `json:"Msg,omitempty"`

	// Err stores a message in case there's an error
	Err string `json:"Err,omitempty"`
}
