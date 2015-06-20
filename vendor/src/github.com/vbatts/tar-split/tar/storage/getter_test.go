package storage

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestGetter(t *testing.T) {
	fgp := NewBufferFileGetPutter()
	files := map[string]map[string][]byte{
		"file1.txt": {"foo": []byte{60, 60, 48, 48, 0, 0, 0, 0}},
		"file2.txt": {"bar": []byte{45, 196, 22, 240, 0, 0, 0, 0}},
	}
	for n, b := range files {
		for body, sum := range b {
			_, csum, err := fgp.Put(n, bytes.NewBufferString(body))
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(csum, sum) {
				t.Errorf("checksum: expected 0x%x; got 0x%x", sum, csum)
			}
		}
	}
	for n, b := range files {
		for body := range b {
			r, err := fgp.Get(n)
			if err != nil {
				t.Error(err)
			}
			buf, err := ioutil.ReadAll(r)
			if err != nil {
				t.Error(err)
			}
			if body != string(buf) {
				t.Errorf("expected %q, got %q", body, string(buf))
			}
		}
	}
}
func TestPutter(t *testing.T) {
	fp := NewDiscardFilePutter()
	// map[filename]map[body]crc64sum
	files := map[string]map[string][]byte{
		"file1.txt": {"foo": []byte{60, 60, 48, 48, 0, 0, 0, 0}},
		"file2.txt": {"bar": []byte{45, 196, 22, 240, 0, 0, 0, 0}},
		"file3.txt": {"baz": []byte{32, 68, 22, 240, 0, 0, 0, 0}},
		"file4.txt": {"bif": []byte{48, 9, 150, 240, 0, 0, 0, 0}},
	}
	for n, b := range files {
		for body, sum := range b {
			_, csum, err := fp.Put(n, bytes.NewBufferString(body))
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(csum, sum) {
				t.Errorf("checksum on %q: expected %v; got %v", n, sum, csum)
			}
		}
	}
}
