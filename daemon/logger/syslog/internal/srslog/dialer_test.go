package srslog

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
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
