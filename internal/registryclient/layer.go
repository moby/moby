package client

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
)

type httpBlob struct {
	*repository

	desc distribution.Descriptor

	rc     io.ReadCloser // remote read closer
	brd    *bufio.Reader // internal buffered io
	offset int64
	err    error
}

func (hb *httpBlob) Read(p []byte) (n int, err error) {
	if hb.err != nil {
		return 0, hb.err
	}

	rd, err := hb.reader()
	if err != nil {
		return 0, err
	}

	n, err = rd.Read(p)
	hb.offset += int64(n)

	// Simulate io.EOF error if we reach filesize.
	if err == nil && hb.offset >= hb.desc.Length {
		err = io.EOF
	}

	return n, err
}

func (hb *httpBlob) Seek(offset int64, whence int) (int64, error) {
	if hb.err != nil {
		return 0, hb.err
	}

	var err error
	newOffset := hb.offset

	switch whence {
	case os.SEEK_CUR:
		newOffset += int64(offset)
	case os.SEEK_END:
		newOffset = hb.desc.Length + int64(offset)
	case os.SEEK_SET:
		newOffset = int64(offset)
	}

	if newOffset < 0 {
		err = fmt.Errorf("cannot seek to negative position")
	} else {
		if hb.offset != newOffset {
			hb.reset()
		}

		// No problems, set the offset.
		hb.offset = newOffset
	}

	return hb.offset, err
}

func (hb *httpBlob) Close() error {
	if hb.err != nil {
		return hb.err
	}

	// close and release reader chain
	if hb.rc != nil {
		hb.rc.Close()
	}

	hb.rc = nil
	hb.brd = nil

	hb.err = fmt.Errorf("httpBlob: closed")

	return nil
}

func (hb *httpBlob) reset() {
	if hb.err != nil {
		return
	}
	if hb.rc != nil {
		hb.rc.Close()
		hb.rc = nil
	}
}

func (hb *httpBlob) reader() (io.Reader, error) {
	if hb.err != nil {
		return nil, hb.err
	}

	if hb.rc != nil {
		return hb.brd, nil
	}

	// If the offset is great than or equal to size, return a empty, noop reader.
	if hb.offset >= hb.desc.Length {
		return ioutil.NopCloser(bytes.NewReader([]byte{})), nil
	}

	blobURL, err := hb.ub.BuildBlobURL(hb.name, hb.desc.Digest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", blobURL, nil)
	if err != nil {
		return nil, err
	}

	if hb.offset > 0 {
		// TODO(stevvooe): Get this working correctly.

		// If we are at different offset, issue a range request from there.
		req.Header.Add("Range", fmt.Sprintf("1-"))
		context.GetLogger(hb.context).Infof("Range: %s", req.Header.Get("Range"))
	}

	resp, err := hb.client.Do(req)
	if err != nil {
		return nil, err
	}

	switch {
	case resp.StatusCode == 200:
		hb.rc = resp.Body
	default:
		defer resp.Body.Close()
		return nil, fmt.Errorf("unexpected status resolving reader: %v", resp.Status)
	}

	if hb.brd == nil {
		hb.brd = bufio.NewReader(hb.rc)
	} else {
		hb.brd.Reset(hb.rc)
	}

	return hb.brd, nil
}
