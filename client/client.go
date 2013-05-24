package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

type Client struct {
	endpoint string
	client   *http.Client
}

func NewClient(endpoint string) (*Client, error) {
	if endpoint == "" {
		return nil, errors.New("Server endpoint cannot be empty")
	}
	return &Client{endpoint: endpoint, client: http.DefaultClient}, nil
}

func (c *Client) do(method, path string, data interface{}) ([]byte, int, error) {
	var params io.Reader
	if data != nil {
		buf, err := json.Marshal(data)
		if err != nil {
			return nil, -1, err
		}
		params = bytes.NewBuffer(buf)
	}
	req, err := http.NewRequest(method, c.getURL(path), params)
	if err != nil {
		return nil, -1, err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+docker.VERSION)
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
		}
		return nil, -1, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, resp.StatusCode, newApiClientError(resp.StatusCode, body)
	}
	return body, resp.StatusCode, nil
}

func (c *Client) getURL(path string) string {
	return fmt.Sprintf("%s/v%f%s", strings.TrimRight(c.endpoint, "/"), docker.API_VERSION, path)
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

func newApiClientError(status int, body []byte) *apiClientError {
	return &apiClientError{status: status, message: string(body)}
}

func (e *apiClientError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.status, e.message)
}
