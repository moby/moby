package client

import (
	"net/http"
	"net/url"
	"path"
	"strings"
)

// Settings stores the settings in use by this client
//
// TODO this should move to a common pkg for sub-client implementations
// to leverage.
type Settings struct {
	// scheme sets the scheme for the client
	Scheme string
	// host holds the server address to connect to
	Host string
	// proto holds the client protocol i.e. unix.
	Proto string
	// addr holds the client address.
	Addr string
	// basePath holds the path to prepend to the requests.
	BasePath string
	// client used to send and receive http requests.
	Client *http.Client
	// version of the server to talk to.
	Version string
	// custom http headers configured by users.
	CustomHTTPHeaders map[string]string
}

// Client is designed to augment the existing Docker API Client with Stacks support
type Client struct {
	settings Settings
}

// NewClientWithSettings creates a new Stack client with settings
func NewClientWithSettings(settings Settings) (*Client, error) {
	c := &Client{
		settings: settings,
	}
	return c, nil
}

// getAPIPath returns the versioned request path to call the api.
// It appends the query parameters to the path if they are not empty.
func (cli *Client) getAPIPath(p string, query url.Values) string {
	var apiPath string
	if cli.settings.Version != "" {
		v := strings.TrimPrefix(cli.settings.Version, "v")
		apiPath = path.Join(cli.settings.BasePath, "/v"+v, p)
	} else {
		apiPath = path.Join(cli.settings.BasePath, p)
	}
	return (&url.URL{Path: apiPath, RawQuery: query.Encode()}).String()
}
