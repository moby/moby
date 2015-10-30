package plaintextlog

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/docker/docker/daemon/logger"
)

func TestPlainTextLogger(t *testing.T) {
	cid := "a7317399f3f857173c6179d44823594f8294678dea9999662e5c625b5a1c7657"
	tmp, err := ioutil.TempDir("", "docker-logger-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	filename := filepath.Join(tmp, "container.log")
	config := map[string]string{"log-path": filename}
	l, err := New(logger.Context{
		ContainerID: cid,
		LogPath:     filename,
		Config:      config,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	if err := l.Log(&logger.Message{ContainerID: cid, Line: []byte("line1"), Source: "src1"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Log(&logger.Message{ContainerID: cid, Line: []byte("line2"), Source: "src2"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Log(&logger.Message{ContainerID: cid, Line: []byte("line3"), Source: "src3"}); err != nil {
		t.Fatal(err)
	}
	res, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	expected := `0001-01-01 00:00:00 +0000 UTC : line1
0001-01-01 00:00:00 +0000 UTC : line2
0001-01-01 00:00:00 +0000 UTC : line3
`

	if string(res) != expected {
		t.Fatalf("Wrong log content: %q, expected %q", res, expected)
	}
}

func TestPlainTextLoggerWithOpts(t *testing.T) {
	cid := "a7317399f3f857173c6179d44823594f8294678dea9999662e5c625b5a1c7657"
	tmp, err := ioutil.TempDir("", "docker-logger-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	filename := filepath.Join(tmp, "container.log")
	config := map[string]string{"log-path": filename, "max-file": "2", "max-size": "1k"}
	l, err := New(logger.Context{
		ContainerID: cid,
		LogPath:     filename,
		Config:      config,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for i := 0; i < 30; i++ {
		if err := l.Log(&logger.Message{ContainerID: cid, Line: []byte("line" + strconv.Itoa(i)), Source: "src1"}); err != nil {
			t.Fatal(err)
		}
	}
	res, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	penUlt, err := ioutil.ReadFile(filename + ".1")
	if err != nil {
		t.Fatal(err)
	}

	expectedPenultimate := `0001-01-01 00:00:00 +0000 UTC : line0
0001-01-01 00:00:00 +0000 UTC : line1
0001-01-01 00:00:00 +0000 UTC : line2
0001-01-01 00:00:00 +0000 UTC : line3
0001-01-01 00:00:00 +0000 UTC : line4
0001-01-01 00:00:00 +0000 UTC : line5
0001-01-01 00:00:00 +0000 UTC : line6
0001-01-01 00:00:00 +0000 UTC : line7
0001-01-01 00:00:00 +0000 UTC : line8
0001-01-01 00:00:00 +0000 UTC : line9
0001-01-01 00:00:00 +0000 UTC : line10
0001-01-01 00:00:00 +0000 UTC : line11
0001-01-01 00:00:00 +0000 UTC : line12
0001-01-01 00:00:00 +0000 UTC : line13
0001-01-01 00:00:00 +0000 UTC : line14
0001-01-01 00:00:00 +0000 UTC : line15
0001-01-01 00:00:00 +0000 UTC : line16
0001-01-01 00:00:00 +0000 UTC : line17
0001-01-01 00:00:00 +0000 UTC : line18
0001-01-01 00:00:00 +0000 UTC : line19
0001-01-01 00:00:00 +0000 UTC : line20
0001-01-01 00:00:00 +0000 UTC : line21
0001-01-01 00:00:00 +0000 UTC : line22
0001-01-01 00:00:00 +0000 UTC : line23
0001-01-01 00:00:00 +0000 UTC : line24
0001-01-01 00:00:00 +0000 UTC : line25
`

	expected := `0001-01-01 00:00:00 +0000 UTC : line26
0001-01-01 00:00:00 +0000 UTC : line27
0001-01-01 00:00:00 +0000 UTC : line28
0001-01-01 00:00:00 +0000 UTC : line29
`

	if string(res) != expected {
		t.Fatalf("Wrong log content: %q, expected %q", res, expected)
	}
	if string(penUlt) != expectedPenultimate {
		t.Fatalf("Wrong log content: %q, expected %q", penUlt, expectedPenultimate)
	}

}
