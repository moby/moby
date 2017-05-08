package httputils

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResumableRequestHeaderSimpleErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, world !")
	}))
	defer ts.Close()

	client := &http.Client{}

	var req *http.Request
	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedError := "client and request can't be nil\n"
	resreq := &resumableRequestReader{}
	_, err = resreq.Read([]byte{})
	if err == nil || err.Error() != expectedError {
		t.Fatalf("Expected an error with '%s', got %v.", expectedError, err)
	}

	resreq = &resumableRequestReader{
		client:    client,
		request:   req,
		totalSize: -1,
	}
	expectedError = "failed to auto detect content length"
	_, err = resreq.Read([]byte{})
	if err == nil || err.Error() != expectedError {
		t.Fatalf("Expected an error with '%s', got %v.", expectedError, err)
	}

}

// Not too much failures, bails out after some wait
func TestResumableRequestHeaderNotTooMuchFailures(t *testing.T) {
	client := &http.Client{}

	var badReq *http.Request
	badReq, err := http.NewRequest("GET", "I'm not an url", nil)
	if err != nil {
		t.Fatal(err)
	}

	resreq := &resumableRequestReader{
		client:      client,
		request:     badReq,
		failures:    0,
		maxFailures: 2,
	}
	read, err := resreq.Read([]byte{})
	if err != nil || read != 0 {
		t.Fatalf("Expected no error and no byte read, got err:%v, read:%v.", err, read)
	}
}

// Too much failures, returns the error
func TestResumableRequestHeaderTooMuchFailures(t *testing.T) {
	client := &http.Client{}

	var badReq *http.Request
	badReq, err := http.NewRequest("GET", "I'm not an url", nil)
	if err != nil {
		t.Fatal(err)
	}

	resreq := &resumableRequestReader{
		client:      client,
		request:     badReq,
		failures:    0,
		maxFailures: 1,
	}
	defer resreq.Close()

	expectedError := `Get I%27m%20not%20an%20url: unsupported protocol scheme ""`
	read, err := resreq.Read([]byte{})
	if err == nil || err.Error() != expectedError || read != 0 {
		t.Fatalf("Expected the error '%s', got err:%v, read:%v.", expectedError, err, read)
	}
}

type errorReaderCloser struct{}

func (errorReaderCloser) Close() error { return nil }

func (errorReaderCloser) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("An error occurred")
}

// If an unknown error is encountered, return 0, nil and log it
func TestResumableRequestReaderWithReadError(t *testing.T) {
	var req *http.Request
	req, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{}

	response := &http.Response{
		Status:        "500 Internal Server",
		StatusCode:    500,
		ContentLength: 0,
		Close:         true,
		Body:          errorReaderCloser{},
	}

	resreq := &resumableRequestReader{
		client:          client,
		request:         req,
		currentResponse: response,
		lastRange:       1,
		totalSize:       1,
	}
	defer resreq.Close()

	buf := make([]byte, 1)
	read, err := resreq.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	if read != 0 {
		t.Fatalf("Expected to have read nothing, but read %v", read)
	}
}

func TestResumableRequestReaderWithEOFWith416Response(t *testing.T) {
	var req *http.Request
	req, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{}

	response := &http.Response{
		Status:        "416 Requested Range Not Satisfiable",
		StatusCode:    416,
		ContentLength: 0,
		Close:         true,
		Body:          ioutil.NopCloser(strings.NewReader("")),
	}

	resreq := &resumableRequestReader{
		client:          client,
		request:         req,
		currentResponse: response,
		lastRange:       1,
		totalSize:       1,
	}
	defer resreq.Close()

	buf := make([]byte, 1)
	_, err = resreq.Read(buf)
	if err == nil || err != io.EOF {
		t.Fatalf("Expected an io.EOF error, got %v", err)
	}
}

func TestResumableRequestReaderWithServerDoesntSupportByteRanges(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "" {
			t.Fatalf("Expected a Range HTTP header, got nothing")
		}
	}))
	defer ts.Close()

	var req *http.Request
	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{}

	resreq := &resumableRequestReader{
		client:    client,
		request:   req,
		lastRange: 1,
	}
	defer resreq.Close()

	buf := make([]byte, 2)
	_, err = resreq.Read(buf)
	if err == nil || err.Error() != "the server doesn't support byte ranges" {
		t.Fatalf("Expected an error 'the server doesn't support byte ranges', got %v", err)
	}
}

func TestResumableRequestReaderWithZeroTotalSize(t *testing.T) {

	srvtxt := "some response text data"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, srvtxt)
	}))
	defer ts.Close()

	var req *http.Request
	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{}
	retries := uint32(5)

	resreq := ResumableRequestReader(client, req, retries, 0)
	defer resreq.Close()

	data, err := ioutil.ReadAll(resreq)
	if err != nil {
		t.Fatal(err)
	}

	resstr := strings.TrimSuffix(string(data), "\n")

	if resstr != srvtxt {
		t.Errorf("resstr != srvtxt")
	}
}

func TestResumableRequestReader(t *testing.T) {

	srvtxt := "some response text data"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, srvtxt)
	}))
	defer ts.Close()

	var req *http.Request
	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{}
	retries := uint32(5)
	imgSize := int64(len(srvtxt))

	resreq := ResumableRequestReader(client, req, retries, imgSize)
	defer resreq.Close()

	data, err := ioutil.ReadAll(resreq)
	if err != nil {
		t.Fatal(err)
	}

	resstr := strings.TrimSuffix(string(data), "\n")

	if resstr != srvtxt {
		t.Errorf("resstr != srvtxt")
	}
}

func TestResumableRequestReaderWithInitialResponse(t *testing.T) {

	srvtxt := "some response text data"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, srvtxt)
	}))
	defer ts.Close()

	var req *http.Request
	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{}
	retries := uint32(5)
	imgSize := int64(len(srvtxt))

	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	resreq := ResumableRequestReaderWithInitialResponse(client, req, retries, imgSize, res)
	defer resreq.Close()

	data, err := ioutil.ReadAll(resreq)
	if err != nil {
		t.Fatal(err)
	}

	resstr := strings.TrimSuffix(string(data), "\n")

	if resstr != srvtxt {
		t.Errorf("resstr != srvtxt")
	}
}
