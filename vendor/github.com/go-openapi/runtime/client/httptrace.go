// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

	"github.com/go-openapi/runtime/logger"
)

// traceSession owns the per-request state for [Runtime.Trace].
//
// It tracks the t=0 anchor for the connection phase, accumulates
// per-phase timestamps (for the trailing summary), and emits each
// event to the runtime logger as it fires. One session per
// SubmitContext call.
type traceSession struct {
	logger logger.Logger
	method string
	url    string

	// tlsCfg points at the *tls.Config of the http.Transport that
	// will run the request, when introspectable (i.e. the transport
	// is an *http.Transport). Used by the TLS diagnostic mode to
	// cross-check user configuration against what the handshake
	// actually attempted. Nil when the transport is custom and
	// the config cannot be reached.
	tlsCfg *tls.Config

	mu      sync.Mutex
	start   time.Time
	last    time.Time // last printed event, for relative-dt rendering
	phases  phaseTimings
	gotConn httptrace.GotConnInfo
	tlsDone tlsResult

	dnsStartAt          time.Time
	connectStartAt      time.Time
	tlsHandshakeStartAt time.Time
	wait100StartAt      time.Time
	gotConnAt           time.Time
	wroteHeadersAt      time.Time
	wroteRequestAt      time.Time
	ttfbAt              time.Time

	statusCode int
	rtError    error
}

// phaseTimings holds the per-phase durations for the trailing
// summary line. Zero values mean "phase did not occur" (e.g. no
// DNS lookup on a reused conn, no TLS on http://).
type phaseTimings struct {
	dns  time.Duration
	dial time.Duration
	tls  time.Duration
	ttfb time.Duration // time from GotConn to first response byte
}

// tlsResult captures whatever we learned from TLSHandshakeDone.
// On the happy path err is nil and state is fully populated; on
// failure state may be partial (and is what the TLS diagnostic
// mode in httptrace_tls.go works from).
type tlsResult struct {
	state tls.ConnectionState
	err   error
	done  bool
}

const tracePrefix = "[trace] "

// staleIdleThreshold is the idle duration above which a reused
// pooled connection earns a HEADS-UP annotation. Per-runtime
// configurability is deferred to v2; 30s matches the issue #336
// territory (typical NAT idle timeouts start in the 60–350s
// range, so a 30s reuse is already in "could be stale" zone).
const staleIdleThreshold = 30 * time.Second

// newTraceSession allocates a session and pre-renders the opening
// line (method + url). The session is not yet attached to a
// context — that's the caller's responsibility via session.attach.
//
// tlsCfg may be nil; when non-nil it is used by the TLS diagnostic
// mode to cross-check user-configured constraints (MinVersion,
// CipherSuites, custom RootCAs) against handshake failures.
func newTraceSession(log logger.Logger, method, url string, tlsCfg *tls.Config) *traceSession {
	s := &traceSession{
		logger: log,
		method: method,
		url:    url,
		tlsCfg: tlsCfg,
		start:  time.Now(),
	}
	s.last = s.start
	s.emitf("%s %s", method, url)
	return s
}

// attach installs the session's ClientTrace on ctx and returns the
// derived context. Callers pass the returned context to
// http.Client.Do (typically by setting it on req via
// req.WithContext) so the transport fires the hooks.
func (s *traceSession) attach(ctx context.Context) context.Context {
	return httptrace.WithClientTrace(ctx, s.clientTrace())
}

// clientTrace wires every httptrace hook to the corresponding
// session method. Each callback is responsible for its own
// locking; the stdlib does not serialize trace callbacks.
func (s *traceSession) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn:              s.onGetConn,
		GotConn:              s.onGotConn,
		PutIdleConn:          s.onPutIdleConn,
		GotFirstResponseByte: s.onGotFirstResponseByte,
		Got100Continue:       s.onGot100Continue,
		DNSStart:             s.onDNSStart,
		DNSDone:              s.onDNSDone,
		ConnectStart:         s.onConnectStart,
		ConnectDone:          s.onConnectDone,
		TLSHandshakeStart:    s.onTLSHandshakeStart,
		TLSHandshakeDone:     s.onTLSHandshakeDone,
		WroteHeaders:         s.onWroteHeaders,
		Wait100Continue:      s.onWait100Continue,
		WroteRequest:         s.onWroteRequest,
	}
}

// ---------------------------------------------------------------
// Phase callbacks (stdlib httptrace hooks)
// ---------------------------------------------------------------

func (s *traceSession) onGetConn(hostPort string) {
	s.emitTf("GetConn(%s)", hostPort)
}

func (s *traceSession) onGotConn(info httptrace.GotConnInfo) {
	s.mu.Lock()
	s.gotConn = info
	s.gotConnAt = time.Now()
	s.mu.Unlock()

	if info.Reused {
		s.emitTf("GotConn(reused=true, idle=%t, idle-time=%s)",
			info.WasIdle, info.IdleTime.Round(time.Millisecond))
	} else {
		s.emitTf("GotConn(reused=false)")
	}

	if isStaleIdleReuse(info) {
		s.emitf("# HEADS-UP: reused idle connection (idle for %s).",
			info.IdleTime.Round(time.Second))
		s.emitf("# If this request fails with EOF/connection reset, the server")
		s.emitf("# or an in-path NAT may have dropped the conn silently.")
	}
}

// isStaleIdleReuse reports whether a GotConn info indicates the
// connection came from the idle pool after sitting idle for
// longer than [staleIdleThreshold]. This is the issue #336
// pattern: long-idle pooled conns are the ones most likely to be
// dead by the time the next request tries to use them.
func isStaleIdleReuse(info httptrace.GotConnInfo) bool {
	return info.Reused && info.WasIdle && info.IdleTime > staleIdleThreshold
}

func (s *traceSession) onPutIdleConn(err error) {
	if err != nil {
		s.emitTf("PutIdleConn(err=%v)", err)
		return
	}
	s.emitTf("PutIdleConn")
}

func (s *traceSession) onGotFirstResponseByte() {
	s.mu.Lock()
	s.ttfbAt = time.Now()
	if !s.gotConnAt.IsZero() {
		s.phases.ttfb = s.ttfbAt.Sub(s.gotConnAt)
	}
	s.mu.Unlock()
	s.emitTf("GotFirstResponseByte (TTFB)")
}

func (s *traceSession) onGot100Continue() {
	s.emitTf("Got100Continue")
}

func (s *traceSession) onDNSStart(info httptrace.DNSStartInfo) {
	s.mu.Lock()
	s.dnsStartAt = time.Now()
	s.mu.Unlock()
	s.emitTf("DNSStart(host=%s)", info.Host)
}

func (s *traceSession) onDNSDone(info httptrace.DNSDoneInfo) {
	s.mu.Lock()
	if !s.dnsStartAt.IsZero() {
		s.phases.dns = time.Since(s.dnsStartAt)
	}
	s.mu.Unlock()

	addrs := make([]string, 0, len(info.Addrs))
	for _, a := range info.Addrs {
		addrs = append(addrs, a.String())
	}
	if info.Err != nil {
		s.emitTf("DNSDone(err=%v, addrs=[%s], coalesced=%t)",
			info.Err, strings.Join(addrs, " "), info.Coalesced)
		return
	}
	s.emitTf("DNSDone(addrs=[%s], coalesced=%t)",
		strings.Join(addrs, " "), info.Coalesced)
}

func (s *traceSession) onConnectStart(network, addr string) {
	s.mu.Lock()
	s.connectStartAt = time.Now()
	s.mu.Unlock()
	s.emitTf("ConnectStart(%s %s)", network, addr)
}

func (s *traceSession) onConnectDone(network, addr string, err error) {
	s.mu.Lock()
	if !s.connectStartAt.IsZero() {
		s.phases.dial = time.Since(s.connectStartAt)
	}
	s.mu.Unlock()

	if err != nil {
		s.emitTf("ConnectDone(%s %s, err=%v)", network, addr, err)
		return
	}
	s.emitTf("ConnectDone(%s %s)", network, addr)
}

func (s *traceSession) onTLSHandshakeStart() {
	s.mu.Lock()
	s.tlsHandshakeStartAt = time.Now()
	s.mu.Unlock()
	s.emitTf("TLSHandshakeStart")
}

func (s *traceSession) onTLSHandshakeDone(state tls.ConnectionState, err error) {
	s.mu.Lock()
	if !s.tlsHandshakeStartAt.IsZero() {
		s.phases.tls = time.Since(s.tlsHandshakeStartAt)
	}
	s.tlsDone = tlsResult{state: state, err: err, done: true}
	s.mu.Unlock()

	if err != nil {
		s.emitTf("TLSHandshakeDone(err=%v)", err)
		s.emitTLSDiagnostic(state, err)
		return
	}
	s.emitTf("TLSHandshakeDone(tls=%s, cipher=%s, server=%s%s)",
		tlsVersionName(state.Version),
		tls.CipherSuiteName(state.CipherSuite),
		state.ServerName,
		certExpiryFragment(state),
	)
}

func (s *traceSession) onWroteHeaders() {
	s.mu.Lock()
	s.wroteHeadersAt = time.Now()
	s.mu.Unlock()
	s.emitTf("WroteHeaders")
}

func (s *traceSession) onWait100Continue() {
	s.mu.Lock()
	s.wait100StartAt = time.Now()
	s.mu.Unlock()
	s.emitTf("Wait100Continue")
}

func (s *traceSession) onWroteRequest(info httptrace.WroteRequestInfo) {
	s.mu.Lock()
	s.wroteRequestAt = time.Now()
	s.mu.Unlock()

	if info.Err != nil {
		s.emitTf("WroteRequest(err=%v)", info.Err)
		return
	}
	s.emitTf("WroteRequest")
}

// ---------------------------------------------------------------
// Body wrapping
// ---------------------------------------------------------------

// bodySide identifies which direction an instrumented body is on.
type bodySide string

const (
	bodySend bodySide = "Sent"
	bodyRecv bodySide = "Received"
)

// instrumentedBody wraps an [io.ReadCloser] and emits a
// BodyChunk{Sent,Received} trace event per Read call. Tracks the
// inter-read delay in `dt` so users can see streaming-body
// cadence.
//
// Read granularity: bytes returned by the underlying body, not
// HTTP/1.1 chunked-framing units. For wire-level chunking, use
// [Runtime.Debug] instead.
//
// Concurrency: a single body is read from a single goroutine in
// practice (http.Transport for request bodies, the application
// for response bodies), so no internal locking is needed beyond
// what the underlying ReadCloser provides.
type instrumentedBody struct {
	wrapped io.ReadCloser
	sess    *traceSession
	side    bodySide
	last    time.Time
}

func (b *instrumentedBody) Read(p []byte) (int, error) {
	n, err := b.wrapped.Read(p)
	if n > 0 {
		first := b.last.IsZero()
		var dt time.Duration
		if !first {
			dt = time.Since(b.last)
		}
		b.last = time.Now()
		b.sess.onBodyChunk(b.side, n, dt, first)
	}
	return n, err
}

func (b *instrumentedBody) Close() error {
	return b.wrapped.Close()
}

// wrapRequestBody returns an instrumented wrapper around the
// outgoing request body, or the original body if nil (which is
// the common case for GET requests). The wrapper observes
// Transport-side reads, so BodyChunkSent events appear between
// WroteHeaders and WroteRequest in the trace timeline.
func (s *traceSession) wrapRequestBody(body io.ReadCloser) io.ReadCloser {
	if body == nil {
		return nil
	}
	return &instrumentedBody{wrapped: body, sess: s, side: bodySend}
}

// wrapResponseBody returns an instrumented wrapper around the
// incoming response body. Stacks cleanly above
// [KeepAliveTransport]'s drain-on-close behavior.
func (s *traceSession) wrapResponseBody(body io.ReadCloser) io.ReadCloser {
	if body == nil {
		return nil
	}
	return &instrumentedBody{wrapped: body, sess: s, side: bodyRecv}
}

// onBodyChunk renders a single BodyChunk{Sent,Received} event.
// dt is the duration since the previous Read on the same body and
// is meaningful only when `first` is false. The first chunk has no
// preceding read, so the dt= field is suppressed; every subsequent
// chunk emits dt= unconditionally — even when the measured value
// rounds to zero (common on Windows, where the system clock
// resolution is coarser than a fast loopback read loop).
func (s *traceSession) onBodyChunk(side bodySide, n int, dt time.Duration, first bool) {
	if first {
		s.emitTf("BodyChunk%s(n=%d)", side, n)
		return
	}
	s.emitTf("BodyChunk%s(n=%d, dt=%s)", side, n, round(dt))
}

// ---------------------------------------------------------------
// Submit-level lifecycle hooks (called from SubmitContext)
// ---------------------------------------------------------------

// onRoundTripError is called by SubmitContext when http.Client.Do
// returns an error. It records the error for the summary line.
func (s *traceSession) onRoundTripError(err error) {
	s.mu.Lock()
	s.rtError = err
	s.mu.Unlock()
	s.emitTf("! error: %v", err)
}

// onResponse is called when http.Client.Do returns successfully.
// It records the status code for the summary line.
func (s *traceSession) onResponse(statusCode int) {
	s.mu.Lock()
	s.statusCode = statusCode
	s.mu.Unlock()
}

// finish renders the trailing single-line summary and is called
// by SubmitContext after the response body has been consumed (or
// on error path, after the error was recorded). When a round-trip
// error happened on a stale-idle reused connection, a tail block
// flags the issue #336 pattern explicitly.
func (s *traceSession) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()

	total := time.Since(s.start)
	var b strings.Builder
	fmt.Fprintf(&b, "Summary: %s — ", s.method)
	if s.rtError != nil {
		fmt.Fprintf(&b, "FAILED (%v)", s.rtError)
	} else {
		fmt.Fprintf(&b, "%d", s.statusCode)
	}
	if s.phases.dns > 0 {
		fmt.Fprintf(&b, ", dns=%s", round(s.phases.dns))
	}
	if s.phases.dial > 0 {
		fmt.Fprintf(&b, ", dial=%s", round(s.phases.dial))
	}
	if s.phases.tls > 0 {
		fmt.Fprintf(&b, ", tls=%s", round(s.phases.tls))
	}
	if s.phases.ttfb > 0 {
		fmt.Fprintf(&b, ", ttfb=%s", round(s.phases.ttfb))
	}
	fmt.Fprintf(&b, ", total=%s", round(total))

	s.emitRaw(b.String())

	// issue #336 tail annotation: a round-trip failure on a
	// stale-idle reused conn is the canonical pattern.
	if s.rtError != nil && isStaleIdleReuse(s.gotConn) {
		s.emitf("# FAILED on a reused idle conn (%s idle).",
			s.gotConn.IdleTime.Round(time.Second))
		s.emitf("# Silently closed the conn while it sat in the idle pool.")
		s.emitf("# Consider lowering http.Transport.IdleConnTimeout to evict")
		s.emitf("# pooled conns before the NAT/server side does.")
	}
}

// ---------------------------------------------------------------
// Emission helpers
// ---------------------------------------------------------------

// emitf prints a plain event line (no t= timestamp). Used for the
// opening line and the summary.
func (s *traceSession) emitf(format string, args ...any) {
	s.logger.Debugf(tracePrefix+format, args...)
}

// emitRaw is like emitf but takes an already-rendered string. Used
// by finish() which builds its line via strings.Builder.
func (s *traceSession) emitRaw(line string) {
	s.logger.Debugf("%s", tracePrefix+line)
}

// emitTf prints a phase event with a cumulative t=... offset from
// the session start.
func (s *traceSession) emitTf(format string, args ...any) {
	t := round(time.Since(s.start))
	msg := fmt.Sprintf(format, args...)
	s.logger.Debugf(tracePrefix+"%s (t=%s)", msg, t)
}

// traceRoundUnit is the rounding granularity for >=1ms durations
// rendered in trace output. 100µs keeps lines readable while
// preserving enough resolution to spot millisecond-scale phase
// differences.
const traceRoundUnit = 100 * time.Microsecond

// round trims durations for human-readable trace output.
// Sub-millisecond durations round to 1µs (preserves visibility on
// fast loopback servers); >=1ms durations round to [traceRoundUnit].
func round(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	if d < time.Millisecond {
		return d.Round(time.Microsecond)
	}
	return d.Round(traceRoundUnit)
}

// ---------------------------------------------------------------
// TLS rendering helpers
// ---------------------------------------------------------------

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "1.0"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS13:
		return "1.3"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}

// certExpiryFragment renders ", expires=YYYY-MM-DD" for the leaf
// cert when available, or an empty string otherwise.
func certExpiryFragment(state tls.ConnectionState) string {
	if len(state.PeerCertificates) == 0 {
		return ""
	}
	return ", expires=" + state.PeerCertificates[0].NotAfter.UTC().Format("2006-01-02")
}
