// A Store that can fetch and set metadata on a remote server.
// Some API constraints:
// - Response bodies for error codes should be unmarshallable as:
//   {"errors": [{..., "detail": <serialized validation error>}]}
//   else validation error details, etc. will be unparsable.  The errors
//   should have a github.com/docker/notary/tuf/validation/SerializableError
//   in the Details field.
//   If writing your own server, please have a look at
//   github.com/docker/distribution/registry/api/errcode

package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary"
	"github.com/docker/notary/tuf/validation"
)

// ErrServerUnavailable indicates an error from the server. code allows us to
// populate the http error we received
type ErrServerUnavailable struct {
	code int
}

// NetworkError represents any kind of network error when attempting to make a request
type NetworkError struct {
	Wrapped error
}

func (n NetworkError) Error() string {
	return n.Wrapped.Error()
}

func (err ErrServerUnavailable) Error() string {
	if err.code == 401 {
		return fmt.Sprintf("you are not authorized to perform this operation: server returned 401.")
	}
	return fmt.Sprintf("unable to reach trust server at this time: %d.", err.code)
}

// ErrMaliciousServer indicates the server returned a response that is highly suspected
// of being malicious. i.e. it attempted to send us more data than the known size of a
// particular role metadata.
type ErrMaliciousServer struct{}

func (err ErrMaliciousServer) Error() string {
	return "trust server returned a bad response."
}

// ErrInvalidOperation indicates that the server returned a 400 response and
// propagate any body we received.
type ErrInvalidOperation struct {
	msg string
}

func (err ErrInvalidOperation) Error() string {
	if err.msg != "" {
		return fmt.Sprintf("trust server rejected operation: %s", err.msg)
	}
	return "trust server rejected operation."
}

// HTTPStore manages pulling and pushing metadata from and to a remote
// service over HTTP. It assumes the URL structure of the remote service
// maps identically to the structure of the TUF repo:
// <baseURL>/<metaPrefix>/(root|targets|snapshot|timestamp).json
// <baseURL>/<targetsPrefix>/foo.sh
//
// If consistent snapshots are disabled, it is advised that caching is not
// enabled. Simple set a cachePath (and ensure it's writeable) to enable
// caching.
type HTTPStore struct {
	baseURL       url.URL
	metaPrefix    string
	metaExtension string
	keyExtension  string
	roundTrip     http.RoundTripper
}

// NewHTTPStore initializes a new store against a URL and a number of configuration options
func NewHTTPStore(baseURL, metaPrefix, metaExtension, keyExtension string, roundTrip http.RoundTripper) (RemoteStore, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if !base.IsAbs() {
		return nil, errors.New("HTTPStore requires an absolute baseURL")
	}
	if roundTrip == nil {
		return &OfflineStore{}, nil
	}
	return &HTTPStore{
		baseURL:       *base,
		metaPrefix:    metaPrefix,
		metaExtension: metaExtension,
		keyExtension:  keyExtension,
		roundTrip:     roundTrip,
	}, nil
}

func tryUnmarshalError(resp *http.Response, defaultError error) error {
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return defaultError
	}
	var parsedErrors struct {
		Errors []struct {
			Detail validation.SerializableError `json:"detail"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(bodyBytes, &parsedErrors); err != nil {
		return defaultError
	}
	if len(parsedErrors.Errors) != 1 {
		return defaultError
	}
	err = parsedErrors.Errors[0].Detail.Error
	if err == nil {
		return defaultError
	}
	return err
}

func translateStatusToError(resp *http.Response, resource string) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrMetaNotFound{Resource: resource}
	case http.StatusBadRequest:
		return tryUnmarshalError(resp, ErrInvalidOperation{})
	default:
		return ErrServerUnavailable{code: resp.StatusCode}
	}
}

// GetSized downloads the named meta file with the given size. A short body
// is acceptable because in the case of timestamp.json, the size is a cap,
// not an exact length.
// If size is "NoSizeLimit", this corresponds to "infinite," but we cut off at a
// predefined threshold "notary.MaxDownloadSize".
func (s HTTPStore) GetSized(name string, size int64) ([]byte, error) {
	url, err := s.buildMetaURL(name)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.roundTrip.RoundTrip(req)
	if err != nil {
		return nil, NetworkError{Wrapped: err}
	}
	defer resp.Body.Close()
	if err := translateStatusToError(resp, name); err != nil {
		logrus.Debugf("received HTTP status %d when requesting %s.", resp.StatusCode, name)
		return nil, err
	}
	if size == NoSizeLimit {
		size = notary.MaxDownloadSize
	}
	if resp.ContentLength > size {
		return nil, ErrMaliciousServer{}
	}
	logrus.Debugf("%d when retrieving metadata for %s", resp.StatusCode, name)
	b := io.LimitReader(resp.Body, size)
	body, err := ioutil.ReadAll(b)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// Set sends a single piece of metadata to the TUF server
func (s HTTPStore) Set(name string, blob []byte) error {
	return s.SetMulti(map[string][]byte{name: blob})
}

// Remove always fails, because we should never be able to delete metadata
// remotely
func (s HTTPStore) Remove(name string) error {
	return ErrInvalidOperation{msg: "cannot delete individual metadata files"}
}

// NewMultiPartMetaRequest builds a request with the provided metadata updates
// in multipart form
func NewMultiPartMetaRequest(url string, metas map[string][]byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for role, blob := range metas {
		part, err := writer.CreateFormFile("files", role)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(part, bytes.NewBuffer(blob))
		if err != nil {
			return nil, err
		}
	}
	err := writer.Close()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

// SetMulti does a single batch upload of multiple pieces of TUF metadata.
// This should be preferred for updating a remote server as it enable the server
// to remain consistent, either accepting or rejecting the complete update.
func (s HTTPStore) SetMulti(metas map[string][]byte) error {
	url, err := s.buildMetaURL("")
	if err != nil {
		return err
	}
	req, err := NewMultiPartMetaRequest(url.String(), metas)
	if err != nil {
		return err
	}
	resp, err := s.roundTrip.RoundTrip(req)
	if err != nil {
		return NetworkError{Wrapped: err}
	}
	defer resp.Body.Close()
	// if this 404's something is pretty wrong
	return translateStatusToError(resp, "POST metadata endpoint")
}

// RemoveAll will attempt to delete all TUF metadata for a GUN
func (s HTTPStore) RemoveAll() error {
	url, err := s.buildMetaURL("")
	if err != nil {
		return err
	}
	req, err := http.NewRequest("DELETE", url.String(), nil)
	if err != nil {
		return err
	}
	resp, err := s.roundTrip.RoundTrip(req)
	if err != nil {
		return NetworkError{Wrapped: err}
	}
	defer resp.Body.Close()
	return translateStatusToError(resp, "DELETE metadata for GUN endpoint")
}

func (s HTTPStore) buildMetaURL(name string) (*url.URL, error) {
	var filename string
	if name != "" {
		filename = fmt.Sprintf("%s.%s", name, s.metaExtension)
	}
	uri := path.Join(s.metaPrefix, filename)
	return s.buildURL(uri)
}

func (s HTTPStore) buildKeyURL(name string) (*url.URL, error) {
	filename := fmt.Sprintf("%s.%s", name, s.keyExtension)
	uri := path.Join(s.metaPrefix, filename)
	return s.buildURL(uri)
}

func (s HTTPStore) buildURL(uri string) (*url.URL, error) {
	sub, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	return s.baseURL.ResolveReference(sub), nil
}

// GetKey retrieves a public key from the remote server
func (s HTTPStore) GetKey(role string) ([]byte, error) {
	url, err := s.buildKeyURL(role)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.roundTrip.RoundTrip(req)
	if err != nil {
		return nil, NetworkError{Wrapped: err}
	}
	defer resp.Body.Close()
	if err := translateStatusToError(resp, role+" key"); err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// RotateKey rotates a private key and returns the public component from the remote server
func (s HTTPStore) RotateKey(role string) ([]byte, error) {
	url, err := s.buildKeyURL(role)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.roundTrip.RoundTrip(req)
	if err != nil {
		return nil, NetworkError{Wrapped: err}
	}
	defer resp.Body.Close()
	if err := translateStatusToError(resp, role+" key"); err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// Location returns a human readable name for the storage location
func (s HTTPStore) Location() string {
	return s.baseURL.String()
}
