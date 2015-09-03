package store

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"

	"github.com/Sirupsen/logrus"
)

type ErrServerUnavailable struct {
	code int
}

func (err ErrServerUnavailable) Error() string {
	return fmt.Sprintf("Unable to reach trust server at this time: %d.", err.code)
}

type ErrShortRead struct{}

func (err ErrShortRead) Error() string {
	return "Trust server returned incompelete response."
}

type ErrMaliciousServer struct{}

func (err ErrMaliciousServer) Error() string {
	return "Trust server returned a bad response."
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
	targetsPrefix string
	keyExtension  string
	roundTrip     http.RoundTripper
}

func NewHTTPStore(baseURL, metaPrefix, metaExtension, targetsPrefix, keyExtension string, roundTrip http.RoundTripper) (*HTTPStore, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if !base.IsAbs() {
		return nil, errors.New("HTTPStore requires an absolute baseURL")
	}
	return &HTTPStore{
		baseURL:       *base,
		metaPrefix:    metaPrefix,
		metaExtension: metaExtension,
		targetsPrefix: targetsPrefix,
		keyExtension:  keyExtension,
		roundTrip:     roundTrip,
	}, nil
}

// GetMeta downloads the named meta file with the given size. A short body
// is acceptable because in the case of timestamp.json, the size is a cap,
// not an exact length.
func (s HTTPStore) GetMeta(name string, size int64) ([]byte, error) {
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
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrMetaNotFound{}
	} else if resp.StatusCode != http.StatusOK {
		return nil, ErrServerUnavailable{code: resp.StatusCode}
	}
	if resp.ContentLength > size {
		return nil, ErrMaliciousServer{}
	}
	logrus.Debugf("%d when retrieving metadata for %s", resp.StatusCode, name)
	b := io.LimitReader(resp.Body, size)
	body, err := ioutil.ReadAll(b)
	if resp.ContentLength > 0 && int64(len(body)) < resp.ContentLength {
		return nil, ErrShortRead{}
	}

	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s HTTPStore) SetMeta(name string, blob []byte) error {
	url, err := s.buildMetaURL("")
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url.String(), bytes.NewReader(blob))
	if err != nil {
		return err
	}
	resp, err := s.roundTrip.RoundTrip(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrMetaNotFound{}
	} else if resp.StatusCode != http.StatusOK {
		return ErrServerUnavailable{code: resp.StatusCode}
	}
	return nil
}

func (s HTTPStore) SetMultiMeta(metas map[string][]byte) error {
	url, err := s.buildMetaURL("")
	if err != nil {
		return err
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for role, blob := range metas {
		part, err := writer.CreateFormFile("files", role)
		_, err = io.Copy(part, bytes.NewBuffer(blob))
		if err != nil {
			return err
		}
	}
	err = writer.Close()
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url.String(), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err != nil {
		return err
	}
	resp, err := s.roundTrip.RoundTrip(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrMetaNotFound{}
	} else if resp.StatusCode != http.StatusOK {
		return ErrServerUnavailable{code: resp.StatusCode}
	}
	return nil
}

func (s HTTPStore) buildMetaURL(name string) (*url.URL, error) {
	var filename string
	if name != "" {
		filename = fmt.Sprintf("%s.%s", name, s.metaExtension)
	}
	uri := path.Join(s.metaPrefix, filename)
	return s.buildURL(uri)
}

func (s HTTPStore) buildTargetsURL(name string) (*url.URL, error) {
	uri := path.Join(s.targetsPrefix, name)
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

// GetTarget returns a reader for the desired target or an error.
// N.B. The caller is responsible for closing the reader.
func (s HTTPStore) GetTarget(path string) (io.ReadCloser, error) {
	url, err := s.buildTargetsURL(path)
	if err != nil {
		return nil, err
	}
	logrus.Debug("Attempting to download target: ", url.String())
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.roundTrip.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrMetaNotFound{}
	} else if resp.StatusCode != http.StatusOK {
		return nil, ErrServerUnavailable{code: resp.StatusCode}
	}
	return resp.Body, nil
}

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
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrMetaNotFound{}
	} else if resp.StatusCode != http.StatusOK {
		return nil, ErrServerUnavailable{code: resp.StatusCode}
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
