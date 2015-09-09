package ioutils

import "testing"

func TestBytesPipeRead(t *testing.T) {
	buf := NewBytesPipe(nil)
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
	buf := NewBytesPipe(nil)
	buf.Write([]byte("12"))
	buf.Write([]byte("34"))
	buf.Write([]byte("56"))
	buf.Write([]byte("78"))
	buf.Write([]byte("90"))
	if string(buf.buf) != "1234567890" {
		t.Fatalf("Buffer %s, must be %s", buf.buf, "1234567890")
	}
}

func BenchmarkBytesPipeWrite(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf := NewBytesPipe(nil)
		for j := 0; j < 1000; j++ {
			buf.Write([]byte("pretty short line, because why not?"))
		}
	}
}

func BenchmarkBytesPipeRead(b *testing.B) {
	rd := make([]byte, 1024)
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		buf := NewBytesPipe(nil)
		for j := 0; j < 1000; j++ {
			buf.Write(make([]byte, 1024))
		}
		b.StartTimer()
		for j := 0; j < 1000; j++ {
			if n, _ := buf.Read(rd); n != 1024 {
				b.Fatalf("Wrong number of bytes: %d", n)
			}
		}
	}
}
