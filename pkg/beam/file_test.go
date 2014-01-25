package beam

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestFileReceive(t *testing.T) {
	f, err := ioutil.TempFile("", "beamtest-file-")
	if err != nil {
		t.Fatalf("tempfile: %s", err)
	}
	defer os.Remove(f.Name())
	input := "hello world!\n"
	f.Write([]byte(input))
	f.Seek(0, 0)
	sFile := File{f}
	data, s, err := sFile.Receive()
	if err != nil {
		t.Fatalf("receive: %s", err)
	}
	if s != nil {
		t.Fatalf("receive: unexpected stream %#v", s)
	}
	if result := string(data); result != input {
		t.Fatalf("unexpected data from file: '%v'", result)
	}
}
