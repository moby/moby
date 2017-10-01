package system

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/docker/docker/api/types/events"
	"golang.org/x/net/context"
)

func TestGetIntervalTime(t *testing.T) {
	testCases := []struct {
		since    string
		until    string
		expErr   string
		expSince time.Time
		expUntil time.Time
	}{
		{"bla", "10", `strconv.ParseInt: parsing "bla": invalid syntax`, time.Time{}, time.Time{}},
		{"10", "bla", `strconv.ParseInt: parsing "bla": invalid syntax`, time.Time{}, time.Time{}},
		{"10", "3", "`since` time (10) cannot be after `until` time (3)", time.Time{}, time.Time{}},
		{"1505028243", "1505038243", "", time.Unix(1505028243, 0), time.Unix(1505038243, 0)},
	}

	for _, tc := range testCases {
		s, u, err := getIntervalTime(tc.since, tc.until)
		if tc.expErr != "" && err != nil && err.Error() != tc.expErr {
			t.Errorf("expected error message to be '%s' but got '%s'", tc.expErr, err)
		}
		if tc.expErr != "" && err == nil {
			t.Error("expected an error but got none")
		}
		if tc.expErr == "" && err != nil {
			t.Error(err)
		}
		if tc.expErr == "" {
			if tc.expSince != s {
				t.Errorf("expected since to be '%s' but got '%s'", tc.expSince, s)
			}
			if tc.expUntil != u {
				t.Errorf("expected since to be '%s' but got '%s'", tc.expUntil, u)
			}
		}
	}
}

func TestEventWaiter(t *testing.T) {
	testCases := []struct {
		until       time.Time
		testTimeout <-chan time.Time
		expOnlyPast bool
	}{
		{time.Now().Add(1 * time.Millisecond), time.Tick(3 * time.Second), false},
		{time.Now().Add(-1 * time.Minute), time.Tick(3 * time.Second), true},
		{time.Now().Add(-1 * time.Minute), time.Tick(1 * time.Millisecond), true},
	}

	for _, tc := range testCases {
		onlyPastEvents, timeout := eventWaiter(tc.until)
		if tc.expOnlyPast != onlyPastEvents {
			t.Errorf("expected onlyPastEvents to be '%v' but got '%v'", tc.expOnlyPast, onlyPastEvents)
		}
		if !onlyPastEvents {
			select {
			case <-tc.testTimeout:
				if !tc.until.IsZero() {
					t.Error("expected timeout to be triggered but it didn't")
				}
			case <-timeout:
			}
		}
	}
}

func TestStreamEvents(t *testing.T) {
	testCases := []struct {
		until    time.Time
		buffered []events.Message
		events   func(context.CancelFunc) chan interface{}
		expected string
	}{
		// make sure only buffered events are printed
		{time.Now().Add(-3 * time.Minute), []events.Message{
			{Action: "bla"},
		},
			func(cancel context.CancelFunc) chan interface{} {
				l := make(chan interface{})
				go func() {
					for i := 0; i < 10; i++ {
						l <- events.Message{}
					}
				}()
				return l
			},
			`{"Type":"","Action":"bla","Actor":{"ID":"","Attributes":null}}
`,
		},
		// make sure when context is canceled streamEvents exists
		{time.Now().Add(5 * time.Minute),
			[]events.Message{},
			func(cancel context.CancelFunc) chan interface{} {
				l := make(chan interface{})
				go func() {
					cancel()
				}()
				return l
			}, "",
		},
		// make sure new events are printed
		{time.Now().Add(5 * time.Minute),
			[]events.Message{},
			func(cancel context.CancelFunc) chan interface{} {
				l := make(chan interface{})
				go func() {
					l <- events.Message{Action: "a"}
					l <- events.Message{Action: "b"}
					cancel()
				}()
				return l
			}, `{"Type":"","Action":"a","Actor":{"ID":"","Attributes":null}}
{"Type":"","Action":"b","Actor":{"ID":"","Attributes":null}}
`},
		// make sure new events are printed until timeout
		{time.Now().Add(3 * time.Millisecond),
			[]events.Message{},
			func(cancel context.CancelFunc) chan interface{} {
				l := make(chan interface{})
				go func() {
					l <- events.Message{Action: "a"}
					time.Sleep(time.Second)
					l <- events.Message{Action: "b"}
				}()
				return l
			}, `{"Type":"","Action":"a","Actor":{"ID":"","Attributes":null}}
`},
		// make sure if the event is not events.Message, streamEvents won't fail
		// and nothing is printed on the screen.
		{time.Now().Add(5 * time.Minute),
			[]events.Message{},
			func(cancel context.CancelFunc) chan interface{} {
				l := make(chan interface{})
				go func() {
					l <- map[string]string{"Action": "a"}
					l <- map[string]string{"Action": "b"}
					l <- map[string]string{"Action": "c"}
					cancel()
				}()
				return l
			}, ""},
	}

	for _, tc := range testCases {
		cancelCtx, cancel := context.WithCancel(context.Background())
		buffer := &bytes.Buffer{}
		if err := streamEvents(cancelCtx, buffer, tc.buffered, tc.events(cancel), tc.until); err != nil {
			t.Fatal(err)
		}
		if tc.expected != buffer.String() {
			t.Errorf("expected to see:\n'%s'\nbut got:\n'%s'\n", tc.expected, buffer)
		}
	}
}

type FailWriter struct {
}

func (f FailWriter) Write(p []byte) (int, error) {
	return 0, errors.New("bad writing")
}

// Test that if there is problem in encoding message streamEvents returns
// an error and not ignoring it
func TestStreamEventsError(t *testing.T) {
	err := streamEvents(context.Background(), FailWriter{}, []events.Message{{}}, make(chan interface{}), time.Now().Add(-3*time.Minute))
	if err == nil {
		t.Error("expected streamEvents to return error but didn't receive it")
	}
	if err.Error() != "bad writing" {
		t.Errorf("expected error to be 'bad writing' but got '%s'", err)
	}
}
