package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/sockets"
	"github.com/docker/docker/pkg/tlsconfig"
)

const (
	versionMimetype = "application/vnd.docker.plugins.v1.1+json"
	defaultTimeOut  = 30
)

type remoteError struct {
	method string
	err    string
}

func (e *remoteError) Error() string {
	return fmt.Sprintf("Plugin Error: %s, %s", e.err, e.method)
}

// NewClient creates a new plugin client (http).
func NewClient(addr string, tlsConfig tlsconfig.Options) (*Client, error) {
	tr := &http.Transport{}

	c, err := tlsconfig.Client(tlsConfig)
	if err != nil {
		return nil, err
	}
	tr.TLSClientConfig = c

	protoAndAddr := strings.Split(addr, "://")
	sockets.ConfigureTCPTransport(tr, protoAndAddr[0], protoAndAddr[1])
	return &Client{&http.Client{Transport: tr}, protoAndAddr[1]}, nil
}

// Client represents a plugin client.
type Client struct {
	http *http.Client // http client to use
	addr string       // http address of the plugin
}

// Call calls the specified method with the specified arguments for the plugin.
// It will retry for 30 seconds if a failure occurs when calling.
func (c *Client) Call(serviceMethod string, args interface{}, ret interface{}) error {
	return c.callWithRetry(serviceMethod, args, ret, true)
}

func (c *Client) callWithRetry(serviceMethod string, args interface{}, ret interface{}, retry bool) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(args); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "/"+serviceMethod, &buf)
	if err != nil {
		return err
	}
	req.Header.Add("Accept", versionMimetype)
	req.URL.Scheme = "http"
	req.URL.Host = c.addr

	var retries int
	start := time.Now()

	for {
		resp, err := c.http.Do(req)
		if err != nil {
			if !retry {
				return err
			}

			timeOff := backoff(retries)
			if abort(start, timeOff) {
				return err
			}
			retries++
			logrus.Warnf("Unable to connect to plugin: %s, retrying in %v", c.addr, timeOff)
			time.Sleep(timeOff)
			continue
		}

		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			remoteErr, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return &remoteError{err.Error(), serviceMethod}
			}
			return &remoteError{string(remoteErr), serviceMethod}
		}

		return json.NewDecoder(resp.Body).Decode(&ret)
	}
}

func backoff(retries int) time.Duration {
	b, max := 1, defaultTimeOut
	for b < max && retries > 0 {
		b *= 2
		retries--
	}
	if b > max {
		b = max
	}
	return time.Duration(b) * time.Second
}

func abort(start time.Time, timeOff time.Duration) bool {
	return timeOff+time.Since(start) >= time.Duration(defaultTimeOut)*time.Second
}
