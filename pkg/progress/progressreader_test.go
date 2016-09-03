package progress

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestOutputOnPrematureClose(c *check.C) {
	content := []byte("TESTING")
	reader := ioutil.NopCloser(bytes.NewReader(content))
	progressChan := make(chan Progress, 10)

	pr := NewProgressReader(reader, ChanOutput(progressChan), int64(len(content)), "Test", "Read")

	part := make([]byte, 4, 4)
	_, err := io.ReadFull(pr, part)
	if err != nil {
		pr.Close()
		c.Fatal(err)
	}

drainLoop:
	for {
		select {
		case <-progressChan:
		default:
			break drainLoop
		}
	}

	pr.Close()

	select {
	case <-progressChan:
	default:
		c.Fatalf("Expected some output when closing prematurely")
	}
}

func (s *DockerSuite) TestCompleteSilently(c *check.C) {
	content := []byte("TESTING")
	reader := ioutil.NopCloser(bytes.NewReader(content))
	progressChan := make(chan Progress, 10)

	pr := NewProgressReader(reader, ChanOutput(progressChan), int64(len(content)), "Test", "Read")

	out, err := ioutil.ReadAll(pr)
	if err != nil {
		pr.Close()
		c.Fatal(err)
	}
	if string(out) != "TESTING" {
		pr.Close()
		c.Fatalf("Unexpected output %q from reader", string(out))
	}

drainLoop:
	for {
		select {
		case <-progressChan:
		default:
			break drainLoop
		}
	}

	pr.Close()

	select {
	case <-progressChan:
		c.Fatalf("Should have closed silently when read is complete")
	default:
	}
}
