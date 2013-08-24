// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package netutil provides network utility functions, complementing the more
// common ones in the net package.
package netutil

import (
	"net"
	"sync"
)

// LimitListener returns a Listener that accepts at most n simultaneous
// connections from the provided Listener.
func LimitListener(l net.Listener, n int) net.Listener {
	ch := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		ch <- struct{}{}
	}
	return &limitListener{l, ch}
}

type limitListener struct {
	net.Listener
	ch chan struct{}
}

func (l *limitListener) Accept() (net.Conn, error) {
	<-l.ch
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &limitListenerConn{Conn: c, ch: l.ch}, nil
}

type limitListenerConn struct {
	net.Conn
	ch    chan<- struct{}
	close sync.Once
}

func (l *limitListenerConn) Close() error {
	err := l.Conn.Close()
	l.close.Do(func() {
		l.ch <- struct{}{}
	})
	return err
}
