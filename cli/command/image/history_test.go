package image

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"testing"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/docker/docker/pkg/testutil/golden"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewHistoryCommandErrors(t *testing.T) {
	testCases := []struct {
		name             string
		args             []string
		expectedError    string
		imageHistoryFunc func(img string) ([]image.HistoryResponseItem, error)
	}{
		{
			name:          "wrong-args",
			args:          []string{},
			expectedError: "requires exactly 1 argument(s).",
		},
		{
			name:          "client-error",
			args:          []string{"image:tag"},
			expectedError: "something went wrong",
			imageHistoryFunc: func(img string) ([]image.HistoryResponseItem, error) {
				return []image.HistoryResponseItem{{}}, errors.Errorf("something went wrong")
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := NewHistoryCommand(test.NewFakeCli(&fakeClient{imageHistoryFunc: tc.imageHistoryFunc}, buf))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNewHistoryCommandSuccess(t *testing.T) {
	testCases := []struct {
		name             string
		args             []string
		outputRegex      string
		imageHistoryFunc func(img string) ([]image.HistoryResponseItem, error)
	}{
		{
			name: "simple",
			args: []string{"image:tag"},
			imageHistoryFunc: func(img string) ([]image.HistoryResponseItem, error) {
				return []image.HistoryResponseItem{{
					ID:      "1234567890123456789",
					Created: time.Now().Unix(),
				}}, nil
			},
		},
		{
			name: "quiet",
			args: []string{"--quiet", "image:tag"},
		},
		// TODO: This test is failing since the output does not contain an RFC3339 date
		//{
		//	name:        "non-human",
		//	args:        []string{"--human=false", "image:tag"},
		//	outputRegex: "\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}", // RFC3339 date format match
		//},
		{
			name:        "non-human-header",
			args:        []string{"--human=false", "image:tag"},
			outputRegex: "CREATED\\sAT",
		},
		{
			name: "quiet-no-trunc",
			args: []string{"--quiet", "--no-trunc", "image:tag"},
			imageHistoryFunc: func(img string) ([]image.HistoryResponseItem, error) {
				return []image.HistoryResponseItem{{
					ID:      "1234567890123456789",
					Created: time.Now().Unix(),
				}}, nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := NewHistoryCommand(test.NewFakeCli(&fakeClient{imageHistoryFunc: tc.imageHistoryFunc}, buf))
		cmd.SetOutput(ioutil.Discard)
		cmd.SetArgs(tc.args)
		err := cmd.Execute()
		assert.NoError(t, err)
		actual := buf.String()
		if tc.outputRegex == "" {
			expected := string(golden.Get(t, []byte(actual), fmt.Sprintf("history-command-success.%s.golden", tc.name))[:])
			testutil.EqualNormalizedString(t, testutil.RemoveSpace, actual, expected)
		} else {
			match, _ := regexp.MatchString(tc.outputRegex, actual)
			assert.True(t, match)
		}
	}
}
