package utils

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// VersionInfo is used to model entities which has a version.
// It is basically a tupple with name and version.
type VersionInfo interface {
	Name() string
	Version() string
}

func validVersion(version VersionInfo) bool {
	stopChars := " \t\r\n/"
	if strings.ContainsAny(version.Name(), stopChars) {
		return false
	}
	if strings.ContainsAny(version.Version(), stopChars) {
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

	var buf bytes.Buffer
	if len(base) > 0 {
		buf.Write([]byte(base))
	}

	for _, v := range versions {
		name := []byte(v.Name())
		version := []byte(v.Version())

		if len(name) == 0 || len(version) == 0 {
			continue
		}
		if !validVersion(v) {
			continue
		}
		buf.Write([]byte(v.Name()))
		buf.Write([]byte("/"))
		buf.Write([]byte(v.Version()))
		buf.Write([]byte(" "))
	}
	return buf.String()
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
	ret := new(HTTPUserAgentDecorator)
	ret.versions = versions
	return ret
}

func (self *HTTPUserAgentDecorator) ChangeRequest(req *http.Request) (newReq *http.Request, err error) {
	if req == nil {
		return req, nil
	}

	userAgent := appendVersions(req.UserAgent(), self.versions...)
	if len(userAgent) > 0 {
		req.Header.Set("User-Agent", userAgent)
	}
	return req, nil
}

// HTTPRequestFactory creates an HTTP request
// and applies a list of decorators on the request.
type HTTPRequestFactory struct {
	decorators []HTTPRequestDecorator
}

func NewHTTPRequestFactory(d ...HTTPRequestDecorator) *HTTPRequestFactory {
	ret := new(HTTPRequestFactory)
	ret.decorators = d
	return ret
}

// NewRequest() creates a new *http.Request,
// applies all decorators in the HTTPRequestFactory on the request,
// then applies decorators provided by d on the request.
func (self *HTTPRequestFactory) NewRequest(method, urlStr string, body io.Reader, d ...HTTPRequestDecorator) (*http.Request, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}

	// By default, a nil factory should work.
	if self == nil {
		return req, nil
	}
	for _, dec := range self.decorators {
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
	return req, err
}
