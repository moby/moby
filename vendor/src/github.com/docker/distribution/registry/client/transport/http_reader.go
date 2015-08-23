package transport

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

// ReadSeekCloser combines io.ReadSeeker with io.Closer.
type ReadSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

// NewHTTPReadSeeker handles reading from an HTTP endpoint using a GET
// request. When seeking and starting a read from a non-zero offset
// the a "Range" header will be added which sets the offset.
// TODO(dmcgowan): Move this into a separate utility package
func NewHTTPReadSeeker(client *http.Client, url string, size int64) ReadSeekCloser {
	return &httpReadSeeker{
		client: client,
		url:    url,
		size:   size,
	}
}

type httpReadSeeker struct {
	client *http.Client
	url    string

	size int64

	rc     io.ReadCloser // remote read closer
	brd    *bufio.Reader // internal buffered io
	offset int64
	err    error
}

func (hrs *httpReadSeeker) Read(p []byte) (n int, err error) {
	if hrs.err != nil {
		return 0, hrs.err
	}

	rd, err := hrs.reader()
	if err != nil {
		return 0, err
	}

	n, err = rd.Read(p)
	hrs.offset += int64(n)

	// Simulate io.EOF error if we reach filesize.
	if err == nil && hrs.offset >= hrs.size {
		err = io.EOF
	}

	return n, err
}

func (hrs *httpReadSeeker) Seek(offset int64, whence int) (int64, error) {
	if hrs.err != nil {
		return 0, hrs.err
	}

	var err error
	newOffset := hrs.offset

	switch whence {
	case os.SEEK_CUR:
		newOffset += int64(offset)
	case os.SEEK_END:
		newOffset = hrs.size + int64(offset)
	case os.SEEK_SET:
		newOffset = int64(offset)
	}

	if newOffset < 0 {
		err = errors.New("cannot seek to negative position")
	} else {
		if hrs.offset != newOffset {
			hrs.reset()
		}

		// No problems, set the offset.
		hrs.offset = newOffset
	}

	return hrs.offset, err
}

func (hrs *httpReadSeeker) Close() error {
	if hrs.err != nil {
		return hrs.err
	}

	// close and release reader chain
	if hrs.rc != nil {
		hrs.rc.Close()
	}

	hrs.rc = nil
	hrs.brd = nil

	hrs.err = errors.New("httpLayer: closed")

	return nil
}

func (hrs *httpReadSeeker) reset() {
	if hrs.err != nil {
		return
	}
	if hrs.rc != nil {
		hrs.rc.Close()
		hrs.rc = nil
	}
}

func (hrs *httpReadSeeker) reader() (io.Reader, error) {
	if hrs.err != nil {
		return nil, hrs.err
	}

	if hrs.rc != nil {
		return hrs.brd, nil
	}

	// If the offset is great than or equal to size, return a empty, noop reader.
	if hrs.offset >= hrs.size {
		return ioutil.NopCloser(bytes.NewReader([]byte{})), nil
	}

	req, err := http.NewRequest("GET", hrs.url, nil)
	if err != nil {
		return nil, err
	}

	if hrs.offset > 0 {
		// TODO(stevvooe): Get this working correctly.

		// If we are at different offset, issue a range request from there.
		req.Header.Add("Range", "1-")
		// TODO: get context in here
		// context.GetLogger(hrs.context).Infof("Range: %s", req.Header.Get("Range"))
	}

	resp, err := hrs.client.Do(req)
	if err != nil {
		return nil, err
	}

	// Normally would use client.SuccessStatus, but that would be a cyclic
	// import
	if resp.StatusCode >= 200 && resp.StatusCode <= 399 {
		hrs.rc = resp.Body
	} else {
		defer resp.Body.Close()
		return nil, fmt.Errorf("unexpected status resolving reader: %v", resp.Status)
	}

	if hrs.brd == nil {
		hrs.brd = bufio.NewReader(hrs.rc)
	} else {
		hrs.brd.Reset(hrs.rc)
	}

	return hrs.brd, nil
}
