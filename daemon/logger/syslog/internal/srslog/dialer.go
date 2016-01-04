package srslog

import (
	"crypto/tls"
	"net"
)

// getDialer returns a "dialer" function that can be called to connect to a
// syslog server.
//
// Each dialer function is responsible for dialing the remote host and returns
// a serverConn, the hostname (or a default if the Writer has not specified a
// hostname), and an error in case dialing fails.
//
// The reason for separate dialers is that different network types may need
// to dial their connection differently, yet still provide a net.Conn interface
// that you can use once they have dialed. Rather than an increasingly long
// conditional, we have a map of network -> dialer function (with a sane default
// value), and adding a new network type is as easy as writing the dialer
// function and adding it to the map.
func (w Writer) getDialer() func() (serverConn, string, error) {
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

// unixDialer uses the unixSyslog method to open a connection to the syslog
// daemon running on the local machine.
func (w Writer) unixDialer() (serverConn, string, error) {
	sc, err := unixSyslog()
	hostname := w.hostname
	if hostname == "" {
		hostname = "localhost"
	}
	return sc, hostname, err
}

// tlsDialer connects to TLS over TCP, and is used for the "tcp+tls" network
// type.
func (w Writer) tlsDialer() (serverConn, string, error) {
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

// basicDialer is the most common dialer for syslog, and supports both TCP and
// UDP connections.
func (w Writer) basicDialer() (serverConn, string, error) {
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
