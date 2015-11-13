package plugins

import (
	"errors"
	"io"
)

// Call calls the specified method with the specified arguments for the plugin.
// We only implement this method on Windows to avoid link errors in libnetwork
// TODO Windows: This can be factored out with some work in libnetwork and revendor
func (c *Client) Call(serviceMethod string, args interface{}, ret interface{}) error {
	return errors.New("plugins Call() not implemented on Windows")
}

// SendFile calls the specified method, and passes through the IO stream.
// TODO Windows: This also may be able to be completely factored out. Used by
// graphdriver plugin in experimental builds.
func (c *Client) SendFile(serviceMethod string, data io.Reader, ret interface{}) error {
	return errors.New("plugins Stream() not implemented on Windows")
}

// Stream calls the specified method with the specified arguments for the plugin and returns the response body
// TODO Windows: This also may be able to be completely factored out. Used by
// graphdriver plugin in experimental builds.
func (c *Client) Stream(serviceMethod string, args interface{}) (io.ReadCloser, error) {
	return nil, errors.New("plugins Stream() not implemented on Windows")
}
