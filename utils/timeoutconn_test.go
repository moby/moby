package utils

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutConnRead(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello")
	}))
	defer ts.Close()
	conn, err := net.Dial("tcp", ts.URL[7:])
	if err != nil {
		t.Fatalf("failed to create connection to %q: %v", ts.URL, err)
	}
	tconn := NewTimeoutConn(conn, 1*time.Second)

	if _, err = bufio.NewReader(tconn).ReadString('\n'); err == nil {
		t.Fatalf("expected timeout error, got none")
	}
	if _, err := fmt.Fprintf(tconn, "GET / HTTP/1.0\r\n\r\n"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err = bufio.NewReader(tconn).ReadString('\n'); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
