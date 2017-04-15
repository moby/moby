package beam

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestFileReceive(t *testing.T) {
	f, err := ioutil.TempFile("", "beamtest-file-")
	if err != nil {
		t.Fatalf("tempfile: %v", err)
	}
	defer os.Remove(f.Name())
	input := "hello world!\n"
	f.Write([]byte(input))
	f.Seek(0, 0)
	sFile := File{f}
	msg, err := sFile.Receive()
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if msg.Stream != nil {
		t.Fatalf("receive: unexpected stream %#v", msg.Stream)
	}
	if result := string(msg.Data); result != input {
		t.Fatalf("unexpected data from file: '%s'", result)
	}
}
