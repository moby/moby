package srslog

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"reflect"
	"testing"
)

// funcEquals checks if the two given functions are the same. This grabs the
// function pointer of each of them and compares them. This seems to be a viable
// way to work around the fact that Go does not have actual function equality.
func funcEquals(f1, f2 interface{}) bool {
	av := reflect.ValueOf(f1).Pointer()
	bv := reflect.ValueOf(f2).Pointer()

	return av == bv
}

func TestGetDialer(t *testing.T) {
	w := Writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "",
		network:  "",
		raddr:    "",
	}

	dialer := w.getDialer()
	if !funcEquals(w.unixDialer, dialer) {
		t.Errorf("should get unixDialer, got: %v", dialer)
	}

	w.network = "tcp+tls"
	dialer = w.getDialer()
	if !funcEquals(w.tlsDialer, dialer) {
		t.Errorf("should get tlsDialer, got: %v", dialer)
	}

	w.network = "tcp"
	dialer = w.getDialer()
	if !funcEquals(w.basicDialer, dialer) {
		t.Errorf("should get basicDialer, got: %v", dialer)
	}

	w.network = "udp"
	dialer = w.getDialer()
	if !funcEquals(w.basicDialer, dialer) {
		t.Errorf("should get basicDialer, got: %v", dialer)
	}

	w.network = "something else entirely"
	dialer = w.getDialer()
	if !funcEquals(w.basicDialer, dialer) {
		t.Errorf("should get basicDialer, got: %v", dialer)
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
