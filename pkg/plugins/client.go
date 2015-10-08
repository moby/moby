package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

	scheme := protoAndAddr[0]
	if scheme != "https" {
		scheme = "http"
	}
	return &Client{&http.Client{Transport: tr}, scheme, protoAndAddr[1]}, nil
}

// Client represents a plugin client.
type Client struct {
	http   *http.Client // http client to use
	scheme string       // scheme protocol of the plugin
	addr   string       // http address of the plugin
}

// Call calls the specified method with the specified arguments for the plugin.
// It will retry for 30 seconds if a failure occurs when calling.
func (c *Client) Call(serviceMethod string, args interface{}, ret interface{}) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(args); err != nil {
		return err
	}
	body, err := c.callWithRetry(serviceMethod, &buf, true)
	if err != nil {
		return err
	}
	defer body.Close()
	if err := json.NewDecoder(body).Decode(&ret); err != nil {
		logrus.Errorf("%s: error reading plugin resp: %v", serviceMethod, err)
		return err
	}
	return nil
}

// Stream calls the specified method with the specified arguments for the plugin and returns the response body
func (c *Client) Stream(serviceMethod string, args interface{}) (io.ReadCloser, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(args); err != nil {
		return nil, err
	}
	return c.callWithRetry(serviceMethod, &buf, true)
}

// SendFile calls the specified method, and passes through the IO stream
func (c *Client) SendFile(serviceMethod string, data io.Reader, ret interface{}) error {
	body, err := c.callWithRetry(serviceMethod, data, true)
	if err != nil {
		return err
	}
	if err := json.NewDecoder(body).Decode(&ret); err != nil {
		logrus.Errorf("%s: error reading plugin resp: %v", serviceMethod, err)
		return err
	}
	return nil
}

func (c *Client) callWithRetry(serviceMethod string, data io.Reader, retry bool) (io.ReadCloser, error) {
	req, err := http.NewRequest("POST", "/"+serviceMethod, data)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", versionMimetype)
	req.URL.Scheme = c.scheme
	req.URL.Host = c.addr

	var retries int
	start := time.Now()

	for {
		resp, err := c.http.Do(req)
		if err != nil {
			if !retry {
				return nil, err
			}

			timeOff := backoff(retries)
			if abort(start, timeOff) {
				return nil, err
			}
			retries++
			logrus.Warnf("Unable to connect to plugin: %s, retrying in %v", c.addr, timeOff)
			time.Sleep(timeOff)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			remoteErr, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, &remoteError{err.Error(), serviceMethod}
			}
			return nil, &remoteError{string(remoteErr), serviceMethod}
		}
		return resp.Body, nil
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
