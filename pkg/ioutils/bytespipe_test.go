package ioutils

import (
	"crypto/sha1"
	"encoding/hex"
	"math/rand"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestBytesPipeRead(c *check.C) {
	buf := NewBytesPipe()
	buf.Write([]byte("12"))
	buf.Write([]byte("34"))
	buf.Write([]byte("56"))
	buf.Write([]byte("78"))
	buf.Write([]byte("90"))
	rd := make([]byte, 4)
	n, err := buf.Read(rd)
	if err != nil {
		c.Fatal(err)
	}
	if n != 4 {
		c.Fatalf("Wrong number of bytes read: %d, should be %d", n, 4)
	}
	if string(rd) != "1234" {
		c.Fatalf("Read %s, but must be %s", rd, "1234")
	}
	n, err = buf.Read(rd)
	if err != nil {
		c.Fatal(err)
	}
	if n != 4 {
		c.Fatalf("Wrong number of bytes read: %d, should be %d", n, 4)
	}
	if string(rd) != "5678" {
		c.Fatalf("Read %s, but must be %s", rd, "5679")
	}
	n, err = buf.Read(rd)
	if err != nil {
		c.Fatal(err)
	}
	if n != 2 {
		c.Fatalf("Wrong number of bytes read: %d, should be %d", n, 2)
	}
	if string(rd[:n]) != "90" {
		c.Fatalf("Read %s, but must be %s", rd, "90")
	}
}

func (s *DockerSuite) TestBytesPipeWrite(c *check.C) {
	buf := NewBytesPipe()
	buf.Write([]byte("12"))
	buf.Write([]byte("34"))
	buf.Write([]byte("56"))
	buf.Write([]byte("78"))
	buf.Write([]byte("90"))
	if buf.buf[0].String() != "1234567890" {
		c.Fatalf("Buffer %q, must be %q", buf.buf[0].String(), "1234567890")
	}
}

// Write and read in different speeds/chunk sizes and check valid data is read.
func (s *DockerSuite) TestBytesPipeWriteRandomChunks(c *check.C) {
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

	for _, ca := range cases {
		// first pass: write directly to hash
		hash := sha1.New()
		for i := 0; i < ca.iterations*ca.writesPerLoop; i++ {
			if _, err := hash.Write(testMessage[:writeChunks[i%len(writeChunks)]]); err != nil {
				c.Fatal(err)
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
				p := make([]byte, readChunks[(ca.iterations*ca.readsPerLoop+i)%len(readChunks)])
				n, _ := buf.Read(p)
				if n == 0 {
					break
				}
				hash.Write(p[:n])
			}

			close(done)
		}()

		for i := 0; i < ca.iterations; i++ {
			for w := 0; w < ca.writesPerLoop; w++ {
				buf.Write(testMessage[:writeChunks[(i*ca.writesPerLoop+w)%len(writeChunks)]])
			}
		}
		buf.Close()
		<-done

		actual := hex.EncodeToString(hash.Sum(nil))

		if expected != actual {
			c.Fatalf("BytesPipe returned invalid data. Expected checksum %v, got %v", expected, actual)
		}

	}
}

func (s *DockerSuite) BenchmarkBytesPipeWrite(c *check.C) {
	testData := []byte("pretty short line, because why not?")
	for i := 0; i < c.N; i++ {
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

func (s *DockerSuite) BenchmarkBytesPipeRead(c *check.C) {
	rd := make([]byte, 512)
	for i := 0; i < c.N; i++ {
		c.StopTimer()
		buf := NewBytesPipe()
		for j := 0; j < 500; j++ {
			buf.Write(make([]byte, 1024))
		}
		c.StartTimer()
		for j := 0; j < 1000; j++ {
			if n, _ := buf.Read(rd); n != 512 {
				c.Fatalf("Wrong number of bytes: %d", n)
			}
		}
	}
}
