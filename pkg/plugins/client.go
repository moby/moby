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
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
)

const (
	versionMimetype = "application/vnd.docker.plugins.v1.2+json"
	defaultTimeOut  = 30
)

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
	if args != nil {
		if err := json.NewEncoder(&buf).Encode(args); err != nil {
			return err
		}
	}
	body, err := c.callWithRetry(serviceMethod, &buf, true)
	if err != nil {
		return err
	}
	defer body.Close()
	if ret != nil {
		if err := json.NewDecoder(body).Decode(&ret); err != nil {
			logrus.Errorf("%s: error reading plugin resp: %v", serviceMethod, err)
			return err
		}
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
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("%s: %s", serviceMethod, err)
			}

			// Plugins' Response(s) should have an Err field indicating what went
			// wrong. Try to unmarshal into ResponseErr. Otherwise fallback to just
			// return the string(body)
			type responseErr struct {
				Err string
			}
			remoteErr := responseErr{}
			if err := json.Unmarshal(b, &remoteErr); err == nil {
				if remoteErr.Err != "" {
					return nil, fmt.Errorf("%s: %s", serviceMethod, remoteErr.Err)
				}
			}
			// old way...
			return nil, fmt.Errorf("%s: %s", serviceMethod, string(b))
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
