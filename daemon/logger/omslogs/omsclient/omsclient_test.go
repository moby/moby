package omsclient

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	idStr       = "customerID string"
	keyStr      = "sharedKey string"
	postTimeout = 30 * time.Second
)

//date string, contentLength int, method string, contentType string, resource string
func TestNewOmsClient(t *testing.T) {
	client := NewOmsLogClient("", idStr, keyStr, postTimeout)

	omsclient, ok := client.(*omslogclient)
	if !ok {
		t.Fatal("Expected omsclient pointer type")
	}
	if omsclient == nil {
		t.Fatal("Did not Create a new Client")
	}

	if omsclient.url != "https://"+idStr+".ods.opinsights.azure.com/api/logs?api-version=2016-04-01" {
		t.Fatal("Default URL domain not correct")
	}
}

func TestNewOmsClientCustomDomain(t *testing.T) {
	client := NewOmsLogClient("ods.opinsights.azure.us", idStr, keyStr, postTimeout)

	omsclient, ok := client.(*omslogclient)
	if !ok {
		t.Fatal("Expected omsclient pointer type")
	}
	if omsclient == nil {
		t.Fatal("Did not Create a new Client")
	}

	if omsclient.url != "https://"+idStr+".ods.opinsights.azure.us/api/logs?api-version=2016-04-01" {
		t.Fatal("Custom URL domain not correct")
	}
}

// mock transport - taken from Docker client mock
type transportFunc func(*http.Request) (*http.Response, error)

func (tf transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return tf(req)
}

// mock http client - take from Docker client mock
func newMockClient(doer func(*http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{
		Transport: transportFunc(doer),
	}
}

// mock response body
type mockBody struct {
	body []byte
}

func newMockBody(body []byte) *mockBody {
	return &mockBody{body}
}

func (b *mockBody) Read(p []byte) (n int, err error) {
	return len(b.body), nil
}

func (b *mockBody) Close() error {
	return nil
}

// taken from Docker client mock.
func plainTextErrorMock(statusCode int, message string) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: statusCode,
			Body:       newMockBody([]byte(message)),
		}, nil
	}
}

func doerErrorMock(text string) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		return nil, errors.New(text)
	}
}

func TestPostDataSuccess(t *testing.T) {

	omsclient := &omslogclient{
		customerID:      idStr,
		sharedKey:       base64.StdEncoding.EncodeToString([]byte(keyStr)),
		url:             "Big Fancy URL string",
		httpPostTimeout: postTimeout,
		client:          newMockClient(plainTextErrorMock(http.StatusOK, "This is a response string")),
	}

	line := []byte("This is a log line")
	logType := "warning"

	if err := omsclient.PostData(&line, logType); err != nil {
		t.Fatal("Received an error from PostData")
	}
}

func TestPostDatasignatureFailure(t *testing.T) {
	omsclient := &omslogclient{
		customerID:      idStr,
		sharedKey:       keyStr, // invalid base64 key
		url:             "Big Fancy URL string",
		httpPostTimeout: postTimeout,
		client:          newMockClient(plainTextErrorMock(http.StatusProcessing, "This is a response string")),
	}

	line := []byte("This is a log line")
	logType := "warning"

	if err := omsclient.PostData(&line, logType); err == nil {
		t.Fatal("Did not receive an error from PostData")
	}
}

func TestPostDataDoerError(t *testing.T) {
	errText := "error text"
	omsclient := &omslogclient{
		customerID:      idStr,
		sharedKey:       base64.StdEncoding.EncodeToString([]byte(keyStr)),
		url:             "thisurl",
		httpPostTimeout: postTimeout,
		client:          newMockClient(doerErrorMock(errText)),
	}

	line := []byte("This is a log line")
	logType := "warning"

	if err := omsclient.PostData(&line, logType); err == nil {
		t.Fatal("Did not receive an error from PostData")
	} else {
		netError := err.Error()
		if !strings.Contains(netError, errText) {
			t.Fatal("Did not receive expected error (text)")
		}
		if !strings.Contains(netError, omsclient.url) {
			t.Fatal("Did not receive expected error (url)")
		}
	}
}
func TestPostDataStatus1XXError(t *testing.T) {
	omsclient := &omslogclient{
		customerID:      idStr,
		sharedKey:       base64.StdEncoding.EncodeToString([]byte(keyStr)),
		url:             "Big Fancy URL string",
		httpPostTimeout: postTimeout,
		client:          newMockClient(plainTextErrorMock(http.StatusProcessing, "This is a response string")),
	}

	line := []byte("This is a log line")
	logType := "warning"

	if err := omsclient.PostData(&line, logType); err == nil {
		t.Fatal("Did not receive an error from PostData")
	}
}

func TestPostDataStatus3XXError(t *testing.T) {
	omsclient := &omslogclient{
		customerID:      idStr,
		sharedKey:       base64.StdEncoding.EncodeToString([]byte(keyStr)),
		url:             "Big Fancy URL string",
		httpPostTimeout: postTimeout,
		client:          newMockClient(plainTextErrorMock(http.StatusForbidden, "This is a response string")),
	}

	line := []byte("This is a log line")
	logType := "warning"

	if err := omsclient.PostData(&line, logType); err == nil {
		t.Fatal("Did not receive an error from PostData")
	}
}
