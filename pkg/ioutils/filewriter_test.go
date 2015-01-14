package ioutils

import (
	"bytes"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"
)

func TestFileWriterWrites(t *testing.T) {
	pth := os.TempDir() + "/test"
	os.RemoveAll(pth)
	defer os.RemoveAll(pth)

	w, err := NewFileWriter(pth)
	if err != nil {
		t.Fatal(err)
	}

	testStr := "asdf"

	if _, err := w.Write([]byte(testStr)); err != nil {
		t.Fatal(err)
	}
	w.Close()

	b, err := ioutil.ReadFile(pth)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(b, []byte(testStr)) != 0 {
		t.Fatalf("Expected file to contain %v, got: %v", testStr, string(b))
	}
}

func TestFileWriterTruncate(t *testing.T) {
	pth := os.TempDir() + "/test"
	os.RemoveAll(pth)
	defer os.RemoveAll(pth)

	w, err := NewFileWriter(pth)
	if err != nil {
		t.Fatal(err)
	}

	testBytes := []byte("asdf")

	if _, err := w.Write(testBytes); err != nil {
		t.Fatal(err)
	}

	w.Close()

	buf := bytes.NewBuffer(make([]byte, 0, len(testBytes)))

	if err := w.Truncate(buf); err != nil {
		t.Fatal(err)
	}

	b, err := ioutil.ReadAll(buf)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(testBytes, b) != 0 {
		t.Fatalf("Expected %s, Got: %s", testBytes, b)
	}

	b, err = ioutil.ReadFile(pth)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(b, []byte{}) != 0 {
		t.Fatalf("Expected file to be empty after truncation")
	}
}

func TestFileWriterTruncateSafe(t *testing.T) {
	pth := os.TempDir() + "/test"
	os.RemoveAll(pth)
	defer os.RemoveAll(pth)

	w, err := NewFileWriter(pth)
	if err != nil {
		t.Fatal(err)
	}

	sigChan := make(chan bool)
	iterations := 1
	var lock sync.RWMutex

	incIterations := func() {
		lock.Lock()
		iterations++
		lock.Unlock()
	}

	testBytes := []byte("testing\n")
	go func() {
		for {
			select {
			case <-sigChan:
				return
			default:
				w.Write(testBytes)
				incIterations()
			}
		}
	}()

	for {
		lock.RLock()
		if iterations < 5 {
			lock.RUnlock()
			break
		}
		lock.RUnlock()
		time.Sleep(1 * time.Millisecond)
	}

	buf := bytes.NewBuffer(make([]byte, 0, len(testBytes)))
	if err := w.Truncate(buf); err != nil {
		t.Fatal(err)
	}
	// Stop writing
	sigChan <- true

	// Close the writer so whatever was buffered during/after the truncate is written and we can compare
	w.Close()

	b, err := ioutil.ReadAll(buf)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := ioutil.ReadFile(pth)
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, b2...)

	bArr := bytes.Split(b, []byte("\n"))
	if len(bArr) != iterations {
		t.Fatalf("lost bytes in truncation %v - %d", len(bArr), iterations)
	}
}
