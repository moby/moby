package plugins

import "net/http"

// Client represents a plugin client.
type Client struct {
	http   *http.Client // http client to use
	scheme string       // scheme protocol of the plugin
	addr   string       // http address of the plugin
}
