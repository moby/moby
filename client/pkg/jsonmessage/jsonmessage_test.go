package jsonmessage

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/term"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

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

	testcases := []struct {
		name     string
		progress JSONProgress
		expected expected
	}{
		{
			name: "no progress",
		},
		{
			name:     "progress 1",
			progress: JSONProgress{Progress: jsonstream.Progress{Current: 1}},
			expected: shortAndLong("      1B", "      1B"),
		},
		{
			name: "some progress with a start time",
			progress: JSONProgress{
				Progress: jsonstream.Progress{
					Current: 20,
					Total:   100,
					Start:   start.Unix(),
				},
				nowFunc: timeAfter(time.Second),
			},
			expected: shortAndLong(
				"     20B/100B 4s",
				"[==========>                                        ]      20B/100B 4s",
			),
		},
		{
			name:     "some progress without a start time",
			progress: JSONProgress{Progress: jsonstream.Progress{Current: 50, Total: 100}},
			expected: shortAndLong(
				"     50B/100B",
				"[=========================>                         ]      50B/100B",
			),
		},
		{
			name:     "current more than total is not negative gh#7136",
			progress: JSONProgress{Progress: jsonstream.Progress{Current: 50, Total: 40}},
			expected: shortAndLong(
				"     50B",
				"[==================================================>]      50B",
			),
		},
		{
			name:     "with units",
			progress: JSONProgress{Progress: jsonstream.Progress{Current: 50, Total: 100, Units: "units"}},
			expected: shortAndLong(
				"50/100 units",
				"[=========================>                         ] 50/100 units",
			),
		},
		{
			name:     "current more than total with units is not negative ",
			progress: JSONProgress{Progress: jsonstream.Progress{Current: 50, Total: 40, Units: "units"}},
			expected: shortAndLong(
				"50 units",
				"[==================================================>] 50 units",
			),
		},
		{
			name:     "hide counts",
			progress: JSONProgress{Progress: jsonstream.Progress{Current: 50, Total: 100, HideCounts: true}},
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
			ID:     "ID",
			Status: "status",
		}: {
			"ID: status\n",
			"ID: status\n",
		},
		// Stream over status
		{
			Status: "status",
			Stream: "stream",
		}: {
			"stream",
			"stream",
		},
		// With progress, stream empty
		{
			Status:   "status",
			Stream:   "",
			Progress: &JSONProgress{Progress: jsonstream.Progress{Current: 1}},
		}: {
			"",
			fmt.Sprintf("%c[2K\rstatus       1B\r", 27),
		},
	}

	// The tests :)
	for jsonMessage, expectedMessages := range messages {
		// Without terminal
		data := bytes.NewBuffer([]byte{})
		if err := jsonMessage.Display(data, false); err != nil {
			t.Fatal(err)
		}
		if data.String() != expectedMessages[0] {
			t.Fatalf("Expected %q,got %q", expectedMessages[0], data.String())
		}
		// With terminal
		data = bytes.NewBuffer([]byte{})
		if err := jsonMessage.Display(data, true); err != nil {
			t.Fatal(err)
		}
		if data.String() != expectedMessages[1] {
			t.Fatalf("\nExpected %q\n     got %q", expectedMessages[1], data.String())
		}
	}
}

// Test JSONMessage with an Error. It returns an error with the given text, not the meaning of the HTTP code.
func TestJSONMessageDisplayWithJSONError(t *testing.T) {
	data := bytes.NewBuffer([]byte{})
	jsonMessage := JSONMessage{Error: &jsonstream.Error{Code: 404, Message: "Can't find it"}}

	err := jsonMessage.Display(data, true)
	if err == nil || err.Error() != "Can't find it" {
		t.Fatalf("Expected a jsonstream.Error 404, got %q", err)
	}

	jsonMessage = JSONMessage{Error: &jsonstream.Error{Code: 401, Message: "Anything"}}
	err = jsonMessage.Display(data, true)
	assert.Check(t, is.Error(err, "Anything"))
}

func TestDisplayJSONMessagesStreamInvalidJSON(t *testing.T) {
	var inFd uintptr
	data := bytes.NewBuffer([]byte{})
	reader := strings.NewReader("This is not a 'valid' JSON []")
	inFd, _ = term.GetFdInfo(reader)

	exp := "invalid character "
	if err := DisplayJSONMessagesStream(reader, data, inFd, false, nil); err == nil || !strings.HasPrefix(err.Error(), exp) {
		t.Fatalf("Expected error (%s...), got %q", exp, err)
	}
}

func TestDisplayJSONMessagesStream(t *testing.T) {
	var inFd uintptr

	messages := map[string][]string{
		// empty string
		"": {
			"",
			"",
		},
		// Without progress & ID
		`{ "status": "status" }`: {
			"status\n",
			"status\n",
		},
		// Without progress, with ID
		`{ "id": "ID","status": "status" }`: {
			"ID: status\n",
			"ID: status\n",
		},
		// With progressDetail
		`{ "id": "ID", "status": "status", "progressDetail": { "Current": 1} }`: {
			"", // progressbar is disabled in non-terminal
			fmt.Sprintf("\n%c[%dA%c[2K\rID: status       1B\r%c[%dB", 27, 1, 27, 27, 1),
		},
	}

	// Use $TERM which is unlikely to exist, forcing DisplayJSONMessageStream to
	// (hopefully) use &noTermInfo.
	t.Setenv("TERM", "xyzzy-non-existent-terminfo")

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
}
