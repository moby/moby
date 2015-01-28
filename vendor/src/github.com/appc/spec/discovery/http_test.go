package discovery

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
)

func fakeHttpOrHttpsGet(filename string, httpSuccess bool, httpsSuccess bool, httpErrorCode int) func(uri string) (*http.Response, error) {
	return func(uri string) (*http.Response, error) {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}

		var resp *http.Response

		switch {
		case strings.HasPrefix(uri, "https://") && httpsSuccess:
			fallthrough
		case strings.HasPrefix(uri, "http://") && httpSuccess:
			resp = &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header: http.Header{
					"Content-Type": []string{"text/html"},
				},
				Body: f,
			}
		case httpErrorCode > 0:
			resp = &http.Response{
				Status:     "Error",
				StatusCode: httpErrorCode,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header: http.Header{
					"Content-Type": []string{"text/html"},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}
		default:
			err = errors.New("fakeHttpOrHttpsGet failed as requested")
			return nil, err
		}

		return resp, nil
	}
}

func TestHttpsOrHTTP(t *testing.T) {
	tests := []struct {
		name          string
		insecure      bool
		get           httpgetter
		expectUrlStr  string
		expectSuccess bool
	}{
		{
			"good-server",
			false,
			fakeHttpOrHttpsGet("myapp.html", true, true, 0),
			"https://good-server?ac-discovery=1",
			true,
		},
		{
			"file-not-found",
			false,
			fakeHttpOrHttpsGet("myapp.html", false, false, 404),
			"",
			false,
		},
		{
			"completely-broken-server",
			false,
			fakeHttpOrHttpsGet("myapp.html", false, false, 0),
			"",
			false,
		},
		{
			"file-only-on-http",
			false, // do not accept fallback on http
			fakeHttpOrHttpsGet("myapp.html", true, false, 404),
			"",
			false,
		},
		{
			"file-only-on-http",
			true, // accept fallback on http
			fakeHttpOrHttpsGet("myapp.html", true, false, 404),
			"http://file-only-on-http?ac-discovery=1",
			true,
		},
		{
			"https-server-is-down",
			true, // accept fallback on http
			fakeHttpOrHttpsGet("myapp.html", true, false, 0),
			"http://https-server-is-down?ac-discovery=1",
			true,
		},
	}

	for i, tt := range tests {
		httpGet = tt.get
		urlStr, body, err := httpsOrHTTP(tt.name, tt.insecure)
		if tt.expectSuccess {
			if err != nil {
				t.Fatalf("#%d httpsOrHTTP failed: %v", i, err)
			}
			if urlStr == "" {
				t.Fatalf("#%d httpsOrHTTP didn't return a urlStr", i)
			}
			if urlStr != tt.expectUrlStr {
				t.Fatalf("#%d httpsOrHTTP urlStr mismatch: want %s got %s",
					i, tt.expectUrlStr, urlStr)
			}
			if body == nil {
				t.Fatalf("#%d httpsOrHTTP didn't return a body", i)
			}
		} else {
			if err == nil {
				t.Fatalf("#%d httpsOrHTTP should have failed", i)
			}
			if urlStr != "" {
				t.Fatalf("#%d httpsOrHTTP should not have returned a urlStr", i)
			}
			if body != nil {
				t.Fatalf("#%d httpsOrHTTP should not have returned a body", i)
			}
		}
	}
}
