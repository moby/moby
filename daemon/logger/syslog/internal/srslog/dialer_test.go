package srslog

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"reflect"
	"runtime"
	"testing"
)

func TestGetDialer(t *testing.T) {
	w := writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "",
		network:  "",
		raddr:    "",
	}

	dialer := w.getDialer()
	name := runtime.FuncForPC(reflect.ValueOf(dialer).Pointer()).Name()
	if name != "github.com/RackSec/srslog.(writer).(github.com/RackSec/srslog.unixDialer)-fm" {
		t.Errorf("should get unixDialer, got: %v", name)
	}

	w.network = "tcp+tls"
	dialer = w.getDialer()
	name = runtime.FuncForPC(reflect.ValueOf(dialer).Pointer()).Name()
	if name != "github.com/RackSec/srslog.(writer).(github.com/RackSec/srslog.tlsDialer)-fm" {
		t.Errorf("should get tlsDialer, got: %v", name)
	}

	w.network = "tcp"
	dialer = w.getDialer()
	name = runtime.FuncForPC(reflect.ValueOf(dialer).Pointer()).Name()
	if name != "github.com/RackSec/srslog.(writer).(github.com/RackSec/srslog.basicDialer)-fm" {
		t.Errorf("should get basicDialer, got: %v", name)
	}

	w.network = "udp"
	dialer = w.getDialer()
	name = runtime.FuncForPC(reflect.ValueOf(dialer).Pointer()).Name()
	if name != "github.com/RackSec/srslog.(writer).(github.com/RackSec/srslog.basicDialer)-fm" {
		t.Errorf("should get basicDialer, got: %v", name)
	}

	w.network = "something else entirely"
	dialer = w.getDialer()
	name = runtime.FuncForPC(reflect.ValueOf(dialer).Pointer()).Name()
	if name != "github.com/RackSec/srslog.(writer).(github.com/RackSec/srslog.basicDialer)-fm" {
		t.Errorf("should get basicDialer, got: %v", name)
	}
}

func TestUnixDialer(t *testing.T) {
	w := writer{
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

	w := writer{
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

	w := writer{
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

	w := writer{
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
