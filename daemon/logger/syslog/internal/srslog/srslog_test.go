package srslog

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
)

func runPktSyslog(c net.PacketConn, done chan<- string) {
	var buf [4096]byte
	var rcvd string
	ct := 0
	for {
		var n int
		var err error

		c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, _, err = c.ReadFrom(buf[:])
		rcvd += string(buf[:n])
		if err != nil {
			if oe, ok := err.(*net.OpError); ok {
				if ct < 3 && oe.Temporary() {
					ct++
					continue
				}
			}
			break
		}
	}
	c.Close()
	done <- rcvd
}

type Crashy struct {
	sync.RWMutex
	is bool
}

func (c *Crashy) IsCrashy() bool {
	c.RLock()
	defer c.RUnlock()
	return c.is
}

func (c *Crashy) Set(is bool) {
	c.Lock()
	c.is = is
	c.Unlock()
}

var crashy = Crashy{is: false}

func testableNetwork(network string) bool {
	switch network {
	case "unix", "unixgram":
		switch runtime.GOOS {
		case "darwin":
			switch runtime.GOARCH {
			case "arm", "arm64":
				return false
			}
		case "android":
			return false
		}
	}
	return true
}

func runStreamSyslog(l net.Listener, done chan<- string, wg *sync.WaitGroup) {
	for {
		var c net.Conn
		var err error
		if c, err = l.Accept(); err != nil {
			return
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			c.SetReadDeadline(time.Now().Add(5 * time.Second))
			b := bufio.NewReader(c)
			for ct := 1; !crashy.IsCrashy() || ct&7 != 0; ct++ {
				s, err := b.ReadString('\n')
				if err != nil {
					break
				}
				done <- s
			}
			c.Close()
		}(c)
	}
}

func startServer(n, la string, done chan<- string) (addr string, sock io.Closer, wg *sync.WaitGroup) {
	if n == "udp" || n == "tcp" || n == "tcp+tls" {
		la = "127.0.0.1:0"
	} else {
		// unix and unixgram: choose an address if none given
		if la == "" {
			// use ioutil.TempFile to get a name that is unique
			f, err := ioutil.TempFile("", "syslogtest")
			if err != nil {
				log.Fatal("TempFile: ", err)
			}
			f.Close()
			la = f.Name()
		}
		os.Remove(la)
	}

	wg = new(sync.WaitGroup)
	if n == "udp" || n == "unixgram" {
		l, e := net.ListenPacket(n, la)
		if e != nil {
			log.Fatalf("startServer failed: %v", e)
		}
		addr = l.LocalAddr().String()
		sock = l
		wg.Add(1)
		go func() {
			defer wg.Done()
			runPktSyslog(l, done)
		}()
	} else if n == "tcp+tls" {
		cert, err := tls.LoadX509KeyPair("test/cert.pem", "test/privkey.pem")
		if err != nil {
			log.Fatalf("failed to load TLS keypair: %v", err)
		}
		config := tls.Config{Certificates: []tls.Certificate{cert}}
		l, e := tls.Listen("tcp", la, &config)
		if e != nil {
			log.Fatalf("startServer failed: %v", e)
		}
		addr = l.Addr().String()
		sock = l
		wg.Add(1)
		go func() {
			defer wg.Done()
			runStreamSyslog(l, done, wg)
		}()
	} else {
		l, e := net.Listen(n, la)
		if e != nil {
			log.Fatalf("startServer failed: %v", e)
		}
		addr = l.Addr().String()
		sock = l
		wg.Add(1)
		go func() {
			defer wg.Done()
			runStreamSyslog(l, done, wg)
		}()
	}
	return
}

func TestWithSimulated(t *testing.T) {
	msg := "Test 123"
	var transport []string
	for _, n := range []string{"unix", "unixgram", "udp", "tcp"} {
		if testableNetwork(n) {
			transport = append(transport, n)
		}
	}

	for _, tr := range transport {
		done := make(chan string)
		addr, sock, srvWG := startServer(tr, "", done)
		defer srvWG.Wait()
		defer sock.Close()
		if tr == "unix" || tr == "unixgram" {
			defer os.Remove(addr)
		}
		s, err := Dial(tr, addr, LOG_INFO|LOG_USER, "syslog_test")
		if err != nil {
			t.Fatalf("Dial() failed: %v", err)
		}
		err = s.Info(msg)
		if err != nil {
			t.Fatalf("log failed: %v", err)
		}
		check(t, msg, <-done)
		s.Close()
	}
}

func TestFlap(t *testing.T) {
	net := "unix"
	if !testableNetwork(net) {
		t.Skipf("skipping on %s/%s; 'unix' is not supported", runtime.GOOS, runtime.GOARCH)
	}

	done := make(chan string)
	addr, sock, srvWG := startServer(net, "", done)
	defer srvWG.Wait()
	defer os.Remove(addr)
	defer sock.Close()

	s, err := Dial(net, addr, LOG_INFO|LOG_USER, "syslog_test")
	if err != nil {
		t.Fatalf("Dial() failed: %v", err)
	}
	msg := "Moo 2"
	err = s.Info(msg)
	if err != nil {
		t.Fatalf("log failed: %v", err)
	}
	check(t, msg, <-done)

	// restart the server
	_, sock2, srvWG2 := startServer(net, addr, done)
	defer srvWG2.Wait()
	defer sock2.Close()

	// and try retransmitting
	msg = "Moo 3"
	err = s.Info(msg)
	if err != nil {
		t.Fatalf("log failed: %v", err)
	}
	check(t, msg, <-done)

	s.Close()
}

func TestNew(t *testing.T) {
	if LOG_LOCAL7 != 23<<3 {
		t.Fatalf("LOG_LOCAL7 has wrong value")
	}
	if testing.Short() {
		// Depends on syslog daemon running, and sometimes it's not.
		t.Skip("skipping syslog test during -short")
	}

	s, err := New(LOG_INFO|LOG_USER, "the_tag")
	if err != nil {
		t.Fatalf("New() failed: %s", err)
	}
	// Don't send any messages.
	s.Close()
}

func TestNewLogger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping syslog test during -short")
	}
	f, err := NewLogger(LOG_USER|LOG_INFO, 0)
	if f == nil {
		t.Error(err)
	}
}

func TestDial(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping syslog test during -short")
	}
	f, err := Dial("", "", (LOG_LOCAL7|LOG_DEBUG)+1, "syslog_test")
	if f != nil {
		t.Fatalf("Should have trapped bad priority")
	}
	f, err = Dial("", "", -1, "syslog_test")
	if f != nil {
		t.Fatalf("Should have trapped bad priority")
	}
	l, err := Dial("", "", LOG_USER|LOG_ERR, "syslog_test")
	if err != nil {
		t.Fatalf("Dial() failed: %s", err)
	}
	l.Close()
}

func TestDialFails(t *testing.T) {
	w, err := Dial("udp", "fakehost", LOG_ERR, "tag")
	if err == nil {
		t.Errorf("should fail to dial")
	}
	if w != nil {
		t.Errorf("should not get a writer")
	}
}

func TestDialTLSFails(t *testing.T) {
	w, err := DialWithTLSCertPath("tcp+tls", "127.0.0.1:0", LOG_ERR, "syslog_test", "test/nocertfound.pem")
	if w != nil {
		t.Fatalf("Should not have a writer")
	}
	if err == nil {
		t.Fatalf("Should have failed to load the cert")
	}
}

func check(t *testing.T, in, out string) {
	if hostname, err := os.Hostname(); err != nil {
		t.Error("Error retrieving hostname")
	} else {
		checkWithPriorityAndTag(t, LOG_USER+LOG_INFO, "syslog_test", hostname, in, out)
	}
}

func checkWithPriorityAndTag(t *testing.T, p Priority, tag, hostname, in, out string) {
	tmpl := fmt.Sprintf("<%d>%%s %%s %s[%%d]: %s\n", p, tag, in)
	var parsedHostname, timestamp string
	var pid int
	if n, err := fmt.Sscanf(out, tmpl, &timestamp, &parsedHostname, &pid); n != 3 || err != nil {
		t.Errorf("Got %q, does not match template %q (%d %s)", out, tmpl, n, err)
	} else if hostname != parsedHostname {
		t.Errorf("hostname expected %v, got %v", hostname, parsedHostname)
	}
}

func TestWrite(t *testing.T) {
	tests := []struct {
		pri Priority
		pre string
		msg string
		exp string
	}{
		{LOG_USER | LOG_ERR, "syslog_test", "", "%s %s syslog_test[%d]: \n"},
		{LOG_USER | LOG_ERR, "syslog_test", "write test", "%s %s syslog_test[%d]: write test\n"},
		// Write should not add \n if there already is one
		{LOG_USER | LOG_ERR, "syslog_test", "write test 2\n", "%s %s syslog_test[%d]: write test 2\n"},
	}

	if hostname, err := os.Hostname(); err != nil {
		t.Fatalf("Error retrieving hostname")
	} else {
		for _, test := range tests {
			done := make(chan string)
			addr, sock, srvWG := startServer("udp", "", done)
			defer srvWG.Wait()
			defer sock.Close()
			l, err := Dial("udp", addr, test.pri, test.pre)
			if err != nil {
				t.Fatalf("syslog.Dial() failed: %v", err)
			}
			defer l.Close()
			_, err = io.WriteString(l, test.msg)
			if err != nil {
				t.Fatalf("WriteString() failed: %v", err)
			}
			rcvd := <-done
			test.exp = fmt.Sprintf("<%d>", test.pri) + test.exp
			var parsedHostname, timestamp string
			var pid int
			if n, err := fmt.Sscanf(rcvd, test.exp, &timestamp, &parsedHostname, &pid); n != 3 || err != nil || hostname != parsedHostname {
				t.Errorf("s.Info() = '%q', didn't match '%q' (%d %s)", rcvd, test.exp, n, err)
			}
		}
	}
}

func TestTLSPathWrite(t *testing.T) {
	tests := []struct {
		pri Priority
		pre string
		msg string
		exp string
	}{
		{LOG_USER | LOG_ERR, "syslog_test", "", "%s %s syslog_test[%d]: \n"},
		{LOG_USER | LOG_ERR, "syslog_test", "write test", "%s %s syslog_test[%d]: write test\n"},
		// Write should not add \n if there already is one
		{LOG_USER | LOG_ERR, "syslog_test", "write test 2\n", "%s %s syslog_test[%d]: write test 2\n"},
	}

	if hostname, err := os.Hostname(); err != nil {
		t.Fatalf("Error retrieving hostname")
	} else {
		for _, test := range tests {
			done := make(chan string)
			addr, sock, srvWG := startServer("tcp+tls", "", done)
			defer srvWG.Wait()
			defer sock.Close()

			l, err := DialWithTLSCertPath("tcp+tls", addr, test.pri, test.pre, "test/cert.pem")
			if err != nil {
				t.Fatalf("syslog.Dial() failed: %v", err)
			}
			defer l.Close()
			_, err = io.WriteString(l, test.msg)
			if err != nil {
				t.Fatalf("WriteString() failed: %v", err)
			}
			rcvd := <-done
			test.exp = fmt.Sprintf("<%d>", test.pri) + test.exp
			var parsedHostname, timestamp string
			var pid int
			if n, err := fmt.Sscanf(rcvd, test.exp, &timestamp, &parsedHostname, &pid); n != 3 || err != nil || hostname != parsedHostname {
				t.Errorf("s.Info() = '%q', didn't match '%q' (%d %s)", rcvd, test.exp, n, err)
			}
		}
	}
}

func TestTLSCertWrite(t *testing.T) {
	tests := []struct {
		pri Priority
		pre string
		msg string
		exp string
	}{
		{LOG_USER | LOG_ERR, "syslog_test", "", "%s %s syslog_test[%d]: \n"},
		{LOG_USER | LOG_ERR, "syslog_test", "write test", "%s %s syslog_test[%d]: write test\n"},
		// Write should not add \n if there already is one
		{LOG_USER | LOG_ERR, "syslog_test", "write test 2\n", "%s %s syslog_test[%d]: write test 2\n"},
	}

	if hostname, err := os.Hostname(); err != nil {
		t.Fatalf("Error retrieving hostname")
	} else {
		for _, test := range tests {
			done := make(chan string)
			addr, sock, srvWG := startServer("tcp+tls", "", done)
			defer srvWG.Wait()
			defer sock.Close()

			cert, err := ioutil.ReadFile("test/cert.pem")
			if err != nil {
				t.Fatalf("cold not read cert: %v", err)
			}

			l, err := DialWithTLSCert("tcp+tls", addr, test.pri, test.pre, cert)
			if err != nil {
				t.Fatalf("syslog.Dial() failed: %v", err)
			}
			defer l.Close()
			_, err = io.WriteString(l, test.msg)
			if err != nil {
				t.Fatalf("WriteString() failed: %v", err)
			}
			rcvd := <-done
			test.exp = fmt.Sprintf("<%d>", test.pri) + test.exp
			var parsedHostname, timestamp string
			var pid int
			if n, err := fmt.Sscanf(rcvd, test.exp, &timestamp, &parsedHostname, &pid); n != 3 || err != nil || hostname != parsedHostname {
				t.Errorf("s.Info() = '%q', didn't match '%q' (%d %s)", rcvd, test.exp, n, err)
			}
		}
	}
}

func TestConcurrentWrite(t *testing.T) {
	addr, sock, srvWG := startServer("udp", "", make(chan string, 1))
	defer srvWG.Wait()
	defer sock.Close()
	w, err := Dial("udp", addr, LOG_USER|LOG_ERR, "how's it going?")
	if err != nil {
		t.Fatalf("syslog.Dial() failed: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.Info("test")
			if err != nil {
				t.Errorf("Info() failed: %v", err)
				return
			}
		}()
	}
	wg.Wait()
}

func TestConcurrentReconnect(t *testing.T) {
	crashy.Set(true)
	defer func() { crashy.Set(false) }()

	const N = 10
	const M = 100
	net := "unix"
	if !testableNetwork(net) {
		net = "tcp"
		if !testableNetwork(net) {
			t.Skipf("skipping on %s/%s; neither 'unix' or 'tcp' is supported", runtime.GOOS, runtime.GOARCH)
		}
	}
	done := make(chan string, N*M)
	addr, sock, srvWG := startServer(net, "", done)
	if net == "unix" {
		defer os.Remove(addr)
	}

	// count all the messages arriving
	count := make(chan int)
	go func() {
		ct := 0
		for range done {
			ct++
			// we are looking for 500 out of 1000 events
			// here because lots of log messages are lost
			// in buffers (kernel and/or bufio)
			if ct > N*M/2 {
				break
			}
		}
		count <- ct
	}()

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			w, err := Dial(net, addr, LOG_USER|LOG_ERR, "tag")
			if err != nil {
				t.Fatalf("syslog.Dial() failed: %v", err)
			}
			defer w.Close()
			for i := 0; i < M; i++ {
				err := w.Info("test")
				if err != nil {
					t.Errorf("Info() failed: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	sock.Close()
	srvWG.Wait()
	close(done)

	select {
	case <-count:
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout in concurrent reconnect")
	}
}

func TestLocalConn(t *testing.T) {
	messages := make([]string, 0)
	conn := newTestLocalConn(&messages)

	lc := localConn{conn: conn}

	lc.writeString(nil, nil, LOG_ERR, "hostname", "tag", "content")

	if len(messages) != 1 {
		t.Errorf("should write one message")
	}

	if messages[0] != DefaultFramer(UnixFormatter(LOG_ERR, "hostname", "tag", "content")) {
		t.Errorf("should use the unix formatter")
	}
}

type testLocalConn struct {
	messages *[]string
}

func newTestLocalConn(messages *[]string) testLocalConn {
	return testLocalConn{
		messages: messages,
	}
}

func (c testLocalConn) Write(b []byte) (int, error) {
	*c.messages = append(*c.messages, string(b))
	return len(b), nil
}

func (c testLocalConn) Close() error {
	return nil
}
