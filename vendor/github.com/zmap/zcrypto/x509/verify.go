// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode/utf8"
)

type InvalidReason int

const (
	// NotAuthorizedToSign results when a certificate is signed by another
	// which isn't marked as a CA certificate.
	NotAuthorizedToSign InvalidReason = iota

	// Expired results when a certificate has expired, based on the time
	// given in the VerifyOptions.
	Expired

	// CANotAuthorizedForThisName results when an intermediate or root
	// certificate has a name constraint which doesn't include the name
	// being checked.
	CANotAuthorizedForThisName

	// CANotAuthorizedForThisEmail results when an intermediate or root
	// certificate has a name constraint which doesn't include the email
	// being checked.
	CANotAuthorizedForThisEmail

	// CANotAuthorizedForThisIP results when an intermediate or root
	// certificate has a name constraint which doesn't include the IP
	// being checked.
	CANotAuthorizedForThisIP

	// CANotAuthorizedForThisDirectory results when an intermediate or root
	// certificate has a name constraint which doesn't include the directory
	// being checked.
	CANotAuthorizedForThisDirectory

	// TooManyIntermediates results when a path length constraint is
	// violated.
	TooManyIntermediates

	// IncompatibleUsage results when the certificate's key usage indicates
	// that it may only be used for a different purpose.
	IncompatibleUsage

	// NameMismatch results when the subject name of a parent certificate
	// does not match the issuer name in the child.
	NameMismatch

	// NeverValid results when the certificate could never have been valid due to
	// some date-related issue, e.g. NotBefore > NotAfter.
	NeverValid

	// IsSelfSigned results when the certificate is self-signed and not a trusted
	// root.
	IsSelfSigned
)

// CertificateInvalidError results when an odd error occurs. Users of this
// library probably want to handle all these errors uniformly.
type CertificateInvalidError struct {
	Cert   *Certificate
	Reason InvalidReason
}

func (e CertificateInvalidError) Error() string {
	switch e.Reason {
	case NotAuthorizedToSign:
		return "x509: certificate is not authorized to sign other certificates"
	case Expired:
		return "x509: certificate has expired or is not yet valid"
	case CANotAuthorizedForThisName:
		return "x509: a root or intermediate certificate is not authorized to sign in this domain"
	case CANotAuthorizedForThisEmail:
		return "x509: a root or intermediate certificate is not authorized to sign this email address"
	case CANotAuthorizedForThisIP:
		return "x509: a root or intermediate certificate is not authorized to sign this IP address"
	case CANotAuthorizedForThisDirectory:
		return "x509: a root or intermediate certificate is not authorized to sign in this directory"
	case TooManyIntermediates:
		return "x509: too many intermediates for path length constraint"
	case IncompatibleUsage:
		return "x509: certificate specifies an incompatible key usage"
	case NameMismatch:
		return "x509: issuer name does not match subject from issuing certificate"
	case NeverValid:
		return "x509: certificate will never be valid"
	}
	return "x509: unknown error"
}

// HostnameError results when the set of authorized names doesn't match the
// requested name.
type HostnameError struct {
	Certificate *Certificate
	Host        string
}

func (h HostnameError) Error() string {
	c := h.Certificate

	var valid string
	if ip := net.ParseIP(h.Host); ip != nil {
		// Trying to validate an IP
		if len(c.IPAddresses) == 0 {
			return "x509: cannot validate certificate for " + h.Host + " because it doesn't contain any IP SANs"
		}
		for _, san := range c.IPAddresses {
			if len(valid) > 0 {
				valid += ", "
			}
			valid += san.String()
		}
	} else {
		if c.hasSANExtension() {
			valid = strings.Join(c.DNSNames, ", ")
		} else {
			valid = c.Subject.CommonName
		}
	}

	if len(valid) == 0 {
		return "x509: certificate is not valid for any names, but wanted to match " + h.Host
	}
	return "x509: certificate is valid for " + valid + ", not " + h.Host
}

// UnknownAuthorityError results when the certificate issuer is unknown
type UnknownAuthorityError struct {
	Cert *Certificate
	// hintErr contains an error that may be helpful in determining why an
	// authority wasn't found.
	hintErr error
	// hintCert contains a possible authority certificate that was rejected
	// because of the error in hintErr.
	hintCert *Certificate
}

func (e UnknownAuthorityError) Error() string {
	s := "x509: certificate signed by unknown authority"
	if e.hintErr != nil {
		certName := e.hintCert.Subject.CommonName
		if len(certName) == 0 {
			if len(e.hintCert.Subject.Organization) > 0 {
				certName = e.hintCert.Subject.Organization[0]
			} else {
				certName = "serial:" + e.hintCert.SerialNumber.String()
			}
		}
		s += fmt.Sprintf(" (possibly because of %q while trying to verify candidate authority certificate %q)", e.hintErr, certName)
	}
	return s
}

// SystemRootsError results when we fail to load the system root certificates.
type SystemRootsError struct {
	Err error
}

func (se SystemRootsError) Error() string {
	msg := "x509: failed to load system roots and no roots provided"
	if se.Err != nil {
		return msg + "; " + se.Err.Error()
	}
	return msg
}

// errNotParsed is returned when a certificate without ASN.1 contents is
// verified. Platform-specific verification needs the ASN.1 contents.
var errNotParsed = errors.New("x509: missing ASN.1 contents; use ParseCertificate")

const maxIntermediateCount = 10

// VerifyOptions contains parameters for Certificate.Verify. It's a structure
// because other PKIX verification APIs have ended up needing many options.
type VerifyOptions struct {
	DNSName      string
	EmailAddress string
	IPAddress    net.IP

	Intermediates *CertPool
	Roots         *CertPool // if nil, the system roots are used
	CurrentTime   time.Time // if zero, the current time is used
	// KeyUsage specifies which Extended Key Usage values are acceptable.
	// An empty list means ExtKeyUsageServerAuth. Key usage is considered a
	// constraint down the chain which mirrors Windows CryptoAPI behaviour,
	// but not the spec. To accept any key usage, include ExtKeyUsageAny.
	KeyUsages []ExtKeyUsage
}

const (
	leafCertificate = iota
	intermediateCertificate
	rootCertificate
)

func matchNameConstraint(domain, constraint string) bool {
	// The meaning of zero length constraints is not specified, but this
	// code follows NSS and accepts them as matching everything.
	if len(constraint) == 0 {
		return true
	}

	if len(domain) < len(constraint) {
		return false
	}

	prefixLen := len(domain) - len(constraint)
	if !strings.EqualFold(domain[prefixLen:], constraint) {
		return false
	}

	if prefixLen == 0 {
		return true
	}

	isSubdomain := domain[prefixLen-1] == '.'
	constraintHasLeadingDot := constraint[0] == '.'
	return isSubdomain != constraintHasLeadingDot
}

// NOTE: the stdlib function does many more checks and is preferable. For backwards compatibility using this version

// isValid performs validity checks on the c. It will never return a
// date-related error.
func (c *Certificate) isValid(certType CertificateType, currentChain CertificateChain) error {

	// KeyUsage status flags are ignored. From Engineering Security, Peter
	// Gutmann: A European government CA marked its signing certificates as
	// being valid for encryption only, but no-one noticed. Another
	// European CA marked its signature keys as not being valid for
	// signatures. A different CA marked its own trusted root certificate
	// as being invalid for certificate signing.  Another national CA
	// distributed a certificate to be used to encrypt data for the
	// country’s tax authority that was marked as only being usable for
	// digital signatures but not for encryption. Yet another CA reversed
	// the order of the bit flags in the keyUsage due to confusion over
	// encoding endianness, essentially setting a random keyUsage in
	// certificates that it issued. Another CA created a self-invalidating
	// certificate by adding a certificate policy statement stipulating
	// that the certificate had to be used strictly as specified in the
	// keyUsage, and a keyUsage containing a flag indicating that the RSA
	// encryption key could only be used for Diffie-Hellman key agreement.

	if certType == CertificateTypeIntermediate && (!c.BasicConstraintsValid || !c.IsCA) {
		return CertificateInvalidError{c, NotAuthorizedToSign}
	}

	if c.BasicConstraintsValid && c.MaxPathLen >= 0 {
		numIntermediates := len(currentChain) - 1
		if numIntermediates > c.MaxPathLen {
			return CertificateInvalidError{c, TooManyIntermediates}
		}
	}

	if len(currentChain) > maxIntermediateCount {
		return CertificateInvalidError{c, TooManyIntermediates}
	}

	return nil
}

// Verify attempts to verify c by building one or more chains from c to a
// certificate in opts.Roots, using certificates in opts.Intermediates if
// needed. If successful, it returns one or more chains where the first
// element of the chain is c and the last element is from opts.Roots.
//
// If opts.Roots is nil and system roots are unavailable the returned error
// will be of type SystemRootsError.
//
// WARNING: this doesn't do any revocation checking.
func (c *Certificate) Verify(opts VerifyOptions) (current, expired, never []CertificateChain, err error) {

	if opts.Roots == nil {
		err = SystemRootsError{}
		return
	}

	err = c.isValid(CertificateTypeLeaf, nil)
	if err != nil {
		return
	}

	candidateChains, err := c.buildChains(make(map[int][]CertificateChain), []*Certificate{c}, &opts)
	if err != nil {
		return
	}

	keyUsages := opts.KeyUsages
	if len(keyUsages) == 0 {
		keyUsages = []ExtKeyUsage{ExtKeyUsageServerAuth}
	}

	// If any key usage is acceptable then we're done.
	hasKeyUsageAny := false
	for _, usage := range keyUsages {
		if usage == ExtKeyUsageAny {
			hasKeyUsageAny = true
			break
		}
	}

	var chains []CertificateChain
	if hasKeyUsageAny {
		chains = candidateChains
	} else {
		for _, candidate := range candidateChains {
			if checkChainForKeyUsage(candidate, keyUsages) {
				chains = append(chains, candidate)
			}
		}
	}

	if len(chains) == 0 {
		err = CertificateInvalidError{c, IncompatibleUsage}
		return
	}

	current, expired, never = FilterByDate(chains, opts.CurrentTime)
	if len(current) == 0 {
		if len(expired) > 0 {
			err = CertificateInvalidError{c, Expired}
		} else if len(never) > 0 {
			err = CertificateInvalidError{c, NeverValid}
		}
		return
	}

	if len(opts.DNSName) > 0 {
		err = c.VerifyHostname(opts.DNSName)
		if err != nil {
			return
		}
	}
	return
}

func appendToFreshChain(chain []*Certificate, cert *Certificate) []*Certificate {
	n := make([]*Certificate, len(chain)+1)
	copy(n, chain)
	n[len(chain)] = cert
	return n
}

// buildChains returns all chains of length < maxIntermediateCount. Chains begin
// the certificate being validated (chain[0] = c), and end at a root. It
// enforces that all intermediates can sign certificates, and checks signatures.
// It does not enforce expiration.
func (c *Certificate) buildChains(cache map[int][]CertificateChain, currentChain CertificateChain, opts *VerifyOptions) (chains []CertificateChain, err error) {

	// If the certificate being validated is a root, add the chain of length one
	// containing just the root. Only do this on the first call to buildChains,
	// when the len(currentChain) = 1.
	if len(currentChain) == 1 && opts.Roots.Contains(c) {
		chains = append(chains, CertificateChain{c})
	}

	if len(chains) == 0 && c.SelfSigned {
		err = CertificateInvalidError{c, IsSelfSigned}
	}

	// Find roots that signed c and have matching SKID/AKID and Subject/Issuer.
	possibleRoots, failedRoot, rootErr := opts.Roots.findVerifiedParents(c)

	// If any roots are parents of c, create new chain for each one of them.
	for _, rootNum := range possibleRoots {
		root := opts.Roots.certs[rootNum]
		err = root.isValid(CertificateTypeRoot, currentChain)
		if err != nil {
			continue
		}
		if !currentChain.CertificateInChain(root) {
			chains = append(chains, currentChain.AppendToFreshChain(root))
		}
	}

	// The root chains of length N+1 are now "done". Now we'll look for any
	// intermediates that issue this certificate, meaning that any chain to a root
	// through these intermediates is at least length N+2.
	possibleIntermediates, failedIntermediate, intermediateErr := opts.Intermediates.findVerifiedParents(c)

	for _, intermediateNum := range possibleIntermediates {
		intermediate := opts.Intermediates.certs[intermediateNum]
		if opts.Roots.Contains(intermediate) {
			continue
		}
		if currentChain.CertificateSubjectAndKeyInChain(intermediate) {
			continue
		}
		err = intermediate.isValid(CertificateTypeIntermediate, currentChain)
		if err != nil {
			continue
		}

		// We don't want to add any certificate to chains that doesn't somehow get
		// to a root. We don't know if all chains through the intermediates will end
		// at a root, so we slice off the back half of the chain and try to build
		// that part separately.
		childChains, ok := cache[intermediateNum]
		if !ok {
			childChains, err = intermediate.buildChains(cache, currentChain.AppendToFreshChain(intermediate), opts)
			cache[intermediateNum] = childChains
		}
		chains = append(chains, childChains...)
	}

	if len(chains) > 0 {
		err = nil
	}

	if len(chains) == 0 && err == nil {
		hintErr := rootErr
		hintCert := failedRoot
		if hintErr == nil {
			hintErr = intermediateErr
			hintCert = failedIntermediate
		}
		err = UnknownAuthorityError{c, hintErr, hintCert}
	}

	return
}

func matchHostnames(pattern, host string) bool {
	host = strings.TrimSuffix(host, ".")
	pattern = strings.TrimSuffix(pattern, ".")

	if len(pattern) == 0 || len(host) == 0 {
		return false
	}

	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")

	if len(patternParts) != len(hostParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if /*i == 0 &&*/ patternPart == "*" {
			continue
		}
		if patternPart != hostParts[i] {
			return false
		}
	}

	return true
}

// toLowerCaseASCII returns a lower-case version of in. See RFC 6125 6.4.1. We use
// an explicitly ASCII function to avoid any sharp corners resulting from
// performing Unicode operations on DNS labels.
func toLowerCaseASCII(in string) string {
	// If the string is already lower-case then there's nothing to do.
	isAlreadyLowerCase := true
	for _, c := range in {
		if c == utf8.RuneError {
			// If we get a UTF-8 error then there might be
			// upper-case ASCII bytes in the invalid sequence.
			isAlreadyLowerCase = false
			break
		}
		if 'A' <= c && c <= 'Z' {
			isAlreadyLowerCase = false
			break
		}
	}

	if isAlreadyLowerCase {
		return in
	}

	out := []byte(in)
	for i, c := range out {
		if 'A' <= c && c <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}

// VerifyHostname returns nil if c is a valid certificate for the named host.
// Otherwise it returns an error describing the mismatch.
func (c *Certificate) VerifyHostname(h string) error {
	// IP addresses may be written in [ ].
	candidateIP := h
	if len(h) >= 3 && h[0] == '[' && h[len(h)-1] == ']' {
		candidateIP = h[1 : len(h)-1]
	}
	if ip := net.ParseIP(candidateIP); ip != nil {
		// We only match IP addresses against IP SANs.
		// https://tools.ietf.org/html/rfc6125#appendix-B.2
		for _, candidate := range c.IPAddresses {
			if ip.Equal(candidate) {
				return nil
			}
		}
		return HostnameError{c, candidateIP}
	}

	lowered := toLowerCaseASCII(h)

	if c.hasSANExtension() {
		for _, match := range c.DNSNames {
			if matchHostnames(toLowerCaseASCII(match), lowered) {
				return nil
			}
		}
		// If Subject Alt Name is given, we ignore the common name.
	} else if matchHostnames(toLowerCaseASCII(c.Subject.CommonName), lowered) {
		return nil
	}

	return HostnameError{c, h}
}

func checkChainForKeyUsage(chain []*Certificate, keyUsages []ExtKeyUsage) bool {
	usages := make([]ExtKeyUsage, len(keyUsages))
	copy(usages, keyUsages)

	if len(chain) == 0 {
		return false
	}

	usagesRemaining := len(usages)

	// We walk down the list and cross out any usages that aren't supported
	// by each certificate. If we cross out all the usages, then the chain
	// is unacceptable.

NextCert:
	for i := len(chain) - 1; i >= 0; i-- {
		cert := chain[i]
		if len(cert.ExtKeyUsage) == 0 && len(cert.UnknownExtKeyUsage) == 0 {
			// The certificate doesn't have any extended key usage specified.
			continue
		}

		for _, usage := range cert.ExtKeyUsage {
			if usage == ExtKeyUsageAny {
				// The certificate is explicitly good for any usage.
				continue NextCert
			}
		}

		const invalidUsage ExtKeyUsage = -1

	NextRequestedUsage:
		for i, requestedUsage := range usages {
			if requestedUsage == invalidUsage {
				continue
			}

			for _, usage := range cert.ExtKeyUsage {
				if requestedUsage == usage {
					continue NextRequestedUsage
				} else if requestedUsage == ExtKeyUsageServerAuth &&
					(usage == ExtKeyUsageNetscapeServerGatedCrypto ||
						usage == ExtKeyUsageMicrosoftServerGatedCrypto) {
					// In order to support COMODO
					// certificate chains, we have to
					// accept Netscape or Microsoft SGC
					// usages as equal to ServerAuth.
					continue NextRequestedUsage
				}
			}

			usages[i] = invalidUsage
			usagesRemaining--
			if usagesRemaining == 0 {
				return false
			}
		}
	}

	return true
}

// earlier returns the earlier of a and b
func earlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// later returns the later of a and b
func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// check expirations divides chains into a set of disjoint chains, containing
// current chains valid now, expired chains that were valid at some point, and
// the set of chains that were never valid.
func FilterByDate(chains []CertificateChain, now time.Time) (current, expired, never []CertificateChain) {
	for _, chain := range chains {
		if len(chain) == 0 {
			continue
		}
		leaf := chain[0]
		lowerBound := leaf.NotBefore
		upperBound := leaf.NotAfter
		for _, c := range chain[1:] {
			lowerBound = later(lowerBound, c.NotBefore)
			upperBound = earlier(upperBound, c.NotAfter)
		}
		valid := lowerBound.Before(now) && upperBound.After(now)
		wasValid := lowerBound.Before(upperBound)
		if valid && !wasValid {
			// Math/logic tells us this is impossible.
			panic("valid && !wasValid should not be possible")
		}
		if valid {
			current = append(current, chain)
		} else if wasValid {
			expired = append(expired, chain)
		} else {
			never = append(never, chain)
		}
	}
	return
}
