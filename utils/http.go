package utils

import (
	"io"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
)

// VersionInfo is used to model entities which has a version.
// It is basically a tupple with name and version.
type VersionInfo interface {
	Name() string
	Version() string
}

func validVersion(version VersionInfo) bool {
	const stopChars = " \t\r\n/"
	name := version.Name()
	vers := version.Version()
	if len(name) == 0 || strings.ContainsAny(name, stopChars) {
		return false
	}
	if len(vers) == 0 || strings.ContainsAny(vers, stopChars) {
		return false
	}
	return true
}

// Convert versions to a string and append the string to the string base.
//
// Each VersionInfo will be converted to a string in the format of
// "product/version", where the "product" is get from the Name() method, while
// version is get from the Version() method. Several pieces of verson information
// will be concatinated and separated by space.
func appendVersions(base string, versions ...VersionInfo) string {
	if len(versions) == 0 {
		return base
	}

	verstrs := make([]string, 0, 1+len(versions))
	if len(base) > 0 {
		verstrs = append(verstrs, base)
	}

	for _, v := range versions {
		if !validVersion(v) {
			continue
		}
		verstrs = append(verstrs, v.Name()+"/"+v.Version())
	}
	return strings.Join(verstrs, " ")
}

// HTTPRequestDecorator is used to change an instance of
// http.Request. It could be used to add more header fields,
// change body, etc.
type HTTPRequestDecorator interface {
	// ChangeRequest() changes the request accordingly.
	// The changed request will be returned or err will be non-nil
	// if an error occur.
	ChangeRequest(req *http.Request) (newReq *http.Request, err error)
}

// HTTPUserAgentDecorator appends the product/version to the user agent field
// of a request.
type HTTPUserAgentDecorator struct {
	versions []VersionInfo
}

func NewHTTPUserAgentDecorator(versions ...VersionInfo) HTTPRequestDecorator {
	return &HTTPUserAgentDecorator{
		versions: versions,
	}
}

func (h *HTTPUserAgentDecorator) ChangeRequest(req *http.Request) (newReq *http.Request, err error) {
	if req == nil {
		return req, nil
	}

	userAgent := appendVersions(req.UserAgent(), h.versions...)
	if len(userAgent) > 0 {
		req.Header.Set("User-Agent", userAgent)
	}
	return req, nil
}

type HTTPMetaHeadersDecorator struct {
	Headers map[string][]string
}

func (h *HTTPMetaHeadersDecorator) ChangeRequest(req *http.Request) (newReq *http.Request, err error) {
	if h.Headers == nil {
		return req, nil
	}
	for k, v := range h.Headers {
		req.Header[k] = v
	}
	return req, nil
}

type HTTPAuthDecorator struct {
	login    string
	password string
}

func NewHTTPAuthDecorator(login, password string) HTTPRequestDecorator {
	return &HTTPAuthDecorator{
		login:    login,
		password: password,
	}
}

func (self *HTTPAuthDecorator) ChangeRequest(req *http.Request) (*http.Request, error) {
	req.SetBasicAuth(self.login, self.password)
	return req, nil
}

// HTTPRequestFactory creates an HTTP request
// and applies a list of decorators on the request.
type HTTPRequestFactory struct {
	decorators []HTTPRequestDecorator
}

func NewHTTPRequestFactory(d ...HTTPRequestDecorator) *HTTPRequestFactory {
	return &HTTPRequestFactory{
		decorators: d,
	}
}

func (self *HTTPRequestFactory) AddDecorator(d ...HTTPRequestDecorator) {
	self.decorators = append(self.decorators, d...)
}

// NewRequest() creates a new *http.Request,
// applies all decorators in the HTTPRequestFactory on the request,
// then applies decorators provided by d on the request.
func (h *HTTPRequestFactory) NewRequest(method, urlStr string, body io.Reader, d ...HTTPRequestDecorator) (*http.Request, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}

	// By default, a nil factory should work.
	if h == nil {
		return req, nil
	}
	for _, dec := range h.decorators {
		req, err = dec.ChangeRequest(req)
		if err != nil {
			return nil, err
		}
	}
	for _, dec := range d {
		req, err = dec.ChangeRequest(req)
		if err != nil {
			return nil, err
		}
	}
	log.Debugf("%v -- HEADERS: %v", req.URL, req.Header)
	return req, err
}
