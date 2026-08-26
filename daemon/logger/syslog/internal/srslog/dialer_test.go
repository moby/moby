package srslog

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"net"
	"testing"
)

func TestGetDialer(t *testing.T) {
	w := Writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "",
		network:  "",
		raddr:    "",
	}

	dialer := w.getDialer()
	if "unixDialer" != dialer.Name {
		t.Errorf("should get unixDialer, got: %v", dialer)
	}

	w.network = "tcp+tls"
	dialer = w.getDialer()
	if "tlsDialer" != dialer.Name {
		t.Errorf("should get tlsDialer, got: %v", dialer)
	}

	w.network = "tcp"
	dialer = w.getDialer()
	if "basicDialer" != dialer.Name {
		t.Errorf("should get basicDialer, got: %v", dialer)
	}

	w.network = "udp"
	dialer = w.getDialer()
	if "basicDialer" != dialer.Name {
		t.Errorf("should get basicDialer, got: %v", dialer)
	}

	w.network = "something else entirely"
	dialer = w.getDialer()
	if "basicDialer" != dialer.Name {
		t.Errorf("should get basicDialer, got: %v", dialer)
	}

	w.network = "custom"
	w.customDial = func(string, string) (net.Conn, error) { return nil, nil }
	dialer = w.getDialer()
	if "customDialer" != dialer.Name {
		t.Errorf("should get customDialer, got: %v", dialer)
	}
}

func TestUnixDialer(t *testing.T) {
	w := Writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "",
		network:  "",
		raddr:    "",
	}

	_, hostname, err := w.unixDialer()

	if err != nil {
		t.Errorf("failed to dial: %v", err)
	}

	if hostname != "localhost" {
		t.Errorf("should set blank hostname")
	}

	w.hostname = "my other hostname"

	_, hostname, err = w.unixDialer()

	if err != nil {
		t.Errorf("failed to dial: %v", err)
	}

	if hostname != "my other hostname" {
		t.Errorf("should not interfere with hostname")
	}
}

func TestTLSDialer(t *testing.T) {
	done := make(chan string)
	addr, sock, _ := startServer("tcp+tls", "", done)
	defer sock.Close()

	pool := x509.NewCertPool()
	serverCert, err := ioutil.ReadFile("test/cert.pem")
	if err != nil {
		t.Errorf("failed to read file: %v", err)
	}
	pool.AppendCertsFromPEM(serverCert)
	config := tls.Config{
		RootCAs: pool,
	}

	w := Writer{
		priority:  LOG_ERR,
		tag:       "tag",
		hostname:  "",
		network:   "tcp+tls",
		raddr:     addr,
		tlsConfig: &config,
	}

	_, hostname, err := w.tlsDialer()

	if err != nil {
		t.Errorf("failed to dial: %v", err)
	}

	if hostname == "" {
		t.Errorf("should set default hostname")
	}

	w.hostname = "my other hostname"

	_, hostname, err = w.tlsDialer()

	if err != nil {
		t.Errorf("failed to dial: %v", err)
	}

	if hostname != "my other hostname" {
		t.Errorf("should not interfere with hostname")
	}
}

func TestTCPDialer(t *testing.T) {
	done := make(chan string)
	addr, sock, _ := startServer("tcp", "", done)
	defer sock.Close()

	w := Writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "",
		network:  "tcp",
		raddr:    addr,
	}

	_, hostname, err := w.basicDialer()

	if err != nil {
		t.Errorf("failed to dial: %v", err)
	}

	if hostname == "" {
		t.Errorf("should set default hostname")
	}

	w.hostname = "my other hostname"

	_, hostname, err = w.basicDialer()

	if err != nil {
		t.Errorf("failed to dial: %v", err)
	}

	if hostname != "my other hostname" {
		t.Errorf("should not interfere with hostname")
	}
}

func TestUDPDialer(t *testing.T) {
	done := make(chan string)
	addr, sock, _ := startServer("udp", "", done)
	defer sock.Close()

	w := Writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "",
		network:  "udp",
		raddr:    addr,
	}

	_, hostname, err := w.basicDialer()

	if err != nil {
		t.Errorf("failed to dial: %v", err)
	}

	if hostname == "" {
		t.Errorf("should set default hostname")
	}

	w.hostname = "my other hostname"

	_, hostname, err = w.basicDialer()

	if err != nil {
		t.Errorf("failed to dial: %v", err)
	}

	if hostname != "my other hostname" {
		t.Errorf("should not interfere with hostname")
	}
}

func TestCustomDialer(t *testing.T) {
	// A custom dialer can really be anything, so we don't test an actual connection
	// instead we test the behavior of this code path

	nwork, addr := "custom", "custom_addr_to_pass"
	w := Writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "",
		network:  nwork,
		raddr:    addr,
		customDial: func(n string, a string) (net.Conn, error) {
			if n != nwork || a != addr {
				return nil, errors.New("Unexpected network or address, expected: (" +
					nwork + ":" + addr + ") but received (" + n + ":" + a + ")")
			}
			return fakeConn{addr: &fakeAddr{nwork, addr}}, nil
		},
	}

	_, hostname, err := w.customDialer()

	if err != nil {
		t.Errorf("failed to dial: %v", err)
	}

	if hostname == "" {
		t.Errorf("should set default hostname")
	}

	w.hostname = "my other hostname"

	_, hostname, err = w.customDialer()

	if err != nil {
		t.Errorf("failed to dial: %v", err)
	}

	if hostname != "my other hostname" {
		t.Errorf("should not interfere with hostname")
	}
}

type fakeConn struct {
	net.Conn
	addr net.Addr
}

func (fc fakeConn) Close() error {
	return nil
}

func (fc fakeConn) Write(p []byte) (int, error) {
	return len(p), nil
}

func (fc fakeConn) LocalAddr() net.Addr {
	return fc.addr
}

type fakeAddr struct {
	nwork, addr string
}

func (fa *fakeAddr) Network() string {
	return fa.nwork
}

func (fa *fakeAddr) String() string {
	return fa.addr
}
