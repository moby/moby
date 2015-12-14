package srslog

import (
	"crypto/tls"
	"net"
	"strings"
	"sync"
)

// A writer is a connection to a syslog server.
type writer struct {
	sync.Mutex // guards conn

	priority  priority
	tag       string
	hostname  string
	network   string
	raddr     string
	tlsConfig *tls.Config

	conn serverConn
}

func (w writer) emptyDialer() (serverConn, string, error) {
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

func (w writer) getDialer() func() (serverConn, string, error) {
	dialers := map[string]func() (serverConn, string, error){
		"":        w.emptyDialer,
		"tcp+tls": w.tlsDialer,
	}
	dialer, ok := dialers[w.network]
	if !ok {
		dialer = w.basicDialer
	}
	return dialer
}

// connect makes a connection to the syslog server.
// It must be called with w.mu held.
func (w *writer) connect() (err error) {
	if w.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		w.conn.close()
		w.conn = nil
	}

	var conn serverConn
	var hostname string
	dialer := w.getDialer()
	conn, hostname, err = dialer()
	if err == nil {
		w.conn = conn
		w.hostname = hostname
	}

	return
}

// Write sends a log message to the syslog daemon.
func (w *writer) Write(b []byte) (int, error) {
	return w.writeAndRetry(w.priority, string(b))
}

// Close closes a connection to the syslog daemon.
func (w *writer) Close() error {
	w.Lock()
	defer w.Unlock()

	if w.conn != nil {
		err := w.conn.close()
		w.conn = nil
		return err
	}
	return nil
}

// Emerg logs a message with severity LOG_EMERG, ignoring the severity
// passed to New.
func (w *writer) Emerg(m string) (err error) {
	_, err = w.writeAndRetry(LOG_EMERG, m)
	return err
}

// Alert logs a message with severity LOG_ALERT, ignoring the severity
// passed to New.
func (w *writer) Alert(m string) (err error) {
	_, err = w.writeAndRetry(LOG_ALERT, m)
	return err
}

// Crit logs a message with severity LOG_CRIT, ignoring the severity
// passed to New.
func (w *writer) Crit(m string) (err error) {
	_, err = w.writeAndRetry(LOG_CRIT, m)
	return err
}

// Err logs a message with severity LOG_ERR, ignoring the severity
// passed to New.
func (w *writer) Err(m string) (err error) {
	_, err = w.writeAndRetry(LOG_ERR, m)
	return err
}

// Warning logs a message with severity LOG_WARNING, ignoring the
// severity passed to New.
func (w *writer) Warning(m string) (err error) {
	_, err = w.writeAndRetry(LOG_WARNING, m)
	return err
}

// Notice logs a message with severity LOG_NOTICE, ignoring the
// severity passed to New.
func (w *writer) Notice(m string) (err error) {
	_, err = w.writeAndRetry(LOG_NOTICE, m)
	return err
}

// Info logs a message with severity LOG_INFO, ignoring the severity
// passed to New.
func (w *writer) Info(m string) (err error) {
	_, err = w.writeAndRetry(LOG_INFO, m)
	return err
}

// Debug logs a message with severity LOG_DEBUG, ignoring the severity
// passed to New.
func (w *writer) Debug(m string) (err error) {
	_, err = w.writeAndRetry(LOG_DEBUG, m)
	return err
}

func (w *writer) writeAndRetry(p priority, s string) (int, error) {
	pr := (w.priority & facilityMask) | (p & severityMask)

	w.Lock()
	defer w.Unlock()

	if w.conn != nil {
		if n, err := w.write(pr, s); err == nil {
			return n, err
		}
	}
	if err := w.connect(); err != nil {
		return 0, err
	}
	return w.write(pr, s)
}

// write generates and writes a syslog formatted string. The
// format is as follows: <PRI>TIMESTAMP HOSTNAME TAG[PID]: MSG
func (w *writer) write(p priority, msg string) (int, error) {
	// ensure it ends in a \n
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}

	err := w.conn.writeString(p, w.hostname, w.tag, msg)
	if err != nil {
		return 0, err
	}
	// Note: return the length of the input, not the number of
	// bytes printed by Fprintf, because this must behave like
	// an io.Writer.
	return len(msg), nil
}
