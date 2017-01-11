package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// QueryOptions are used to parameterize a query
type QueryOptions struct {
	// Providing a datacenter overwrites the DC provided
	// by the Config
	Datacenter string

	// AllowStale allows any Consul server (non-leader) to service
	// a read. This allows for lower latency and higher throughput
	AllowStale bool

	// RequireConsistent forces the read to be fully consistent.
	// This is more expensive but prevents ever performing a stale
	// read.
	RequireConsistent bool

	// WaitIndex is used to enable a blocking query. Waits
	// until the timeout or the next index is reached
	WaitIndex uint64

	// WaitTime is used to bound the duration of a wait.
	// Defaults to that of the Config, but can be overriden.
	WaitTime time.Duration

	// Token is used to provide a per-request ACL token
	// which overrides the agent's default token.
	Token string
}

// WriteOptions are used to parameterize a write
type WriteOptions struct {
	// Providing a datacenter overwrites the DC provided
	// by the Config
	Datacenter string

	// Token is used to provide a per-request ACL token
	// which overrides the agent's default token.
	Token string
}

// QueryMeta is used to return meta data about a query
type QueryMeta struct {
	// LastIndex. This can be used as a WaitIndex to perform
	// a blocking query
	LastIndex uint64

	// Time of last contact from the leader for the
	// server servicing the request
	LastContact time.Duration

	// Is there a known leader
	KnownLeader bool

	// How long did the request take
	RequestTime time.Duration
}

// WriteMeta is used to return meta data about a write
type WriteMeta struct {
	// How long did the request take
	RequestTime time.Duration
}

// HttpBasicAuth is used to authenticate http client with HTTP Basic Authentication
type HttpBasicAuth struct {
	// Username to use for HTTP Basic Authentication
	Username string

	// Password to use for HTTP Basic Authentication
	Password string
}

// Config is used to configure the creation of a client
type Config struct {
	// Address is the address of the Consul server
	Address string

	// Scheme is the URI scheme for the Consul server
	Scheme string

	// Datacenter to use. If not provided, the default agent datacenter is used.
	Datacenter string

	// HttpClient is the client to use. Default will be
	// used if not provided.
	HttpClient *http.Client

	// HttpAuth is the auth info to use for http access.
	HttpAuth *HttpBasicAuth

	// WaitTime limits how long a Watch will block. If not provided,
	// the agent default values will be used.
	WaitTime time.Duration

	// Token is used to provide a per-request ACL token
	// which overrides the agent's default token.
	Token string
}

// DefaultConfig returns a default configuration for the client
func DefaultConfig() *Config {
	config := &Config{
		Address:    "127.0.0.1:8500",
		Scheme:     "http",
		HttpClient: http.DefaultClient,
	}

	if addr := os.Getenv("CONSUL_HTTP_ADDR"); addr != "" {
		config.Address = addr
	}

	return config
}

// Client provides a client to the Consul API
type Client struct {
	config Config
}

// NewClient returns a new client
func NewClient(config *Config) (*Client, error) {
	// bootstrap the config
	defConfig := DefaultConfig()

	if len(config.Address) == 0 {
		config.Address = defConfig.Address
	}

	if len(config.Scheme) == 0 {
		config.Scheme = defConfig.Scheme
	}

	if config.HttpClient == nil {
		config.HttpClient = defConfig.HttpClient
	}

	if parts := strings.SplitN(config.Address, "unix://", 2); len(parts) == 2 {
		config.HttpClient = &http.Client{
			Transport: &http.Transport{
				Dial: func(_, _ string) (net.Conn, error) {
					return net.Dial("unix", parts[1])
				},
			},
		}
		config.Address = parts[1]
	}

	client := &Client{
		config: *config,
	}
	return client, nil
}

// request is used to help build up a request
type request struct {
	config *Config
	method string
	url    *url.URL
	params url.Values
	body   io.Reader
	obj    interface{}
}

// setQueryOptions is used to annotate the request with
// additional query options
func (r *request) setQueryOptions(q *QueryOptions) {
	if q == nil {
		return
	}
	if q.Datacenter != "" {
		r.params.Set("dc", q.Datacenter)
	}
	if q.AllowStale {
		r.params.Set("stale", "")
	}
	if q.RequireConsistent {
		r.params.Set("consistent", "")
	}
	if q.WaitIndex != 0 {
		r.params.Set("index", strconv.FormatUint(q.WaitIndex, 10))
	}
	if q.WaitTime != 0 {
		r.params.Set("wait", durToMsec(q.WaitTime))
	}
	if q.Token != "" {
		r.params.Set("token", q.Token)
	}
}

// durToMsec converts a duration to a millisecond specified string
func durToMsec(dur time.Duration) string {
	return fmt.Sprintf("%dms", dur/time.Millisecond)
}

// setWriteOptions is used to annotate the request with
// additional write options
func (r *request) setWriteOptions(q *WriteOptions) {
	if q == nil {
		return
	}
	if q.Datacenter != "" {
		r.params.Set("dc", q.Datacenter)
	}
	if q.Token != "" {
		r.params.Set("token", q.Token)
	}
}

// toHTTP converts the request to an HTTP request
func (r *request) toHTTP() (*http.Request, error) {
	// Encode the query parameters
	r.url.RawQuery = r.params.Encode()

	// Check if we should encode the body
	if r.body == nil && r.obj != nil {
		if b, err := encodeBody(r.obj); err != nil {
			return nil, err
		} else {
			r.body = b
		}
	}

	// Create the HTTP request
	req, err := http.NewRequest(r.method, r.url.RequestURI(), r.body)
	if err != nil {
		return nil, err
	}

	req.URL.Host = r.url.Host
	req.URL.Scheme = r.url.Scheme
	req.Host = r.url.Host

	// Setup auth
	if r.config.HttpAuth != nil {
		req.SetBasicAuth(r.config.HttpAuth.Username, r.config.HttpAuth.Password)
	}

	return req, nil
}

// newRequest is used to create a new request
func (c *Client) newRequest(method, path string) *request {
	r := &request{
		config: &c.config,
		method: method,
		url: &url.URL{
			Scheme: c.config.Scheme,
			Host:   c.config.Address,
			Path:   path,
		},
		params: make(map[string][]string),
	}
	if c.config.Datacenter != "" {
		r.params.Set("dc", c.config.Datacenter)
	}
	if c.config.WaitTime != 0 {
		r.params.Set("wait", durToMsec(r.config.WaitTime))
	}
	if c.config.Token != "" {
		r.params.Set("token", r.config.Token)
	}
	return r
}

// doRequest runs a request with our client
func (c *Client) doRequest(r *request) (time.Duration, *http.Response, error) {
	req, err := r.toHTTP()
	if err != nil {
		return 0, nil, err
	}
	start := time.Now()
	resp, err := c.config.HttpClient.Do(req)
	diff := time.Now().Sub(start)
	return diff, resp, err
}

// parseQueryMeta is used to help parse query meta-data
func parseQueryMeta(resp *http.Response, q *QueryMeta) error {
	header := resp.Header

	// Parse the X-Consul-Index
	index, err := strconv.ParseUint(header.Get("X-Consul-Index"), 10, 64)
	if err != nil {
		return fmt.Errorf("Failed to parse X-Consul-Index: %v", err)
	}
	q.LastIndex = index

	// Parse the X-Consul-LastContact
	last, err := strconv.ParseUint(header.Get("X-Consul-LastContact"), 10, 64)
	if err != nil {
		return fmt.Errorf("Failed to parse X-Consul-LastContact: %v", err)
	}
	q.LastContact = time.Duration(last) * time.Millisecond

	// Parse the X-Consul-KnownLeader
	switch header.Get("X-Consul-KnownLeader") {
	case "true":
		q.KnownLeader = true
	default:
		q.KnownLeader = false
	}
	return nil
}

// decodeBody is used to JSON decode a body
func decodeBody(resp *http.Response, out interface{}) error {
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

// encodeBody is used to encode a request body
func encodeBody(obj interface{}) (io.Reader, error) {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(obj); err != nil {
		return nil, err
	}
	return buf, nil
}

// requireOK is used to wrap doRequest and check for a 200
func requireOK(d time.Duration, resp *http.Response, e error) (time.Duration, *http.Response, error) {
	if e != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return d, nil, e
	}
	if resp.StatusCode != 200 {
		var buf bytes.Buffer
		io.Copy(&buf, resp.Body)
		resp.Body.Close()
		return d, nil, fmt.Errorf("Unexpected response code: %d (%s)", resp.StatusCode, buf.Bytes())
	}
	return d, resp, nil
}
