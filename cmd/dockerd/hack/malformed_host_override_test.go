// +build !windows

package hack

import (
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
)

func TestHeaderOverrideHack(t *testing.T) {
	client, srv := net.Pipe()
	tests := [][2][]byte{
		{
			[]byte("GET /foo\nHost: /var/run/docker.sock\nUser-Agent: Docker\r\n\r\n"),
			[]byte("GET /foo\nHost: \r\nConnection: close\r\nUser-Agent: Docker\r\n\r\n"),
		},
		{
			[]byte("GET /foo\nHost: /var/run/docker.sock\nUser-Agent: Docker\nFoo: Bar\r\n"),
			[]byte("GET /foo\nHost: \r\nConnection: close\r\nUser-Agent: Docker\nFoo: Bar\r\n"),
		},
		{
			[]byte("GET /foo\nHost: /var/run/docker.sock\nUser-Agent: Docker\r\n\r\ntest something!"),
			[]byte("GET /foo\nHost: \r\nConnection: close\r\nUser-Agent: Docker\r\n\r\ntest something!"),
		},
		{
			[]byte("GET /foo\nHost: /var/run/docker.sock\nUser-Agent: Docker\r\n\r\ntest something! " + strings.Repeat("test", 15000)),
			[]byte("GET /foo\nHost: \r\nConnection: close\r\nUser-Agent: Docker\r\n\r\ntest something! " + strings.Repeat("test", 15000)),
		},
		{
			[]byte("GET /foo\nFoo: Bar\nHost: /var/run/docker.sock\nUser-Agent: Docker\r\n\r\n"),
			[]byte("GET /foo\nFoo: Bar\nHost: /var/run/docker.sock\nUser-Agent: Docker\r\n\r\n"),
		},
	}
	l := MalformedHostHeaderOverrideConn{client, true}
	read := make([]byte, 4096)

	for _, pair := range tests {
		go func(x []byte) {
			srv.Write(x)
		}(pair[0])
		n, err := l.Read(read)
		if err != nil && err != io.EOF {
			t.Fatalf("read: %d - %d, err: %v\n%s", n, len(pair[0]), err, string(read[:n]))
		}
		if !bytes.Equal(read[:n], pair[1][:n]) {
			t.Fatalf("\n%s\n%s\n", read[:n], pair[1][:n])
		}
		l.first = true
		// clean out the slice
		read = read[:0]
	}
	srv.Close()
	l.Close()
}

func BenchmarkWithHack(b *testing.B) {
	client, srv := net.Pipe()
	done := make(chan struct{})
	req := []byte("GET /foo\nHost: /var/run/docker.sock\nUser-Agent: Docker\n")
	read := make([]byte, 4096)
	b.SetBytes(int64(len(req) * 30))

	l := MalformedHostHeaderOverrideConn{client, true}
	go func() {
		for {
			if _, err := srv.Write(req); err != nil {
				srv.Close()
				break
			}
			l.first = true // make sure each subsequent run uses the hack parsing
		}
		close(done)
	}()

	for i := 0; i < b.N; i++ {
		for i := 0; i < 30; i++ {
			if n, err := l.Read(read); err != nil && err != io.EOF {
				b.Fatalf("read: %d - %d, err: %v\n%s", n, len(req), err, string(read[:n]))
			}
		}
	}
	l.Close()
	<-done
}

func BenchmarkNoHack(b *testing.B) {
	client, srv := net.Pipe()
	done := make(chan struct{})
	req := []byte("GET /foo\nHost: /var/run/docker.sock\nUser-Agent: Docker\n")
	read := make([]byte, 4096)
	b.SetBytes(int64(len(req) * 30))

	go func() {
		for {
			if _, err := srv.Write(req); err != nil {
				srv.Close()
				break
			}
		}
		close(done)
	}()

	for i := 0; i < b.N; i++ {
		for i := 0; i < 30; i++ {
			if _, err := client.Read(read); err != nil && err != io.EOF {
				b.Fatal(err)
			}
		}
	}
	client.Close()
	<-done
}
