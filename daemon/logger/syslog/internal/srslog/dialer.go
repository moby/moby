package srslog

import (
	"crypto/tls"
	"net"
)

func (w writer) getDialer() func() (serverConn, string, error) {
	dialers := map[string]func() (serverConn, string, error){
		"":        w.unixDialer,
		"tcp+tls": w.tlsDialer,
	}
	dialer, ok := dialers[w.network]
	if !ok {
		dialer = w.basicDialer
	}
	return dialer
}

func (w writer) unixDialer() (serverConn, string, error) {
	sc, err := unixSyslog()
	hostname := w.hostname
	if hostname == "" {
		hostname = "localhost"
	}
	return sc, hostname, err
}

func (w writer) tlsDialer() (serverConn, string, error) {
	c, err := tls.Dial("tcp", w.raddr, w.tlsConfig)
	var sc serverConn
	hostname := w.hostname
	if err == nil {
		sc = &netConn{conn: c}
		if hostname == "" {
			hostname = c.LocalAddr().String()
		}
	}
	return sc, hostname, err
}

func (w writer) basicDialer() (serverConn, string, error) {
	c, err := net.Dial(w.network, w.raddr)
	var sc serverConn
	hostname := w.hostname
	if err == nil {
		sc = &netConn{conn: c}
		if hostname == "" {
			hostname = c.LocalAddr().String()
		}
	}
	return sc, hostname, err
}
