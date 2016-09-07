package streamformatter

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestFormatStream(c *check.C) {
	sf := NewStreamFormatter()
	res := sf.FormatStream("stream")
	if string(res) != "stream"+"\r" {
		c.Fatalf("%q", res)
	}
}

func (s *DockerSuite) TestFormatJSONStatus(c *check.C) {
	sf := NewStreamFormatter()
	res := sf.FormatStatus("ID", "%s%d", "a", 1)
	if string(res) != "a1\r\n" {
		c.Fatalf("%q", res)
	}
}

func (s *DockerSuite) TestFormatSimpleError(c *check.C) {
	sf := NewStreamFormatter()
	res := sf.FormatError(errors.New("Error for formatter"))
	if string(res) != "Error: Error for formatter\r\n" {
		c.Fatalf("%q", res)
	}
}

func (s *DockerSuite) TestJSONFormatStream(c *check.C) {
	sf := NewJSONStreamFormatter()
	res := sf.FormatStream("stream")
	if string(res) != `{"stream":"stream"}`+"\r\n" {
		c.Fatalf("%q", res)
	}
}

func (s *DockerSuite) TestJSONFormatStatus(c *check.C) {
	sf := NewJSONStreamFormatter()
	res := sf.FormatStatus("ID", "%s%d", "a", 1)
	if string(res) != `{"status":"a1","id":"ID"}`+"\r\n" {
		c.Fatalf("%q", res)
	}
}

func (s *DockerSuite) TestJSONFormatSimpleError(c *check.C) {
	sf := NewJSONStreamFormatter()
	res := sf.FormatError(errors.New("Error for formatter"))
	if string(res) != `{"errorDetail":{"message":"Error for formatter"},"error":"Error for formatter"}`+"\r\n" {
		c.Fatalf("%q", res)
	}
}

func (s *DockerSuite) TestJSONFormatJSONError(c *check.C) {
	sf := NewJSONStreamFormatter()
	err := &jsonmessage.JSONError{Code: 50, Message: "Json error"}
	res := sf.FormatError(err)
	if string(res) != `{"errorDetail":{"code":50,"message":"Json error"},"error":"Json error"}`+"\r\n" {
		c.Fatalf("%q", res)
	}
}

func (s *DockerSuite) TestJSONFormatProgress(c *check.C) {
	sf := NewJSONStreamFormatter()
	progress := &jsonmessage.JSONProgress{
		Current: 15,
		Total:   30,
		Start:   1,
	}
	res := sf.FormatProgress("id", "action", progress, nil)
	msg := &jsonmessage.JSONMessage{}
	if err := json.Unmarshal(res, msg); err != nil {
		c.Fatal(err)
	}
	if msg.ID != "id" {
		c.Fatalf("ID must be 'id', got: %s", msg.ID)
	}
	if msg.Status != "action" {
		c.Fatalf("Status must be 'action', got: %s", msg.Status)
	}

	// The progress will always be in the format of:
	// [=========================>                         ]     15 B/30 B 404933h7m11s
	// The last entry '404933h7m11s' is the timeLeftBox.
	// However, the timeLeftBox field may change as progress.String() depends on time.Now().
	// Therefore, we have to strip the timeLeftBox from the strings to do the comparison.

	// Compare the progress strings before the timeLeftBox
	expectedProgress := "[=========================>                         ]     15 B/30 B"
	// if terminal column is <= 110, expectedProgressShort is expected.
	expectedProgressShort := "    15 B/30 B"
	if !(strings.HasPrefix(msg.ProgressMessage, expectedProgress) ||
		strings.HasPrefix(msg.ProgressMessage, expectedProgressShort)) {
		c.Fatalf("ProgressMessage without the timeLeftBox must be %s or %s, got: %s",
			expectedProgress, expectedProgressShort, msg.ProgressMessage)
	}

	if !reflect.DeepEqual(msg.Progress, progress) {
		c.Fatal("Original progress not equals progress from FormatProgress")
	}
}
