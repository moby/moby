package resumable // import "github.com/moby/moby/registry/resumable"

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type requestReader struct {
	client          *http.Client
	request         *http.Request
	lastRange       int64
	totalSize       int64
	currentResponse *http.Response
	failures        uint32
	maxFailures     uint32
	waitDuration    time.Duration
}

// NewRequestReader makes it possible to resume reading a request's body transparently
// maxfail is the number of times we retry to make requests again (not resumes)
// totalsize is the total length of the body; auto detect if not provided
func NewRequestReader(c *http.Client, r *http.Request, maxfail uint32, totalsize int64) io.ReadCloser {
	return &requestReader{client: c, request: r, maxFailures: maxfail, totalSize: totalsize, waitDuration: 5 * time.Second}
}

// NewRequestReaderWithInitialResponse makes it possible to resume
// reading the body of an already initiated request.
func NewRequestReaderWithInitialResponse(c *http.Client, r *http.Request, maxfail uint32, totalsize int64, initialResponse *http.Response) io.ReadCloser {
	return &requestReader{client: c, request: r, maxFailures: maxfail, totalSize: totalsize, currentResponse: initialResponse, waitDuration: 5 * time.Second}
}

func (r *requestReader) Read(p []byte) (n int, err error) {
	if r.client == nil || r.request == nil {
		return 0, fmt.Errorf("client and request can't be nil")
	}
	isFreshRequest := false
	if r.lastRange != 0 && r.currentResponse == nil {
		readRange := fmt.Sprintf("bytes=%d-%d", r.lastRange, r.totalSize)
		r.request.Header.Set("Range", readRange)
		time.Sleep(r.waitDuration)
	}
	if r.currentResponse == nil {
		r.currentResponse, err = r.client.Do(r.request)
		isFreshRequest = true
	}
	if err != nil && r.failures+1 != r.maxFailures {
		r.cleanUpResponse()
		r.failures++
		time.Sleep(time.Duration(r.failures) * r.waitDuration)
		return 0, nil
	} else if err != nil {
		r.cleanUpResponse()
		return 0, err
	}
	if r.currentResponse.StatusCode == http.StatusRequestedRangeNotSatisfiable && r.lastRange == r.totalSize && r.currentResponse.ContentLength == 0 {
		r.cleanUpResponse()
		return 0, io.EOF
	} else if r.currentResponse.StatusCode != http.StatusPartialContent && r.lastRange != 0 && isFreshRequest {
		r.cleanUpResponse()
		return 0, fmt.Errorf("the server doesn't support byte ranges")
	}
	if r.totalSize == 0 {
		r.totalSize = r.currentResponse.ContentLength
	} else if r.totalSize <= 0 {
		r.cleanUpResponse()
		return 0, fmt.Errorf("failed to auto detect content length")
	}
	n, err = r.currentResponse.Body.Read(p)
	r.lastRange += int64(n)
	if err != nil {
		r.cleanUpResponse()
	}
	if err != nil && err != io.EOF {
		logrus.Infof("encountered error during pull and clearing it before resume: %s", err)
		err = nil
	}
	return n, err
}

func (r *requestReader) Close() error {
	r.cleanUpResponse()
	r.client = nil
	r.request = nil
	return nil
}

func (r *requestReader) cleanUpResponse() {
	if r.currentResponse != nil {
		r.currentResponse.Body.Close()
		r.currentResponse = nil
	}
}
