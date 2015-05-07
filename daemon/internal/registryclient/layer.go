package client

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
)

type httpLayer struct {
	*layers

	size      int64
	digest    digest.Digest
	createdAt time.Time

	rc     io.ReadCloser // remote read closer
	brd    *bufio.Reader // internal buffered io
	offset int64
	err    error
}

func (hl *httpLayer) CreatedAt() time.Time {
	return hl.createdAt
}

func (hl *httpLayer) Digest() digest.Digest {
	return hl.digest
}

func (hl *httpLayer) Read(p []byte) (n int, err error) {
	if hl.err != nil {
		return 0, hl.err
	}

	rd, err := hl.reader()
	if err != nil {
		return 0, err
	}

	n, err = rd.Read(p)
	hl.offset += int64(n)

	// Simulate io.EOR error if we reach filesize.
	if err == nil && hl.offset >= hl.size {
		err = io.EOF
	}

	return n, err
}

func (hl *httpLayer) Seek(offset int64, whence int) (int64, error) {
	if hl.err != nil {
		return 0, hl.err
	}

	var err error
	newOffset := hl.offset

	switch whence {
	case os.SEEK_CUR:
		newOffset += int64(offset)
	case os.SEEK_END:
		newOffset = hl.size + int64(offset)
	case os.SEEK_SET:
		newOffset = int64(offset)
	}

	if newOffset < 0 {
		err = fmt.Errorf("cannot seek to negative position")
	} else {
		if hl.offset != newOffset {
			hl.reset()
		}

		// No problems, set the offset.
		hl.offset = newOffset
	}

	return hl.offset, err
}

func (hl *httpLayer) Close() error {
	if hl.err != nil {
		return hl.err
	}

	// close and release reader chain
	if hl.rc != nil {
		hl.rc.Close()
	}

	hl.rc = nil
	hl.brd = nil

	hl.err = fmt.Errorf("httpLayer: closed")

	return nil
}

func (hl *httpLayer) reset() {
	if hl.err != nil {
		return
	}
	if hl.rc != nil {
		hl.rc.Close()
		hl.rc = nil
	}
}

func (hl *httpLayer) reader() (io.Reader, error) {
	if hl.err != nil {
		return nil, hl.err
	}

	if hl.rc != nil {
		return hl.brd, nil
	}

	// If the offset is great than or equal to size, return a empty, noop reader.
	if hl.offset >= hl.size {
		return ioutil.NopCloser(bytes.NewReader([]byte{})), nil
	}

	blobURL, err := hl.ub.BuildBlobURL(hl.name, hl.digest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", blobURL, nil)
	if err != nil {
		return nil, err
	}

	if hl.offset > 0 {
		// TODO(stevvooe): Get this working correctly.

		// If we are at different offset, issue a range request from there.
		req.Header.Add("Range", fmt.Sprintf("1-"))
		context.GetLogger(hl.context).Infof("Range: %s", req.Header.Get("Range"))
	}

	resp, err := hl.client.Do(req)
	if err != nil {
		return nil, err
	}

	switch {
	case resp.StatusCode == 200:
		hl.rc = resp.Body
	default:
		defer resp.Body.Close()
		return nil, fmt.Errorf("unexpected status resolving reader: %v", resp.Status)
	}

	if hl.brd == nil {
		hl.brd = bufio.NewReader(hl.rc)
	} else {
		hl.brd.Reset(hl.rc)
	}

	return hl.brd, nil
}

func (hl *httpLayer) Length() int64 {
	return hl.size
}

func (hl *httpLayer) Handler(r *http.Request) (http.Handler, error) {
	panic("Not implemented")
}
