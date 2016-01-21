package authorization

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

// ResponseModifier allows authorization plugins to read and modify the content of the http.response
type ResponseModifier interface {
	http.ResponseWriter

	// RawBody returns the current http content
	RawBody() []byte

	// RawHeaders returns the current content of the http headers
	RawHeaders() ([]byte, error)

	// StatusCode returns the current status code
	StatusCode() int

	// OverrideBody replace the body of the HTTP reply
	OverrideBody(b []byte)

	// OverrideHeader replace the headers of the HTTP reply
	OverrideHeader(b []byte) error

	// OverrideStatusCode replaces the status code of the HTTP reply
	OverrideStatusCode(statusCode int)

	// Flush flushes all data to the HTTP response
	Flush() error
}

// NewResponseModifier creates a wrapper to an http.ResponseWriter to allow inspecting and modifying the content
func NewResponseModifier(rw http.ResponseWriter) ResponseModifier {
	return &responseModifier{rw: rw, header: make(http.Header)}
}

// responseModifier is used as an adapter to http.ResponseWriter in order to manipulate and explore
// the http request/response from docker daemon
type responseModifier struct {
	// The original response writer
	rw     http.ResponseWriter
	status int
	// body holds the response body
	body []byte
	// header holds the response header
	header http.Header
	// statusCode holds the response status code
	statusCode int
}

// WriterHeader stores the http status code
func (rm *responseModifier) WriteHeader(s int) {
	rm.statusCode = s
}

// Header returns the internal http header
func (rm *responseModifier) Header() http.Header {
	return rm.header
}

// Header returns the internal http header
func (rm *responseModifier) StatusCode() int {
	return rm.statusCode
}

// Override replace the body of the HTTP reply
func (rm *responseModifier) OverrideBody(b []byte) {
	rm.body = b
}

func (rm *responseModifier) OverrideStatusCode(statusCode int) {
	rm.statusCode = statusCode
}

// Override replace the headers of the HTTP reply
func (rm *responseModifier) OverrideHeader(b []byte) error {
	header := http.Header{}
	if err := json.Unmarshal(b, &header); err != nil {
		return err
	}
	rm.header = header
	return nil
}

// Write stores the byte array inside content
func (rm *responseModifier) Write(b []byte) (int, error) {
	rm.body = append(rm.body, b...)
	return len(b), nil
}

// Body returns the response body
func (rm *responseModifier) RawBody() []byte {
	return rm.body
}

func (rm *responseModifier) RawHeaders() ([]byte, error) {
	var b bytes.Buffer
	if err := rm.header.Write(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Hijack returns the internal connection of the wrapped http.ResponseWriter
func (rm *responseModifier) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rm.rw.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("Internal reponse writer doesn't support the Hijacker interface")
	}
	return hijacker.Hijack()
}

// Flush flushes all data to the HTTP response
func (rm *responseModifier) Flush() error {
	// Copy the status code
	if rm.statusCode > 0 {
		rm.rw.WriteHeader(rm.statusCode)
	}

	// Copy the header
	for k, vv := range rm.header {
		for _, v := range vv {
			rm.rw.Header().Add(k, v)
		}
	}

	// Write body
	_, err := rm.rw.Write(rm.body)
	return err
}
