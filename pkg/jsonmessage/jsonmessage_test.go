package jsonmessage // import "github.com/docker/docker/pkg/jsonmessage"

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/term"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestError(t *testing.T) {
	je := JSONError{404, "Not found"}
	assert.Assert(t, is.Error(&je, "Not found"))
}

func TestProgressString(t *testing.T) {
	type expected struct {
		short string
		long  string
	}

	shortAndLong := func(short, long string) expected {
		return expected{short: short, long: long}
	}

	start := time.Date(2017, 12, 3, 15, 10, 1, 0, time.UTC)
	timeAfter := func(delta time.Duration) func() time.Time {
		return func() time.Time {
			return start.Add(delta)
		}
	}

	var testcases = []struct {
		name     string
		progress JSONProgress
		expected expected
	}{
		{
			name: "no progress",
		},
		{
			name:     "progress 1",
			progress: JSONProgress{Current: 1},
			expected: shortAndLong("      1B", "      1B"),
		},
		{
			name: "some progress with a start time",
			progress: JSONProgress{
				Current: 20,
				Total:   100,
				Start:   start.Unix(),
				nowFunc: timeAfter(time.Second),
			},
			expected: shortAndLong(
				"     20B/100B 4s",
				"[==========>                                        ]      20B/100B 4s",
			),
		},
		{
			name:     "some progress without a start time",
			progress: JSONProgress{Current: 50, Total: 100},
			expected: shortAndLong(
				"     50B/100B",
				"[=========================>                         ]      50B/100B",
			),
		},
		{
			name:     "current more than total is not negative gh#7136",
			progress: JSONProgress{Current: 50, Total: 40},
			expected: shortAndLong(
				"     50B",
				"[==================================================>]      50B",
			),
		},
		{
			name:     "with units",
			progress: JSONProgress{Current: 50, Total: 100, Units: "units"},
			expected: shortAndLong(
				"50/100 units",
				"[=========================>                         ] 50/100 units",
			),
		},
		{
			name:     "current more than total with units is not negative ",
			progress: JSONProgress{Current: 50, Total: 40, Units: "units"},
			expected: shortAndLong(
				"50 units",
				"[==================================================>] 50 units",
			),
		},
		{
			name:     "hide counts",
			progress: JSONProgress{Current: 50, Total: 100, HideCounts: true},
			expected: shortAndLong(
				"",
				"[=========================>                         ] ",
			),
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			testcase.progress.winSize = 100
			assert.Equal(t, testcase.progress.String(), testcase.expected.short)

			testcase.progress.winSize = 200
			assert.Equal(t, testcase.progress.String(), testcase.expected.long)
		})
	}
}

func TestJSONMessageDisplay(t *testing.T) {
	now := time.Now()
	messages := map[JSONMessage][]string{
		// Empty
		{}: {"\n", "\n"},
		// Status
		{
			Status: "status",
		}: {
			"status\n",
			"status\n",
		},
		// General
		{
			Time:   now.Unix(),
			ID:     "ID",
			From:   "From",
			Status: "status",
		}: {
			fmt.Sprintf("%v ID: (from From) status\n", time.Unix(now.Unix(), 0).Format(RFC3339NanoFixed)),
			fmt.Sprintf("%v ID: (from From) status\n", time.Unix(now.Unix(), 0).Format(RFC3339NanoFixed)),
		},
		// General, with nano precision time
		{
			TimeNano: now.UnixNano(),
			ID:       "ID",
			From:     "From",
			Status:   "status",
		}: {
			fmt.Sprintf("%v ID: (from From) status\n", time.Unix(0, now.UnixNano()).Format(RFC3339NanoFixed)),
			fmt.Sprintf("%v ID: (from From) status\n", time.Unix(0, now.UnixNano()).Format(RFC3339NanoFixed)),
		},
		// General, with both times Nano is preferred
		{
			Time:     now.Unix(),
			TimeNano: now.UnixNano(),
			ID:       "ID",
			From:     "From",
			Status:   "status",
		}: {
			fmt.Sprintf("%v ID: (from From) status\n", time.Unix(0, now.UnixNano()).Format(RFC3339NanoFixed)),
			fmt.Sprintf("%v ID: (from From) status\n", time.Unix(0, now.UnixNano()).Format(RFC3339NanoFixed)),
		},
		// Stream over status
		{
			Status: "status",
			Stream: "stream",
		}: {
			"stream",
			"stream",
		},
		// With progress message
		{
			Status:          "status",
			ProgressMessage: "progressMessage",
		}: {
			"status progressMessage",
			"status progressMessage",
		},
		// With progress, stream empty
		{
			Status:   "status",
			Stream:   "",
			Progress: &JSONProgress{Current: 1},
		}: {
			"",
			fmt.Sprintf("%c[1K%c[K\rstatus       1B\r", 27, 27),
		},
	}

	// The tests :)
	for jsonMessage, expectedMessages := range messages {
		// Without terminal
		data := bytes.NewBuffer([]byte{})
		if err := jsonMessage.Display(data, nil); err != nil {
			t.Fatal(err)
		}
		if data.String() != expectedMessages[0] {
			t.Fatalf("Expected %q,got %q", expectedMessages[0], data.String())
		}
		// With terminal
		data = bytes.NewBuffer([]byte{})
		if err := jsonMessage.Display(data, &noTermInfo{}); err != nil {
			t.Fatal(err)
		}
		if data.String() != expectedMessages[1] {
			t.Fatalf("\nExpected %q\n     got %q", expectedMessages[1], data.String())
		}
	}
}

// Test JSONMessage with an Error. It will return an error with the text as error, not the meaning of the HTTP code.
func TestJSONMessageDisplayWithJSONError(t *testing.T) {
	data := bytes.NewBuffer([]byte{})
	jsonMessage := JSONMessage{Error: &JSONError{404, "Can't find it"}}

	err := jsonMessage.Display(data, &noTermInfo{})
	if err == nil || err.Error() != "Can't find it" {
		t.Fatalf("Expected a JSONError 404, got %q", err)
	}

	jsonMessage = JSONMessage{Error: &JSONError{401, "Anything"}}
	err = jsonMessage.Display(data, &noTermInfo{})
	assert.Check(t, is.Error(err, "authentication is required"))
}

func TestDisplayJSONMessagesStreamInvalidJSON(t *testing.T) {
	var (
		inFd uintptr
	)
	data := bytes.NewBuffer([]byte{})
	reader := strings.NewReader("This is not a 'valid' JSON []")
	inFd, _ = term.GetFdInfo(reader)

	if err := DisplayJSONMessagesStream(reader, data, inFd, false, nil); err == nil && err.Error()[:17] != "invalid character" {
		t.Fatalf("Should have thrown an error (invalid character in ..), got %q", err)
	}
}

func TestDisplayJSONMessagesStream(t *testing.T) {
	var (
		inFd uintptr
	)

	messages := map[string][]string{
		// empty string
		"": {
			"",
			""},
		// Without progress & ID
		"{ \"status\": \"status\" }": {
			"status\n",
			"status\n",
		},
		// Without progress, with ID
		"{ \"id\": \"ID\",\"status\": \"status\" }": {
			"ID: status\n",
			fmt.Sprintf("ID: status\n"),
		},
		// With progress
		"{ \"id\": \"ID\", \"status\": \"status\", \"progress\": \"ProgressMessage\" }": {
			"ID: status ProgressMessage",
			fmt.Sprintf("\n%c[%dAID: status ProgressMessage%c[%dB", 27, 1, 27, 1),
		},
		// With progressDetail
		"{ \"id\": \"ID\", \"status\": \"status\", \"progressDetail\": { \"Current\": 1} }": {
			"", // progressbar is disabled in non-terminal
			fmt.Sprintf("\n%c[%dA%c[1K%c[K\rID: status       1B\r%c[%dB", 27, 1, 27, 27, 27, 1),
		},
	}

	// Use $TERM which is unlikely to exist, forcing DisplayJSONMessageStream to
	// (hopefully) use &noTermInfo.
	origTerm := os.Getenv("TERM")
	os.Setenv("TERM", "xyzzy-non-existent-terminfo")

	for jsonMessage, expectedMessages := range messages {
		data := bytes.NewBuffer([]byte{})
		reader := strings.NewReader(jsonMessage)
		inFd, _ = term.GetFdInfo(reader)

		// Without terminal
		if err := DisplayJSONMessagesStream(reader, data, inFd, false, nil); err != nil {
			t.Fatal(err)
		}
		if data.String() != expectedMessages[0] {
			t.Fatalf("Expected an %q, got %q", expectedMessages[0], data.String())
		}

		// With terminal
		data = bytes.NewBuffer([]byte{})
		reader = strings.NewReader(jsonMessage)
		if err := DisplayJSONMessagesStream(reader, data, inFd, true, nil); err != nil {
			t.Fatal(err)
		}
		if data.String() != expectedMessages[1] {
			t.Fatalf("\nExpected %q\n     got %q", expectedMessages[1], data.String())
		}
	}
	os.Setenv("TERM", origTerm)

}
