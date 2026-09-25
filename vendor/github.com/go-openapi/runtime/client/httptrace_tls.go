// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TLS alert codes used by the diagnostic to classify handshake
// failures. The crypto/tls package does not export named constants
// for individual alerts, so we declare the ones we care about.
// Values are from RFC 8446 §6 (the TLS 1.3 alert protocol; the
// numbering is shared with earlier TLS versions for these alerts).
//
// The `err`-prefixed names satisfy the errname linter — tls.AlertError
// implements error, so these are sentinel errors.
const (
	errTLSAlertHandshakeFailure tls.AlertError = 40
	errTLSAlertProtocolVersion  tls.AlertError = 70
)

// introspectTLSConfig returns the *tls.Config of the http.Transport
// that will run a request, when reachable, or nil otherwise.
//
// Reachable means the client's Transport is an *http.Transport
// (the default and most common case). Custom transports — wrappers
// around the default, or entirely user-provided — break introspection;
// the TLS diagnostic falls back to "configured: not introspectable"
// in that case.
//
// A nil client (zero value) or nil Transport falls through to
// [http.DefaultTransport], whose TLSClientConfig is also nil; the
// function returns nil and the diagnostic reports defaults.
func introspectTLSConfig(client *http.Client) *tls.Config {
	if client == nil {
		return nil
	}
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	t, ok := transport.(*http.Transport)
	if !ok {
		return nil
	}
	return t.TLSClientConfig
}

// emitTLSDiagnostic renders the failure-mode TLS diagnostic block.
// Called from [traceSession.onTLSHandshakeDone] when err != nil.
//
// The block covers three axes (per the plan):
//
//  1. Protocol-version negotiation — detected from
//     [errTLSAlertProtocolVersion] or a "protocol version" substring.
//  2. Cipher-suite negotiation — detected from
//     [errTLSAlertHandshakeFailure] when the user pinned CipherSuites.
//  3. Certificate-chain validity — detected from
//     [x509.CertificateInvalidError], [x509.UnknownAuthorityError]
//     or [x509.HostnameError].
//
// When none of the specific axes match, a generic fallback emits
// the raw error and whatever inspectable config the session holds.
func (s *traceSession) emitTLSDiagnostic(state tls.ConnectionState, err error) {
	s.emitf("# TLS DIAGNOSTIC")

	// tlsAxisGeneric is handled by the default branch.
	switch axis := classifyTLSError(err); axis {
	case tlsAxisProtocolVersion:
		s.diagnoseProtocolVersion(state, err)
	case tlsAxisCipher:
		s.diagnoseCipher(err)
	case tlsAxisCertChain:
		s.diagnoseCertChain(err)
	default:
		s.diagnoseTLSGeneric(err)
	}
}

// tlsAxis is the diagnostic dimension a TLS handshake error maps
// to. Axes are mutually exclusive at classification time.
type tlsAxis int

const (
	tlsAxisGeneric tlsAxis = iota
	tlsAxisProtocolVersion
	tlsAxisCipher
	tlsAxisCertChain
)

// classifyTLSError maps a TLS handshake error to one of the
// diagnostic axes. The ordering matters: cert-chain errors win
// over the generic handshake_failure alert because the alert is
// what the server sends back, but the local error type carries
// the more specific reason.
func classifyTLSError(err error) tlsAxis {
	if err == nil {
		return tlsAxisGeneric
	}

	// Cert-chain errors are the most specific local diagnostic
	// and should be reported even if a generic alert is also
	// present in the chain.
	var certInvalid x509.CertificateInvalidError
	if errors.As(err, &certInvalid) {
		return tlsAxisCertChain
	}
	var unknownAuth x509.UnknownAuthorityError
	if errors.As(err, &unknownAuth) {
		return tlsAxisCertChain
	}
	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return tlsAxisCertChain
	}

	// TLS alert classification.
	var alert tls.AlertError
	if errors.As(err, &alert) {
		switch alert {
		case errTLSAlertProtocolVersion:
			return tlsAxisProtocolVersion
		case errTLSAlertHandshakeFailure:
			return tlsAxisCipher
		}
	}

	// Fall back on substring detection for protocol-version
	// failures that arrive via the local error path rather than
	// a server-side alert (e.g. when the client refuses the
	// server's offered version).
	msg := err.Error()
	if strings.Contains(msg, "protocol version") || strings.Contains(msg, "unsupported protocol") {
		return tlsAxisProtocolVersion
	}

	return tlsAxisGeneric
}

// ---------------------------------------------------------------
// Axis renderers
// ---------------------------------------------------------------

func (s *traceSession) diagnoseProtocolVersion(state tls.ConnectionState, err error) {
	s.emitf("#   axis: protocol-version")
	s.emitf("#   error: %v", err)

	configuredMin, configuredMax := configuredVersionRange(s.tlsCfg)
	s.emitf("#   client offered: TLS %s — TLS %s",
		tlsVersionName(configuredMin), tlsVersionName(configuredMax))

	if state.Version != 0 {
		s.emitf("#   negotiated up to: TLS %s", tlsVersionName(state.Version))
	}
	s.emitf("#   suggested: widen TLSClientOptions.MinVersion/MaxVersion,")
	s.emitf("#              or pin to a version the server speaks.")
}

func (s *traceSession) diagnoseCipher(err error) {
	s.emitf("#   axis: cipher-suite")
	s.emitf("#   error: %v", err)

	if s.tlsCfg != nil && len(s.tlsCfg.CipherSuites) > 0 {
		s.emitf("#   client configured: [%s]",
			strings.Join(cipherSuiteNames(s.tlsCfg.CipherSuites), ", "))
		s.emitf("#   server set: not exposed by Go stdlib")
		s.emitf("#              (capture with: openssl s_client -cipher ALL)")
		s.emitf("#   suggested: drop the explicit CipherSuites restriction,")
		s.emitf("#              or align it with the server's policy.")
		return
	}
	// No client-side restriction. The handshake_failure alert
	// is generic; without more info we can only surface the
	// fact and suggest investigation.
	s.emitf("#   client configured: defaults (no CipherSuites restriction)")
	s.emitf("#   note: alert 40 is generic; the server may have rejected")
	s.emitf("#         the handshake for a non-cipher reason. Try")
	s.emitf("#         openssl s_client to capture details.")
}

func (s *traceSession) diagnoseCertChain(err error) {
	s.emitf("#   axis: cert-chain")

	var certInvalid x509.CertificateInvalidError
	if errors.As(err, &certInvalid) {
		s.diagnoseCertInvalid(certInvalid)
		return
	}

	var unknownAuth x509.UnknownAuthorityError
	if errors.As(err, &unknownAuth) {
		s.diagnoseUnknownAuthority(unknownAuth)
		return
	}

	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		s.diagnoseHostnameMismatch(hostnameErr)
		return
	}

	// Defensive: should not happen — classifyTLSError already
	// matched one of the three.
	s.emitf("#   error: %v", err)
}

func (s *traceSession) diagnoseCertInvalid(certInvalid x509.CertificateInvalidError) {
	cert := certInvalid.Cert
	s.emitf("#   reason: %s", certInvalidReasonName(certInvalid.Reason))

	switch certInvalid.Reason {
	case x509.Expired:
		s.emitf("#   leaf:    subject=%s", cert.Subject)
		s.emitf("#            NotBefore=%s", cert.NotBefore.UTC().Format(time.RFC3339))
		s.emitf("#            NotAfter=%s", cert.NotAfter.UTC().Format(time.RFC3339))
		s.emitf("#            now=%s", time.Now().UTC().Format(time.RFC3339))
		delta := time.Since(cert.NotAfter).Round(time.Hour)
		s.emitf("#            expired %s ago", delta)
		s.emitf("#   suggested: renew the server cert.")
	case x509.NameMismatch, x509.CANotAuthorizedForThisName:
		s.emitf("#   leaf:    subject=%s", cert.Subject)
		s.emitf("#            DNS SANs=%v", cert.DNSNames)
		s.emitf("#   suggested: set TLSClientOptions.ServerName to match")
		s.emitf("#              one of the cert SANs, or fix the cert.")
	default:
		// Less-common reasons render via the default branch (issuer + NotAfter dump).
		s.emitf("#   leaf:    subject=%s, issuer=%s", cert.Subject, cert.Issuer)
		s.emitf("#            NotBefore=%s", cert.NotBefore.UTC().Format(time.RFC3339))
		s.emitf("#            NotAfter=%s", cert.NotAfter.UTC().Format(time.RFC3339))
		s.emitf("#   error: %v", certInvalid)
	}
}

func (s *traceSession) diagnoseUnknownAuthority(unknownAuth x509.UnknownAuthorityError) {
	s.emitf("#   reason: chain root not in trust store (unknown-CA)")
	if cert := unknownAuth.Cert; cert != nil {
		s.emitf("#   offending: subject=%s", cert.Subject)
		s.emitf("#              issuer=%s", cert.Issuer)
		s.emitf("#              NotAfter=%s", cert.NotAfter.UTC().Format(time.RFC3339))
	}

	trust := "SystemCertPool"
	if s.tlsCfg != nil && s.tlsCfg.RootCAs != nil {
		trust = "TLSClientOptions.CA (custom RootCAs)"
	}
	s.emitf("#   trust store in use: %s", trust)

	s.emitf("#   suggested: set TLSClientOptions.CA to a bundle that")
	s.emitf("#              includes the issuing CA, or add it to the")
	s.emitf("#              OS trust store.")
}

func (s *traceSession) diagnoseHostnameMismatch(hostnameErr x509.HostnameError) {
	s.emitf("#   reason: hostname mismatch")
	s.emitf("#   dialed: %s", hostnameErr.Host)
	if cert := hostnameErr.Certificate; cert != nil {
		s.emitf("#   leaf:    subject=%s", cert.Subject)
		s.emitf("#            DNS SANs=%v", cert.DNSNames)
		s.emitf("#            IP SANs=%v", cert.IPAddresses)
	}
	if s.tlsCfg != nil && s.tlsCfg.ServerName != "" {
		s.emitf("#   TLSClientOptions.ServerName=%q", s.tlsCfg.ServerName)
	}
	s.emitf("#   suggested: dial the hostname listed in the cert SANs,")
	s.emitf("#              or set TLSClientOptions.ServerName to match.")
}

func (s *traceSession) diagnoseTLSGeneric(err error) {
	s.emitf("#   axis: unclassified")
	s.emitf("#   error: %v", err)
	if s.tlsCfg != nil {
		minV, maxV := configuredVersionRange(s.tlsCfg)
		s.emitf("#   configured: MinVersion=TLS %s, MaxVersion=TLS %s",
			tlsVersionName(minV), tlsVersionName(maxV))
		if s.tlsCfg.InsecureSkipVerify {
			s.emitf("#   note: TLSClientOptions.InsecureSkipVerify=true — yet")
			s.emitf("#         a TLS error still surfaced. Something deeper than")
			s.emitf("#         certificate verification is failing.")
		}
	}
}

// ---------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------

// configuredVersionRange returns the effective (Min, Max) TLS
// version range a client config negotiates. Zero values in the
// stdlib config mean "use Go default", which is TLS 1.2 .. 1.3 in
// modern Go. We materialize those defaults for display.
func configuredVersionRange(cfg *tls.Config) (uint16, uint16) {
	const (
		defaultMin = tls.VersionTLS12
		defaultMax = tls.VersionTLS13
	)
	if cfg == nil {
		return defaultMin, defaultMax
	}
	minV := cfg.MinVersion
	if minV == 0 {
		minV = defaultMin
	}
	maxV := cfg.MaxVersion
	if maxV == 0 {
		maxV = defaultMax
	}
	return minV, maxV
}

func cipherSuiteNames(ids []uint16) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, tls.CipherSuiteName(id))
	}
	return out
}

// certInvalidReasonName renders an x509.InvalidReason as a short
// human-readable label. The stdlib does not expose a String()
// method for these, so we keep a small table.
//
// Anything outside the listed cases falls through to the numeric default.
func certInvalidReasonName(r x509.InvalidReason) string {
	switch r {
	case x509.NotAuthorizedToSign:
		return "not-authorized-to-sign"
	case x509.Expired:
		return "expired"
	case x509.CANotAuthorizedForThisName:
		return "ca-not-authorized-for-this-name"
	case x509.TooManyIntermediates:
		return "too-many-intermediates"
	case x509.IncompatibleUsage:
		return "incompatible-usage"
	case x509.NameMismatch:
		return "name-mismatch"
	case x509.NameConstraintsWithoutSANs:
		return "name-constraints-without-sans"
	case x509.TooManyConstraints:
		return "too-many-constraints"
	case x509.CANotAuthorizedForExtKeyUsage:
		return "ca-not-authorized-for-ext-key-usage"
	default:
		return fmt.Sprintf("invalid-reason-%d", r)
	}
}
