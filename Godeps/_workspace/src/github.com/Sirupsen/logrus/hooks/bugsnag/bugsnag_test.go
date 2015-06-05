package logrus_bugsnag

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/bugsnag/bugsnag-go"
)

type notice struct {
	Events []struct {
		Exceptions []struct {
			Message string `json:"message"`
		} `json:"exceptions"`
	} `json:"events"`
}

func TestNoticeReceived(t *testing.T) {
	msg := make(chan string, 1)
	expectedMsg := "foo"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var notice notice
		data, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(data, &notice); err != nil {
			t.Error(err)
		}
		_ = r.Body.Close()

		msg <- notice.Events[0].Exceptions[0].Message
	}))
	defer ts.Close()

	hook := &bugsnagHook{}

	bugsnag.Configure(bugsnag.Configuration{
		Endpoint:     ts.URL,
		ReleaseStage: "production",
		APIKey:       "12345678901234567890123456789012",
		Synchronous:  true,
	})

	log := logrus.New()
	log.Hooks.Add(hook)

	log.WithFields(logrus.Fields{
		"error": errors.New(expectedMsg),
	}).Error("Bugsnag will not see this string")

	select {
	case received := <-msg:
		if received != expectedMsg {
			t.Errorf("Unexpected message received: %s", received)
		}
	case <-time.After(time.Second):
		t.Error("Timed out; no notice received by Bugsnag API")
	}
}
