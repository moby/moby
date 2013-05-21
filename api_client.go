package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
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

func (c *APIClient) getURL(path string) string {
	return strings.TrimRight(c.endpoint, "/") + path
}

func (c *APIClient) ListContainers(opts *ListContainersOptions) ([]ApiContainer, error) {
	url := c.getURL("/containers/ps") + "?" + queryString(opts)
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

func queryString(opts interface{}) string {
	if opts == nil {
		return ""
	}
	value := reflect.ValueOf(opts)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	items := url.Values(map[string][]string{})
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		key := strings.ToLower(field.Name)
		v := value.Field(i)
		switch v.Kind() {
		case reflect.Bool:
			if v.Bool() {
				items.Add(key, "1")
			}
		case reflect.Int:
			fallthrough
		case reflect.Int8:
			fallthrough
		case reflect.Int16:
			fallthrough
		case reflect.Int32:
			fallthrough
		case reflect.Int64:
			if v.Int() > 0 {
				items.Add(key, strconv.FormatInt(v.Int(), 10))
			}
		case reflect.Float32:
			fallthrough
		case reflect.Float64:
			if v.Float() > 0 {
				items.Add(key, strconv.FormatFloat(v.Float(), 'f', -1, 64))
			}
		case reflect.String:
			if v.String() != "" {
				items.Add(key, v.String())
			}
		}
	}
	return items.Encode()
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
