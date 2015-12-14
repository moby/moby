package srslog

import (
	"testing"
)

func TestCloseNonOpenWriter(t *testing.T) {
	w := writer{}

	err := w.Close()
	if err != nil {
		t.Errorf("should not fail to close if there is nothing to close")
	}
}

func TestWriteAndRetryFails(t *testing.T) {
	w := writer{network: "udp", raddr: "fakehost"}

	n, err := w.writeAndRetry(LOG_ERR, "nope")
	if err == nil {
		t.Errorf("should fail to write")
	}
	if n != 0 {
		t.Errorf("should not write any bytes")
	}
}

func TestDebug(t *testing.T) {
	done := make(chan string)
	addr, sock, srvWG := startServer("udp", "", done)
	defer sock.Close()
	defer srvWG.Wait()

	w := writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "hostname",
		network:  "udp",
		raddr:    addr,
	}

	w.Lock()
	err := w.connect()
	if err != nil {
		t.Errorf("failed to connect: %v", err)
		w.Unlock()
	}
	w.Unlock()
	defer w.Close()

	err = w.Debug("this is a test message")
	if err != nil {
		t.Errorf("failed to debug: %v", err)
	}

	checkWithPriorityAndTag(t, LOG_DEBUG, "tag", "hostname", "this is a test message", <-done)
}

func TestInfo(t *testing.T) {
	done := make(chan string)
	addr, sock, srvWG := startServer("udp", "", done)
	defer sock.Close()
	defer srvWG.Wait()

	w := writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "hostname",
		network:  "udp",
		raddr:    addr,
	}

	w.Lock()
	err := w.connect()
	if err != nil {
		t.Errorf("failed to connect: %v", err)
		w.Unlock()
	}
	w.Unlock()
	defer w.Close()

	err = w.Info("this is a test message")
	if err != nil {
		t.Errorf("failed to info: %v", err)
	}

	checkWithPriorityAndTag(t, LOG_INFO, "tag", "hostname", "this is a test message", <-done)
}

func TestNotice(t *testing.T) {
	done := make(chan string)
	addr, sock, srvWG := startServer("udp", "", done)
	defer sock.Close()
	defer srvWG.Wait()

	w := writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "hostname",
		network:  "udp",
		raddr:    addr,
	}

	w.Lock()
	err := w.connect()
	if err != nil {
		t.Errorf("failed to connect: %v", err)
		w.Unlock()
	}
	w.Unlock()
	defer w.Close()

	err = w.Notice("this is a test message")
	if err != nil {
		t.Errorf("failed to notice: %v", err)
	}

	checkWithPriorityAndTag(t, LOG_NOTICE, "tag", "hostname", "this is a test message", <-done)
}

func TestWarning(t *testing.T) {
	done := make(chan string)
	addr, sock, srvWG := startServer("udp", "", done)
	defer sock.Close()
	defer srvWG.Wait()

	w := writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "hostname",
		network:  "udp",
		raddr:    addr,
	}

	w.Lock()
	err := w.connect()
	if err != nil {
		t.Errorf("failed to connect: %v", err)
		w.Unlock()
	}
	w.Unlock()
	defer w.Close()

	err = w.Warning("this is a test message")
	if err != nil {
		t.Errorf("failed to warn: %v", err)
	}

	checkWithPriorityAndTag(t, LOG_WARNING, "tag", "hostname", "this is a test message", <-done)
}

func TestErr(t *testing.T) {
	done := make(chan string)
	addr, sock, srvWG := startServer("udp", "", done)
	defer sock.Close()
	defer srvWG.Wait()

	w := writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "hostname",
		network:  "udp",
		raddr:    addr,
	}

	w.Lock()
	err := w.connect()
	if err != nil {
		t.Errorf("failed to connect: %v", err)
		w.Unlock()
	}
	w.Unlock()
	defer w.Close()

	err = w.Err("this is a test message")
	if err != nil {
		t.Errorf("failed to err: %v", err)
	}

	checkWithPriorityAndTag(t, LOG_ERR, "tag", "hostname", "this is a test message", <-done)
}

func TestCrit(t *testing.T) {
	done := make(chan string)
	addr, sock, srvWG := startServer("udp", "", done)
	defer sock.Close()
	defer srvWG.Wait()

	w := writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "hostname",
		network:  "udp",
		raddr:    addr,
	}

	w.Lock()
	err := w.connect()
	if err != nil {
		t.Errorf("failed to connect: %v", err)
		w.Unlock()
	}
	w.Unlock()
	defer w.Close()

	err = w.Crit("this is a test message")
	if err != nil {
		t.Errorf("failed to crit: %v", err)
	}

	checkWithPriorityAndTag(t, LOG_CRIT, "tag", "hostname", "this is a test message", <-done)
}

func TestAlert(t *testing.T) {
	done := make(chan string)
	addr, sock, srvWG := startServer("udp", "", done)
	defer sock.Close()
	defer srvWG.Wait()

	w := writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "hostname",
		network:  "udp",
		raddr:    addr,
	}

	w.Lock()
	err := w.connect()
	if err != nil {
		t.Errorf("failed to connect: %v", err)
		w.Unlock()
	}
	w.Unlock()
	defer w.Close()

	err = w.Alert("this is a test message")
	if err != nil {
		t.Errorf("failed to alert: %v", err)
	}

	checkWithPriorityAndTag(t, LOG_ALERT, "tag", "hostname", "this is a test message", <-done)
}

func TestEmerg(t *testing.T) {
	done := make(chan string)
	addr, sock, srvWG := startServer("udp", "", done)
	defer sock.Close()
	defer srvWG.Wait()

	w := writer{
		priority: LOG_ERR,
		tag:      "tag",
		hostname: "hostname",
		network:  "udp",
		raddr:    addr,
	}

	w.Lock()
	err := w.connect()
	if err != nil {
		t.Errorf("failed to connect: %v", err)
		w.Unlock()
	}
	w.Unlock()
	defer w.Close()

	err = w.Emerg("this is a test message")
	if err != nil {
		t.Errorf("failed to emerg: %v", err)
	}

	checkWithPriorityAndTag(t, LOG_EMERG, "tag", "hostname", "this is a test message", <-done)
}
