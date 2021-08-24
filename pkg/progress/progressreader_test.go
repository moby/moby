package progress // import "github.com/docker/docker/pkg/progress"

import (
	"bytes"
	"io"
	"testing"
)

func TestOutputOnPrematureClose(t *testing.T) {
	content := []byte("TESTING")
	reader := io.NopCloser(bytes.NewReader(content))
	progressChan := make(chan Progress, 10)

	pr := NewProgressReader(reader, ChanOutput(progressChan), int64(len(content)), "Test", "Read")

	part := make([]byte, 4)
	_, err := io.ReadFull(pr, part)
	if err != nil {
		pr.Close()
		t.Fatal(err)
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
		t.Fatalf("Expected some output when closing prematurely")
	}
}

func TestCompleteSilently(t *testing.T) {
	content := []byte("TESTING")
	reader := io.NopCloser(bytes.NewReader(content))
	progressChan := make(chan Progress, 10)

	pr := NewProgressReader(reader, ChanOutput(progressChan), int64(len(content)), "Test", "Read")

	out, err := io.ReadAll(pr)
	if err != nil {
		pr.Close()
		t.Fatal(err)
	}
	if string(out) != "TESTING" {
		pr.Close()
		t.Fatalf("Unexpected output %q from reader", string(out))
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
		t.Fatalf("Should have closed silently when read is complete")
	default:
	}
}
