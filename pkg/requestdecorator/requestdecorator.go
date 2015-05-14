// Package requestdecorator provides helper functions to decorate a request with
// user agent versions, auth, meta headers.
package requestdecorator

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
)

var (
	ErrNilRequest = errors.New("request cannot be nil")
)

// UAVersionInfo is used to model UserAgent versions.
type UAVersionInfo struct {
	Name    string
	Version string
}

func NewUAVersionInfo(name, version string) UAVersionInfo {
	return UAVersionInfo{
		Name:    name,
		Version: version,
	}
}

func (vi *UAVersionInfo) isValid() bool {
	const stopChars = " \t\r\n/"
	name := vi.Name
	vers := vi.Version
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
// Each UAVersionInfo will be converted to a string in the format of
// "product/version", where the "product" is get from the name field, while
// version is get from the version field. Several pieces of verson information
// will be concatinated and separated by space.
func AppendVersions(base string, versions ...UAVersionInfo) string {
	if len(versions) == 0 {
		return base
	}

	verstrs := make([]string, 0, 1+len(versions))
	if len(base) > 0 {
		verstrs = append(verstrs, base)
	}

	for _, v := range versions {
		if !v.isValid() {
			continue
		}
		verstrs = append(verstrs, v.Name+"/"+v.Version)
	}
	return strings.Join(verstrs, " ")
}

// Decorator is used to change an instance of
// http.Request. It could be used to add more header fields,
// change body, etc.
type Decorator interface {
	// ChangeRequest() changes the request accordingly.
	// The changed request will be returned or err will be non-nil
	// if an error occur.
	ChangeRequest(req *http.Request) (newReq *http.Request, err error)
}

// UserAgentDecorator appends the product/version to the user agent field
// of a request.
type UserAgentDecorator struct {
	Versions []UAVersionInfo
}

func (h *UserAgentDecorator) ChangeRequest(req *http.Request) (*http.Request, error) {
	if req == nil {
		return req, ErrNilRequest
	}

	userAgent := AppendVersions(req.UserAgent(), h.Versions...)
	if len(userAgent) > 0 {
		req.Header.Set("User-Agent", userAgent)
	}
	return req, nil
}

type MetaHeadersDecorator struct {
	Headers map[string][]string
}

func (h *MetaHeadersDecorator) ChangeRequest(req *http.Request) (*http.Request, error) {
	if h.Headers == nil {
		return req, ErrNilRequest
	}
	for k, v := range h.Headers {
		req.Header[k] = v
	}
	return req, nil
}

type AuthDecorator struct {
	login    string
	password string
}

func NewAuthDecorator(login, password string) Decorator {
	return &AuthDecorator{
		login:    login,
		password: password,
	}
}

func (self *AuthDecorator) ChangeRequest(req *http.Request) (*http.Request, error) {
	if req == nil {
		return req, ErrNilRequest
	}
	req.SetBasicAuth(self.login, self.password)
	return req, nil
}

// RequestFactory creates an HTTP request
// and applies a list of decorators on the request.
type RequestFactory struct {
	decorators []Decorator
}

func NewRequestFactory(d ...Decorator) *RequestFactory {
	return &RequestFactory{
		decorators: d,
	}
}

func (f *RequestFactory) AddDecorator(d ...Decorator) {
	f.decorators = append(f.decorators, d...)
}

func (f *RequestFactory) GetDecorators() []Decorator {
	return f.decorators
}

// NewRequest() creates a new *http.Request,
// applies all decorators in the Factory on the request,
// then applies decorators provided by d on the request.
func (h *RequestFactory) NewRequest(method, urlStr string, body io.Reader, d ...Decorator) (*http.Request, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}

	// By default, a nil factory should work.
	if h == nil {
		return req, nil
	}
	for _, dec := range h.decorators {
		req, _ = dec.ChangeRequest(req)
	}
	for _, dec := range d {
		req, _ = dec.ChangeRequest(req)
	}
	logrus.Debugf("%v -- HEADERS: %v", req.URL, req.Header)
	return req, err
}
