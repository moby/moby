package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type APIClient struct {
	endpoint string
	client   *http.Client
}

func NewAPIClient(endpoint string) (*APIClient, error) {
	if endpoint == "" {
		return nil, errors.New("Server endpoint cannot be empty")
	}
	return &APIClient{endpoint: endpoint, client: http.DefaultClient}, nil
}

type ListContainersOptions struct {
	All    bool
	Limit  int
	Since  string
	Before string
}

func (opts *ListContainersOptions) queryString() string {
	if opts == nil {
		return ""
	}
	items := url.Values(map[string][]string{})
	if opts.All {
		items.Add("all", "1")
	}
	if opts.Limit > 0 {
		items.Add("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Since != "" {
		items.Add("since", opts.Since)
	}
	if opts.Before != "" {
		items.Add("before", opts.Before)
	}
	return items.Encode()
}

func (c *APIClient) getURL(path string) string {
	return strings.TrimRight(c.endpoint, "/") + path
}

func (c *APIClient) ListContainers(opts *ListContainersOptions) ([]ApiContainer, error) {
	url := c.getURL("/containers/ps") + "?" + opts.queryString()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, newApiClientError(resp)
	}
	var containers []ApiContainer
	err = json.NewDecoder(resp.Body).Decode(&containers)
	if err != nil {
		return nil, err
	}
	return containers, nil
}

type apiClientError struct {
	status  int
	message string
}

func newApiClientError(resp *http.Response) *apiClientError {
	body, _ := ioutil.ReadAll(resp.Body)
	return &apiClientError{status: resp.StatusCode, message: string(body)}
}

func (e *apiClientError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.status, e.message)
}
