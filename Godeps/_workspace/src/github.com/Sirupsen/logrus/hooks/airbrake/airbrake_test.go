package airbrake

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
)

type notice struct {
	Error NoticeError `xml:"error"`
}
type NoticeError struct {
	Class   string `xml:"class"`
	Message string `xml:"message"`
}

type customErr struct {
	msg string
}

func (e *customErr) Error() string {
	return e.msg
}

const (
	testAPIKey    = "abcxyz"
	testEnv       = "development"
	expectedClass = "*airbrake.customErr"
	expectedMsg   = "foo"
	unintendedMsg = "Airbrake will not see this string"
)

var (
	noticeError = make(chan NoticeError, 1)
)

// TestLogEntryMessageReceived checks if invoking Logrus' log.Error
// method causes an XML payload containing the log entry message is received
// by a HTTP server emulating an Airbrake-compatible endpoint.
func TestLogEntryMessageReceived(t *testing.T) {
	log := logrus.New()
	ts := startAirbrakeServer(t)
	defer ts.Close()

	hook := NewHook(ts.URL, testAPIKey, "production")
	log.Hooks.Add(hook)

	log.Error(expectedMsg)

	select {
	case received := <-noticeError:
		if received.Message != expectedMsg {
			t.Errorf("Unexpected message received: %s", received.Message)
		}
	case <-time.After(time.Second):
		t.Error("Timed out; no notice received by Airbrake API")
	}
}

// TestLogEntryMessageReceived confirms that, when passing an error type using
// logrus.Fields, a HTTP server emulating an Airbrake endpoint receives the
// error message returned by the Error() method on the error interface
// rather than the logrus.Entry.Message string.
func TestLogEntryWithErrorReceived(t *testing.T) {
	log := logrus.New()
	ts := startAirbrakeServer(t)
	defer ts.Close()

	hook := NewHook(ts.URL, testAPIKey, "production")
	log.Hooks.Add(hook)

	log.WithFields(logrus.Fields{
		"error": &customErr{expectedMsg},
	}).Error(unintendedMsg)

	select {
	case received := <-noticeError:
		if received.Message != expectedMsg {
			t.Errorf("Unexpected message received: %s", received.Message)
		}
		if received.Class != expectedClass {
			t.Errorf("Unexpected error class: %s", received.Class)
		}
	case <-time.After(time.Second):
		t.Error("Timed out; no notice received by Airbrake API")
	}
}

// TestLogEntryWithNonErrorTypeNotReceived confirms that, when passing a
// non-error type using logrus.Fields, a HTTP server emulating an Airbrake
// endpoint receives the logrus.Entry.Message string.
//
// Only error types are supported when setting the 'error' field using
// logrus.WithFields().
func TestLogEntryWithNonErrorTypeNotReceived(t *testing.T) {
	log := logrus.New()
	ts := startAirbrakeServer(t)
	defer ts.Close()

	hook := NewHook(ts.URL, testAPIKey, "production")
	log.Hooks.Add(hook)

	log.WithFields(logrus.Fields{
		"error": expectedMsg,
	}).Error(unintendedMsg)

	select {
	case received := <-noticeError:
		if received.Message != unintendedMsg {
			t.Errorf("Unexpected message received: %s", received.Message)
		}
	case <-time.After(time.Second):
		t.Error("Timed out; no notice received by Airbrake API")
	}
}

func startAirbrakeServer(t *testing.T) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var notice notice
		if err := xml.NewDecoder(r.Body).Decode(&notice); err != nil {
			t.Error(err)
		}
		r.Body.Close()

		noticeError <- notice.Error
	}))

	return ts
}
