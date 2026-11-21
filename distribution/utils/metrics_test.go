package utils // import "github.com/docker/docker/distribution/utils"

import (
	"bytes"
	"io"
	"testing"
)

type fakeCounter struct {
	count int
}

func (f *fakeCounter) Inc(vs ...float64) {
	if len(vs) == 0 {
		f.count++
		return
	}

	for _, v := range vs {
		f.count += int(v)
	}
}

func TestMetricsReader(t *testing.T) {
	// create a byte array of a known size, just filled with whatever
	testBytes := []byte{}
	for i := 0; i < 1234; i++ {
		testBytes = append(testBytes, 123)
	}
	testReader := bytes.NewBuffer(testBytes)
	f := &fakeCounter{}

	met := MetricsReader{
		ReadCloser: io.NopCloser(testReader),
		Counter:    f,
	}
	p := make([]byte, 11)
	read, err := met.Read(p)
	if err != nil {
		t.Errorf("error in read: %v", err)
	}

	count := f.count
	if read != count {
		t.Errorf("count read is not count recorded: %v != %v", read, count)
	}

	read, err = met.Read(p)
	if err != nil {
		t.Errorf("error in read: %v", err)
	}

	// get the count as the difference between the previous and current counts
	count = f.count - count
	if read != count {
		t.Errorf("count read is not count recorded: %v != %v", read, count)
	}
}
