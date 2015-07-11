package transport

import (
	"io"
	"net/http"
	"sync"
)

type RequestModifier interface {
	ModifyRequest(*http.Request) error
}

type headerModifier http.Header

// NewHeaderRequestModifier returns a RequestModifier that merges the HTTP headers
// passed as an argument, with the HTTP headers of a request.
//
// If the same key is present in both, the modifying header values for that key,
// are appended to the values for that same key in the request header.
func NewHeaderRequestModifier(header http.Header) RequestModifier {
	return headerModifier(header)
}

func (h headerModifier) ModifyRequest(req *http.Request) error {
	for k, s := range http.Header(h) {
		req.Header[k] = append(req.Header[k], s...)
	}

	return nil
}

// NewTransport returns an http.RoundTripper that modifies requests according to
// the RequestModifiers passed in the arguments, before sending the requests to
// the base http.RoundTripper (which, if nil, defaults to http.DefaultTransport).
func NewTransport(base http.RoundTripper, modifiers ...RequestModifier) http.RoundTripper {
	return &transport{
		Modifiers: modifiers,
		Base:      base,
	}
}

// transport is an http.RoundTripper that makes HTTP requests after
// copying and modifying the request
type transport struct {
	Modifiers []RequestModifier
	Base      http.RoundTripper

	mu     sync.Mutex                      // guards modReq
	modReq map[*http.Request]*http.Request // original -> modified
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := CloneRequest(req)
	for _, modifier := range t.Modifiers {
		if err := modifier.ModifyRequest(req2); err != nil {
			return nil, err
		}
	}

	t.setModReq(req, req2)
	res, err := t.base().RoundTrip(req2)
	if err != nil {
		t.setModReq(req, nil)
		return nil, err
	}
	res.Body = &OnEOFReader{
		Rc: res.Body,
		Fn: func() { t.setModReq(req, nil) },
	}
	return res, nil
}

// CancelRequest cancels an in-flight request by closing its connection.
func (t *transport) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := t.base().(canceler); ok {
		t.mu.Lock()
		modReq := t.modReq[req]
		delete(t.modReq, req)
		t.mu.Unlock()
		cr.CancelRequest(modReq)
	}
}

func (t *transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *transport) setModReq(orig, mod *http.Request) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.modReq == nil {
		t.modReq = make(map[*http.Request]*http.Request)
	}
	if mod == nil {
		delete(t.modReq, orig)
	} else {
		t.modReq[orig] = mod
	}
}

// CloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func CloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}

	return r2
}

// OnEOFReader ensures a callback function is called
// on Close() and when the underlying Reader returns an io.EOF error
type OnEOFReader struct {
	Rc io.ReadCloser
	Fn func()
}

func (r *OnEOFReader) Read(p []byte) (n int, err error) {
	n, err = r.Rc.Read(p)
	if err == io.EOF {
		r.runFunc()
	}
	return
}

func (r *OnEOFReader) Close() error {
	err := r.Rc.Close()
	r.runFunc()
	return err
}

func (r *OnEOFReader) runFunc() {
	if fn := r.Fn; fn != nil {
		fn()
		r.Fn = nil
	}
}
