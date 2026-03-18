package main

import (
	"net"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// TestUDPOneSided makes sure that the conntrack entry isn't GC'd if the
// backend never writes to the UDP client.
func TestUDPOneSided(t *testing.T) {
	frontend, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	assert.NilError(t, err)
	defer frontend.Close()

	backend, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	assert.NilError(t, err)
	defer backend.Close()

	type udpMsg struct {
		data  []byte
		saddr *net.UDPAddr
	}
	msgs := make(chan udpMsg)
	go func() {
		for {
			buf := make([]byte, 1024)
			n, saddr, err := backend.ReadFromUDP(buf)
			if err != nil {
				return
			}
			msgs <- udpMsg{data: buf[:n], saddr: saddr}
		}
	}()

	proxy, err := NewUDPProxy(frontend, backend.LocalAddr().(*net.UDPAddr), ip4)
	assert.NilError(t, err)
	defer proxy.Close()

	const connTrackTimeout = 1 * time.Second
	proxy.connTrackTimeout = connTrackTimeout

	go func() {
		proxy.Run()
	}()

	client, err := net.DialUDP("udp", nil, frontend.LocalAddr().(*net.UDPAddr))
	assert.NilError(t, err)
	defer client.Close()

	var expSaddr *net.UDPAddr
	for i := range 15 {
		_, err = client.Write([]byte("hello"))
		assert.NilError(t, err)
		time.Sleep(100 * time.Millisecond)

		msg := <-msgs
		assert.Equal(t, string(msg.data), "hello")
		if i == 0 {
			expSaddr = msg.saddr
		} else {
			assert.Equal(t, msg.saddr.Port, expSaddr.Port)
		}
	}

	// The conntrack entry is checked every connTrackTimeout, but the latest
	// write might be less than connTrackTimeout ago. So we need to wait for
	// at least twice the conntrack timeout to make sure the entry is GC'd.
	time.Sleep(2 * connTrackTimeout)
	_, err = client.Write([]byte("hello"))
	assert.NilError(t, err)

	msg := <-msgs
	assert.Equal(t, string(msg.data), "hello")
	assert.Check(t, msg.saddr.Port != expSaddr.Port)
}
