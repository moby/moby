package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// serverResponse is a wrapper for http API responses.
type serverResponse struct {
	body       io.ReadCloser
	header     http.Header
	statusCode int
	reqURL     *url.URL
}

// versionedClient is used to perform API requests and other version-dependent
// operations at a particular API version.
type versionedClient struct {
	cli     *Client
	version string
}

// versioned returns a new versionedClient, negotiating API version if
// necessary.
//
// Client methods which require access to the currently-configured API version
// should construct a versionedClient and access its version field or call its
// methods. The same versionedClient should be used for the entire duration of a
// request to ensure that the same version is consistently used, even if the
// client's configured version is concurrently modified.
func (cli *Client) versioned(ctx context.Context) (versionedClient, error) {
	ver := cli.negotiateAPIVersion(ctx, false)
	return versionedClient{cli: cli, version: ver}, ctx.Err()
}

// head sends an HTTP HEAD request to the docker API.
func (cli versionedClient) head(ctx context.Context, path string, query url.Values, headers map[string][]string) (serverResponse, error) {
	return cli.sendRequest(ctx, http.MethodHead, path, query, nil, headers)
}

// head sends an HTTP HEAD request to the docker API using a temporary
// versionedClient. Client methods which need to know the version while
// preparing the request or handling the response should explicitly construct a
// versionedClient and make HTTP requests through it directly instead of using
// this convenience wrapper.
func (cli *Client) head(ctx context.Context, path string, query url.Values, headers map[string][]string) (serverResponse, error) {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return serverResponse{}, err
	}
	return versioned.head(ctx, path, query, headers)
}

// get sends an HTTP GET request to the docker API.
func (cli versionedClient) get(ctx context.Context, path string, query url.Values, headers map[string][]string) (serverResponse, error) {
	return cli.sendRequest(ctx, http.MethodGet, path, query, nil, headers)
}

// get sends an HTTP GET request to the docker API using a temporary
// versionedClient. Client methods which need to know the version while
// preparing the request or handling the response should explicitly construct a
// versionedClient and make HTTP requests through it directly instead of using
// this convenience wrapper.
func (cli *Client) get(ctx context.Context, path string, query url.Values, headers map[string][]string) (serverResponse, error) {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return serverResponse{}, err
	}
	return versioned.get(ctx, path, query, headers)
}

// post sends an HTTP POST request to the docker API with obj as the request
// body, encoded to JSON.
func (cli versionedClient) post(ctx context.Context, path string, query url.Values, obj interface{}, headers map[string][]string) (serverResponse, error) {
	body, headers, err := encodeBody(obj, headers)
	if err != nil {
		return serverResponse{}, err
	}
	return cli.postRaw(ctx, path, query, body, headers)
}

// post sends an HTTP POST request to the docker API using a temporary
// versionedClient. Client methods which need to know the version while
// preparing the request or handling the response should explicitly construct a
// versionedClient and make HTTP requests through it directly instead of using
// this convenience wrapper.
func (cli *Client) post(ctx context.Context, path string, query url.Values, obj interface{}, headers map[string][]string) (serverResponse, error) {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return serverResponse{}, err
	}
	return versioned.post(ctx, path, query, obj, headers)
}

// postRaw sends an HTTP POST request to the docker API.
func (cli versionedClient) postRaw(ctx context.Context, path string, query url.Values, body io.Reader, headers map[string][]string) (serverResponse, error) {
	return cli.sendRequest(ctx, http.MethodPost, path, query, body, headers)
}

// postRaw sends an HTTP POST request to the docker API using a temporary
// versionedClient. Client methods which need to know the version while
// preparing the request or handling the response should explicitly construct a
// versionedClient and make HTTP requests through it directly instead of using
// this convenience wrapper.
func (cli *Client) postRaw(ctx context.Context, path string, query url.Values, body io.Reader, headers map[string][]string) (serverResponse, error) {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return serverResponse{}, err
	}
	return versioned.postRaw(ctx, path, query, body, headers)
}

// put sends an HTTP PUT request to the docker API with obj as the request body,
// encoded to JSON.
func (cli versionedClient) put(ctx context.Context, path string, query url.Values, obj interface{}, headers map[string][]string) (serverResponse, error) {
	body, headers, err := encodeBody(obj, headers)
	if err != nil {
		return serverResponse{}, err
	}
	return cli.sendRequest(ctx, http.MethodPut, path, query, body, headers)
}

// putRaw sends an HTTP PUT request to the docker API.
func (cli versionedClient) putRaw(ctx context.Context, path string, query url.Values, body io.Reader, headers map[string][]string) (serverResponse, error) {
	return cli.sendRequest(ctx, http.MethodPut, path, query, body, headers)
}

// putRaw sends an HTTP PUT request to the docker API using a temporary
// versionedClient. Client methods which need to know the version while
// preparing the request or handling the response should explicitly construct a
// versionedClient and make HTTP requests through it directly instead of using
// this convenience wrapper.
func (cli *Client) putRaw(ctx context.Context, path string, query url.Values, body io.Reader, headers map[string][]string) (serverResponse, error) {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return serverResponse{}, err
	}
	return versioned.putRaw(ctx, path, query, body, headers)
}

// delete sends an HTTP DELETE request to the docker API.
func (cli versionedClient) delete(ctx context.Context, path string, query url.Values, headers map[string][]string) (serverResponse, error) {
	return cli.sendRequest(ctx, http.MethodDelete, path, query, nil, headers)
}

// delete sends an HTTP DELETE request to the docker API using a temporary
// versionedClient. Client methods which need to know the version while
// preparing the request or handling the response should explicitly construct a
// versionedClient and make HTTP requests through it directly instead of using
// this convenience wrapper.
func (cli *Client) delete(ctx context.Context, path string, query url.Values, headers map[string][]string) (serverResponse, error) {
	versioned, err := cli.versioned(ctx)
	if err != nil {
		return serverResponse{}, err
	}
	return versioned.delete(ctx, path, query, headers)
}

type headers map[string][]string

func encodeBody(obj interface{}, headers headers) (io.Reader, headers, error) {
	if obj == nil {
		return nil, headers, nil
	}

	body, err := encodeData(obj)
	if err != nil {
		return nil, headers, err
	}
	if headers == nil {
		headers = make(map[string][]string)
	}
	headers["Content-Type"] = []string{"application/json"}
	return body, headers, nil
}

func (cli versionedClient) buildRequest(method, path string, body io.Reader, headers headers) (*http.Request, error) {
	expectedPayload := (method == http.MethodPost || method == http.MethodPut)
	if expectedPayload && body == nil {
		body = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	req = cli.addHeaders(req, headers)

	if cli.cli.proto == "unix" || cli.cli.proto == "npipe" {
		// For local communications, it doesn't matter what the host is. We just
		// need a valid and meaningful host name. (See #189)
		req.Host = "docker"
	}

	req.URL.Host = cli.cli.addr
	req.URL.Scheme = cli.cli.scheme

	if expectedPayload && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "text/plain")
	}
	return req, nil
}

func (cli versionedClient) sendRequest(ctx context.Context, method, path string, query url.Values, body io.Reader, headers headers) (serverResponse, error) {
	req, err := cli.buildRequest(method, cli.getAPIPath(path, query), body, headers)
	if err != nil {
		return serverResponse{}, err
	}

	resp, err := cli.cli.doRequest(ctx, req)
	switch {
	case errors.Is(err, context.Canceled):
		return serverResponse{}, errdefs.Cancelled(err)
	case errors.Is(err, context.DeadlineExceeded):
		return serverResponse{}, errdefs.Deadline(err)
	case err == nil:
		err = cli.checkResponseErr(resp)
	}
	return resp, errdefs.FromStatusCode(err, resp.statusCode)
}

func (cli *Client) doRequest(ctx context.Context, req *http.Request) (serverResponse, error) {
	serverResp := serverResponse{statusCode: -1, reqURL: req.URL}

	req = req.WithContext(ctx)
	resp, err := cli.client.Do(req)
	if err != nil {
		if cli.scheme != "https" && strings.Contains(err.Error(), "malformed HTTP response") {
			return serverResp, fmt.Errorf("%v.\n* Are you trying to connect to a TLS-enabled daemon without TLS?", err)
		}

		if cli.scheme == "https" && strings.Contains(err.Error(), "bad certificate") {
			return serverResp, errors.Wrap(err, "The server probably has client authentication (--tlsverify) enabled. Please check your TLS client certification settings")
		}

		// Don't decorate context sentinel errors; users may be comparing to
		// them directly.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return serverResp, err
		}

		if nErr, ok := err.(*url.Error); ok {
			if nErr, ok := nErr.Err.(*net.OpError); ok {
				if os.IsPermission(nErr.Err) {
					return serverResp, errors.Wrapf(err, "Got permission denied while trying to connect to the Docker daemon socket at %v", cli.host)
				}
			}
		}

		if err, ok := err.(net.Error); ok {
			if err.Timeout() {
				return serverResp, ErrorConnectionFailed(cli.host)
			}
			if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "dial unix") {
				return serverResp, ErrorConnectionFailed(cli.host)
			}
		}

		// Although there's not a strongly typed error for this in go-winio,
		// lots of people are using the default configuration for the docker
		// daemon on Windows where the daemon is listening on a named pipe
		// `//./pipe/docker_engine, and the client must be running elevated.
		// Give users a clue rather than the not-overly useful message
		// such as `error during connect: Get http://%2F%2F.%2Fpipe%2Fdocker_engine/v1.26/info:
		// open //./pipe/docker_engine: The system cannot find the file specified.`.
		// Note we can't string compare "The system cannot find the file specified" as
		// this is localised - for example in French the error would be
		// `open //./pipe/docker_engine: Le fichier spécifié est introuvable.`
		if strings.Contains(err.Error(), `open //./pipe/docker_engine`) {
			// Checks if client is running with elevated privileges
			if f, elevatedErr := os.Open("\\\\.\\PHYSICALDRIVE0"); elevatedErr == nil {
				err = errors.Wrap(err, "In the default daemon configuration on Windows, the docker client must be run with elevated privileges to connect.")
			} else {
				f.Close()
				err = errors.Wrap(err, "This error may indicate that the docker daemon is not running.")
			}
		}

		return serverResp, errors.Wrap(err, "error during connect")
	}

	if resp != nil {
		serverResp.statusCode = resp.StatusCode
		serverResp.body = resp.Body
		serverResp.header = resp.Header
	}
	return serverResp, nil
}

func (cli versionedClient) checkResponseErr(serverResp serverResponse) error {
	if serverResp.statusCode >= 200 && serverResp.statusCode < 400 {
		return nil
	}

	var body []byte
	var err error
	if serverResp.body != nil {
		bodyMax := 1 * 1024 * 1024 // 1 MiB
		bodyR := &io.LimitedReader{
			R: serverResp.body,
			N: int64(bodyMax),
		}
		body, err = io.ReadAll(bodyR)
		if err != nil {
			return err
		}
		if bodyR.N == 0 {
			return fmt.Errorf("request returned %s with a message (> %d bytes) for API route and version %s, check if the server supports the requested API version", http.StatusText(serverResp.statusCode), bodyMax, serverResp.reqURL)
		}
	}
	if len(body) == 0 {
		return fmt.Errorf("request returned %s for API route and version %s, check if the server supports the requested API version", http.StatusText(serverResp.statusCode), serverResp.reqURL)
	}

	var ct string
	if serverResp.header != nil {
		ct = serverResp.header.Get("Content-Type")
	}

	var errorMessage string
	if (cli.version == "" || versions.GreaterThan(cli.version, "1.23")) && ct == "application/json" {
		var errorResponse types.ErrorResponse
		if err := json.Unmarshal(body, &errorResponse); err != nil {
			return errors.Wrap(err, "Error reading JSON")
		}
		errorMessage = strings.TrimSpace(errorResponse.Message)
	} else {
		errorMessage = strings.TrimSpace(string(body))
	}

	return errors.Wrap(errors.New(errorMessage), "Error response from daemon")
}

func (cli versionedClient) addHeaders(req *http.Request, headers headers) *http.Request {
	// Add CLI Config's HTTP Headers BEFORE we set the Docker headers
	// then the user can't change OUR headers
	for k, v := range cli.cli.customHTTPHeaders {
		if versions.LessThan(cli.version, "1.25") && http.CanonicalHeaderKey(k) == "User-Agent" {
			continue
		}
		req.Header.Set(k, v)
	}

	for k, v := range headers {
		req.Header[http.CanonicalHeaderKey(k)] = v
	}
	return req
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

func ensureReaderClosed(response serverResponse) {
	if response.body != nil {
		// Drain up to 512 bytes and close the body to let the Transport reuse the connection
		io.CopyN(io.Discard, response.body, 512)
		response.body.Close()
	}
}
