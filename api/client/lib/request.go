package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/docker/docker/utils"
)

// ServerResponse is a wrapper for http API responses.
type ServerResponse struct {
	body       io.ReadCloser
	header     http.Header
	statusCode int
}

// HEAD sends an http request to the docker API using the method HEAD.
func (cli *Client) HEAD(path string, query url.Values, headers map[string][]string) (*ServerResponse, error) {
	return cli.sendRequest("HEAD", path, query, nil, headers)
}

// GET sends an http request to the docker API using the method GET.
func (cli *Client) GET(path string, query url.Values, headers map[string][]string) (*ServerResponse, error) {
	return cli.sendRequest("GET", path, query, nil, headers)
}

// POST sends an http request to the docker API using the method POST.
func (cli *Client) POST(path string, query url.Values, body interface{}, headers map[string][]string) (*ServerResponse, error) {
	return cli.sendRequest("POST", path, query, body, headers)
}

// POSTRaw sends the raw input to the docker API using the method POST.
func (cli *Client) POSTRaw(path string, query url.Values, body io.Reader, headers map[string][]string) (*ServerResponse, error) {
	return cli.sendClientRequest("POST", path, query, body, headers)
}

// PUT sends an http request to the docker API using the method PUT.
func (cli *Client) PUT(path string, query url.Values, body interface{}, headers map[string][]string) (*ServerResponse, error) {
	return cli.sendRequest("PUT", path, query, body, headers)
}

// DELETE sends an http request to the docker API using the method DELETE.
func (cli *Client) DELETE(path string, query url.Values, headers map[string][]string) (*ServerResponse, error) {
	return cli.sendRequest("DELETE", path, query, nil, headers)
}

func (cli *Client) sendRequest(method, path string, query url.Values, body interface{}, headers map[string][]string) (*ServerResponse, error) {
	params, err := encodeData(body)
	if err != nil {
		return nil, err
	}

	if body != nil {
		if headers == nil {
			headers = make(map[string][]string)
		}
		headers["Content-Type"] = []string{"application/json"}
	}

	return cli.sendClientRequest(method, path, query, params, headers)
}

func (cli *Client) sendClientRequest(method, path string, query url.Values, in io.Reader, headers map[string][]string) (*ServerResponse, error) {
	serverResp := &ServerResponse{
		body:       nil,
		statusCode: -1,
	}

	expectedPayload := (method == "POST" || method == "PUT")
	if expectedPayload && in == nil {
		in = bytes.NewReader([]byte{})
	}

	apiPath := cli.getAPIPath(path, query)
	req, err := http.NewRequest(method, apiPath, in)
	if err != nil {
		return serverResp, err
	}

	// Add CLI Config's HTTP Headers BEFORE we set the Docker headers
	// then the user can't change OUR headers
	for k, v := range cli.customHTTPHeaders {
		req.Header.Set(k, v)
	}

	req.URL.Host = cli.Addr
	req.URL.Scheme = cli.Scheme

	if headers != nil {
		for k, v := range headers {
			req.Header[k] = v
		}
	}

	if expectedPayload && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "text/plain")
	}

	resp, err := cli.HTTPClient.Do(req)
	if resp != nil {
		serverResp.statusCode = resp.StatusCode
	}

	if err != nil {
		if utils.IsTimeout(err) || strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "dial unix") {
			return serverResp, errConnectionFailed
		}

		if cli.Scheme == "http" && strings.Contains(err.Error(), "malformed HTTP response") {
			return serverResp, fmt.Errorf("%v.\n* Are you trying to connect to a TLS-enabled daemon without TLS?", err)
		}
		if cli.Scheme == "https" && strings.Contains(err.Error(), "remote error: bad certificate") {
			return serverResp, fmt.Errorf("The server probably has client authentication (--tlsverify) enabled. Please check your TLS client certification settings: %v", err)
		}

		return serverResp, fmt.Errorf("An error occurred trying to connect: %v", err)
	}

	if serverResp.statusCode < 200 || serverResp.statusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return serverResp, err
		}
		if len(body) == 0 {
			return serverResp, fmt.Errorf("Error: request returned %s for API route and version %s, check if the server supports the requested API version", http.StatusText(serverResp.statusCode), req.URL)
		}
		return serverResp, fmt.Errorf("Error response from daemon: %s", bytes.TrimSpace(body))
	}

	serverResp.body = resp.Body
	serverResp.header = resp.Header
	return serverResp, nil
}

func encodeData(data interface{}) (*bytes.Buffer, error) {
	params := bytes.NewBuffer(nil)
	if data != nil {
		if err := json.NewEncoder(params).Encode(data); err != nil {
			return nil, err
		}
	}
	return params, nil
}

<<<<<<< HEAD
func ensureReaderClosed(response *ServerResponse) {
=======
func ensureReaderClosed(response *serverResponse) {
>>>>>>> 9c13063... Implement docker network with standalone client lib.
	if response != nil && response.body != nil {
		response.body.Close()
	}
}
