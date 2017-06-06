package packngo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	libraryVersion = "0.1.0"
	baseURL        = "https://api.packet.net/"
	userAgent      = "packngo/" + libraryVersion
	mediaType      = "application/json"

	headerRateLimit     = "X-RateLimit-Limit"
	headerRateRemaining = "X-RateLimit-Remaining"
	headerRateReset     = "X-RateLimit-Reset"
)

// ListOptions specifies optional global API parameters
type ListOptions struct {
	// for paginated result sets, page of results to retrieve
	Page int `url:"page,omitempty"`

	// for paginated result sets, the number of results to return per page
	PerPage int `url:"per_page,omitempty"`

	// specify which resources you want to return as collections instead of references
	Includes string
}

// Response is the http response from api calls
type Response struct {
	*http.Response
	Rate
}

func (r *Response) populateRate() {
	// parse the rate limit headers and populate Response.Rate
	if limit := r.Header.Get(headerRateLimit); limit != "" {
		r.Rate.RequestLimit, _ = strconv.Atoi(limit)
	}
	if remaining := r.Header.Get(headerRateRemaining); remaining != "" {
		r.Rate.RequestsRemaining, _ = strconv.Atoi(remaining)
	}
	if reset := r.Header.Get(headerRateReset); reset != "" {
		if v, _ := strconv.ParseInt(reset, 10, 64); v != 0 {
			r.Rate.Reset = Timestamp{time.Unix(v, 0)}
		}
	}
}

// ErrorResponse is the http response used on errrors
type ErrorResponse struct {
	Response *http.Response
	Errors   []string `json:"errors"`
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d %v",
		r.Response.Request.Method, r.Response.Request.URL, r.Response.StatusCode, strings.Join(r.Errors, ", "))
}

// Client is the base API Client
type Client struct {
	client *http.Client

	BaseURL *url.URL

	UserAgent     string
	ConsumerToken string
	APIKey        string

	RateLimit Rate

	// Packet Api Objects
	Plans            PlanService
	Users            UserService
	Emails           EmailService
	SSHKeys          SSHKeyService
	Devices          DeviceService
	Projects         ProjectService
	Facilities       FacilityService
	OperatingSystems OSService
	Ips              IPService
	IpReservations   IPReservationService
	Volumes          VolumeService
}

// NewRequest inits a new http request with the proper headers
func (c *Client) NewRequest(method, path string, body interface{}) (*http.Request, error) {
	// relative path to append to the endpoint url, no leading slash please
	rel, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.ResolveReference(rel)

	// json encode the request body, if any
	buf := new(bytes.Buffer)
	if body != nil {
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	req.Close = true

	req.Header.Add("X-Auth-Token", c.APIKey)
	req.Header.Add("X-Consumer-Token", c.ConsumerToken)

	req.Header.Add("Content-Type", mediaType)
	req.Header.Add("Accept", mediaType)
	req.Header.Add("User-Agent", userAgent)
	return req, nil
}

// Do executes the http request
func (c *Client) Do(req *http.Request, v interface{}) (*Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	response := Response{Response: resp}
	response.populateRate()
	c.RateLimit = response.Rate

	err = checkResponse(resp)
	// if the response is an error, return the ErrorResponse
	if err != nil {
		return &response, err
	}

	if v != nil {
		// if v implements the io.Writer interface, return the raw response
		if w, ok := v.(io.Writer); ok {
			io.Copy(w, resp.Body)
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
			if err != nil {
				return &response, err
			}
		}
	}

	return &response, err
}

// NewClient initializes and returns a Client, use this to get an API Client to operate on
// N.B.: Packet's API certificate requires Go 1.5+ to successfully parse. If you are using
// an older version of Go, pass in a custom http.Client with a custom TLS configuration
// that sets "InsecureSkipVerify" to "true"
func NewClient(consumerToken string, apiKey string, httpClient *http.Client) *Client {
	client, _ := NewClientWithBaseURL(consumerToken, apiKey, httpClient, baseURL)
	return client
}
func NewClientWithBaseURL(consumerToken string, apiKey string, httpClient *http.Client, apiBaseURL string) (*Client, error) {
	if httpClient == nil {
		// Don't fall back on http.DefaultClient as it's not nice to adjust state
		// implicitly. If the client wants to use http.DefaultClient, they can
		// pass it in explicitly.
		httpClient = &http.Client{}
	}

	u, err := url.Parse(apiBaseURL)
	if err != nil {
		return nil, err
	}

	c := &Client{client: httpClient, BaseURL: u, UserAgent: userAgent, ConsumerToken: consumerToken, APIKey: apiKey}
	c.Plans = &PlanServiceOp{client: c}
	c.Users = &UserServiceOp{client: c}
	c.Emails = &EmailServiceOp{client: c}
	c.SSHKeys = &SSHKeyServiceOp{client: c}
	c.Devices = &DeviceServiceOp{client: c}
	c.Projects = &ProjectServiceOp{client: c}
	c.Facilities = &FacilityServiceOp{client: c}
	c.OperatingSystems = &OSServiceOp{client: c}
	c.Ips = &IPServiceOp{client: c}
	c.IpReservations = &IPReservationServiceOp{client: c}
	c.Volumes = &VolumeServiceOp{client: c}

	return c, nil
}

func checkResponse(r *http.Response) error {
	// return if http status code is within 200 range
	if c := r.StatusCode; c >= 200 && c <= 299 {
		// response is good, return
		return nil
	}

	errorResponse := &ErrorResponse{Response: r}
	data, err := ioutil.ReadAll(r.Body)
	// if the response has a body, populate the message in errorResponse
	if err == nil && len(data) > 0 {
		json.Unmarshal(data, errorResponse)
	}

	return errorResponse
}
