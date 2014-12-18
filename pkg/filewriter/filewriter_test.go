package filewriter

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestFileWriterWrites(t *testing.T) {
	pth, err := ioutil.TempFile(os.TempDir(), "truncate test")
	if err != nil {
		t.Fatal(err)
	}
	pth.Close()
	defer os.RemoveAll(pth.Name())

	w := NewFileWriter(pth.Name())

	testStr := "asdf"

	if _, err := w.Write([]byte(testStr)); err != nil {
		t.Fatal(err)
	}

	b, err := ioutil.ReadFile(pth.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != testStr {
		t.Fatalf("Expected file to contain %s, got: %s", testStr, string(b))
	}
}

func TestFileWriterTruncate(t *testing.T) {
	pth, err := ioutil.TempFile(os.TempDir(), "truncate test")
	if err != nil {
		t.Fatal(err)
	}
	pth.Close()
	defer os.RemoveAll(pth.Name())

	w := NewFileWriter(pth.Name())

	testStr := []byte("asdf")

	if _, err := w.Write(testStr); err != nil {
		t.Fatal(err)
	}

	b, err := w.Truncate()
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(testStr, b) != 0 {
		t.Fatalf("Expected %s, Got: %s", testStr, string(b))
	}

	b, err = ioutil.ReadFile(pth.Name())
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(b, []byte{}) != 0 {
		t.Fatalf("Expected file to be empty after truncation")
	}
}

func TestFileWriterTruncateSafe(t *testing.T) {
	pth, err := ioutil.TempFile(os.TempDir(), "truncate test")
	if err != nil {
		t.Fatal(err)
	}
	pth.Close()
	defer os.RemoveAll(pth.Name())

	w := NewFileWriter(pth.Name())
	ch := make(chan bool, 1)
	defer func() {
		ch <- true
	}()

	var iterations int
	go func() {
		testStr := []byte("testing\n")
		for {
			select {
			case <-ch:
				break
			default:
				w.Write(testStr)
				iterations++
			}
		}
	}()

	for iterations < 5 {
		time.Sleep(1 * time.Millisecond)
	}

	b, err := w.Truncate()
	if err != nil {
		t.Fatal(err)
	}
	// Stop writing
	ch <- true

	b2, err := ioutil.ReadFile(pth.Name())
	if err != nil {
		t.Fatal(err)
	}

	bArr := bytes.Split(b, []byte("\n"))
	if len(bArr) < 5 {
		t.Fatalf("Expected at least 5 lines in the truncated output")
	}

	b2Arr := bytes.Split(b2, []byte("\n"))
	if (len(bArr) + len(b2Arr)) < iterations {
		t.Fatal("lost bytes during truncation, %d")
	}
}
