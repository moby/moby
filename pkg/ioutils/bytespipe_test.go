package ioutils // import "github.com/docker/docker/pkg/ioutils"

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"testing"
	"time"
)

func TestBytesPipeRead(t *testing.T) {
	buf := NewBytesPipe()
	buf.Write([]byte("12"))
	buf.Write([]byte("34"))
	buf.Write([]byte("56"))
	buf.Write([]byte("78"))
	buf.Write([]byte("90"))
	rd := make([]byte, 4)
	n, err := buf.Read(rd)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("Wrong number of bytes read: %d, should be %d", n, 4)
	}
	if string(rd) != "1234" {
		t.Fatalf("Read %s, but must be %s", rd, "1234")
	}
	n, err = buf.Read(rd)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("Wrong number of bytes read: %d, should be %d", n, 4)
	}
	if string(rd) != "5678" {
		t.Fatalf("Read %s, but must be %s", rd, "5679")
	}
	n, err = buf.Read(rd)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("Wrong number of bytes read: %d, should be %d", n, 2)
	}
	if string(rd[:n]) != "90" {
		t.Fatalf("Read %s, but must be %s", rd, "90")
	}
}

func TestBytesPipeWrite(t *testing.T) {
	buf := NewBytesPipe()
	buf.Write([]byte("12"))
	buf.Write([]byte("34"))
	buf.Write([]byte("56"))
	buf.Write([]byte("78"))
	buf.Write([]byte("90"))
	if buf.buf[0].String() != "1234567890" {
		t.Fatalf("Buffer %q, must be %q", buf.buf[0].String(), "1234567890")
	}
}

// Regression test for #41941.
func TestBytesPipeDeadlock(t *testing.T) {
	bp := NewBytesPipe()
	bp.buf = []*fixedBuffer{getBuffer(blockThreshold)}

	rd := make(chan error)
	go func() {
		n, err := bp.Read(make([]byte, 1))
		t.Logf("Read n=%d, err=%v", n, err)
		if n != 1 {
			t.Errorf("short read: got %d, want 1", n)
		}
		rd <- err
	}()

	wr := make(chan error)
	go func() {
		const writeLen int = blockThreshold + 1
		time.Sleep(time.Millisecond)
		n, err := bp.Write(make([]byte, writeLen))
		t.Logf("Write n=%d, err=%v", n, err)
		if n != writeLen {
			t.Errorf("short write: got %d, want %d", n, writeLen)
		}
		wr <- err
	}()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Fatal("deadlock! Neither Read() nor Write() returned.")
	case rerr := <-rd:
		if rerr != nil {
			t.Fatal(rerr)
		}
		select {
		case <-timer.C:
			t.Fatal("deadlock! Write() did not return.")
		case werr := <-wr:
			if werr != nil {
				t.Fatal(werr)
			}
		}
	case werr := <-wr:
		if werr != nil {
			t.Fatal(werr)
		}
		select {
		case <-timer.C:
			t.Fatal("deadlock! Read() did not return.")
		case rerr := <-rd:
			if rerr != nil {
				t.Fatal(rerr)
			}
		}
	}
}

// Write and read in different speeds/chunk sizes and check valid data is read.
func TestBytesPipeWriteRandomChunks(t *testing.T) {
	cases := []struct{ iterations, writesPerLoop, readsPerLoop int }{
		{100, 10, 1},
		{1000, 10, 5},
		{1000, 100, 0},
		{1000, 5, 6},
		{10000, 50, 25},
	}

	testMessage := []byte("this is a random string for testing")
	// random slice sizes to read and write
	writeChunks := []int{25, 35, 15, 20}
	readChunks := []int{5, 45, 20, 25}

	for _, c := range cases {
		// first pass: write directly to hash
		hash := sha256.New()
		for i := 0; i < c.iterations*c.writesPerLoop; i++ {
			if _, err := hash.Write(testMessage[:writeChunks[i%len(writeChunks)]]); err != nil {
				t.Fatal(err)
			}
		}
		expected := hex.EncodeToString(hash.Sum(nil))

		// write/read through buffer
		buf := NewBytesPipe()
		hash.Reset()

		done := make(chan struct{})

		go func() {
			// random delay before read starts
			<-time.After(time.Duration(rand.Intn(10)) * time.Millisecond)
			for i := 0; ; i++ {
				p := make([]byte, readChunks[(c.iterations*c.readsPerLoop+i)%len(readChunks)])
				n, _ := buf.Read(p)
				if n == 0 {
					break
				}
				hash.Write(p[:n])
			}

			close(done)
		}()

		for i := 0; i < c.iterations; i++ {
			for w := 0; w < c.writesPerLoop; w++ {
				buf.Write(testMessage[:writeChunks[(i*c.writesPerLoop+w)%len(writeChunks)]])
			}
		}
		buf.Close()
		<-done

		actual := hex.EncodeToString(hash.Sum(nil))

		if expected != actual {
			t.Fatalf("BytesPipe returned invalid data. Expected checksum %v, got %v", expected, actual)
		}
	}
}

func BenchmarkBytesPipeWrite(b *testing.B) {
	testData := []byte("pretty short line, because why not?")
	for i := 0; i < b.N; i++ {
		readBuf := make([]byte, 1024)
		buf := NewBytesPipe()
		go func() {
			var err error
			for err == nil {
				_, err = buf.Read(readBuf)
			}
		}()
		for j := 0; j < 1000; j++ {
			buf.Write(testData)
		}
		buf.Close()
	}
}

func BenchmarkBytesPipeRead(b *testing.B) {
	rd := make([]byte, 512)
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		buf := NewBytesPipe()
		for j := 0; j < 500; j++ {
			buf.Write(make([]byte, 1024))
		}
		b.StartTimer()
		for j := 0; j < 1000; j++ {
			if n, _ := buf.Read(rd); n != 512 {
				b.Fatalf("Wrong number of bytes: %d", n)
			}
		}
	}
}
