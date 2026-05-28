package in_toto

import (
	"crypto/x509"
	"fmt"
	"net/url"
)

const (
	AllowAllConstraint = "*"
)

// CertificateConstraint defines the attributes a certificate must have to act as a functionary.
// A wildcard `*` allows any value in the specified attribute, where as an empty array or value
// asserts that the certificate must have nothing for that attribute. A certificate must have
// every value defined in a constraint to match.
type CertificateConstraint struct {
	CommonName    string   `json:"common_name"`
	DNSNames      []string `json:"dns_names"`
	Emails        []string `json:"emails"`
	Organizations []string `json:"organizations"`
	Roots         []string `json:"roots"`
	URIs          []string `json:"uris"`
}

// checkResult is a data structure used to hold
// certificate constraint errors
type checkResult struct {
	errors []error
}

// newCheckResult initializes a new checkResult
func newCheckResult() *checkResult {
	return &checkResult{
		errors: make([]error, 0),
	}
}

// evaluate runs a constraint check on a certificate
func (cr *checkResult) evaluate(cert *x509.Certificate, constraintCheck func(*x509.Certificate) error) *checkResult {
	err := constraintCheck(cert)
	if err != nil {
		cr.errors = append(cr.errors, err)
	}
	return cr
}

// error reduces all of the errors into one error with a
// combined error message. If there are no errors, nil
// will be returned.
func (cr *checkResult) error() error {
	if len(cr.errors) == 0 {
		return nil
	}
	return fmt.Errorf("cert failed constraints check: %+q", cr.errors)
}

// Check tests the provided certificate against the constraint. An error is returned if the certificate
// fails any of the constraints. nil is returned if the certificate passes all of the constraints.
func (cc CertificateConstraint) Check(cert *x509.Certificate, rootCAIDs []string, rootCertPool, intermediateCertPool *x509.CertPool) error {
	return newCheckResult().
		evaluate(cert, cc.checkCommonName).
		evaluate(cert, cc.checkDNSNames).
		evaluate(cert, cc.checkEmails).
		evaluate(cert, cc.checkOrganizations).
		evaluate(cert, cc.checkRoots(rootCAIDs, rootCertPool, intermediateCertPool)).
		evaluate(cert, cc.checkURIs).
		error()
}

// checkCommonName verifies that the certificate's common name matches the constraint.
func (cc CertificateConstraint) checkCommonName(cert *x509.Certificate) error {
	return checkCertConstraint("common name", []string{cc.CommonName}, []string{cert.Subject.CommonName})
}

// checkDNSNames verifies that the certificate's dns names matches the constraint.
func (cc CertificateConstraint) checkDNSNames(cert *x509.Certificate) error {
	return checkCertConstraint("dns name", cc.DNSNames, cert.DNSNames)
}

// checkEmails verifies that the certificate's emails matches the constraint.
func (cc CertificateConstraint) checkEmails(cert *x509.Certificate) error {
	return checkCertConstraint("email", cc.Emails, cert.EmailAddresses)
}

// checkOrganizations verifies that the certificate's organizations matches the constraint.
func (cc CertificateConstraint) checkOrganizations(cert *x509.Certificate) error {
	return checkCertConstraint("organization", cc.Organizations, cert.Subject.Organization)
}

// checkRoots verifies that the certificate's roots matches the constraint.
// The certificates trust chain must also be verified.
func (cc CertificateConstraint) checkRoots(rootCAIDs []string, rootCertPool, intermediateCertPool *x509.CertPool) func(*x509.Certificate) error {
	return func(cert *x509.Certificate) error {
		_, err := VerifyCertificateTrust(cert, rootCertPool, intermediateCertPool)
		if err != nil {
			return fmt.Errorf("failed to verify roots: %w", err)
		}
		return checkCertConstraint("root", cc.Roots, rootCAIDs)
	}
}

// checkURIs verifies that the certificate's URIs matches the constraint.
func (cc CertificateConstraint) checkURIs(cert *x509.Certificate) error {
	return checkCertConstraint("uri", cc.URIs, urisToStrings(cert.URIs))
}

// urisToStrings is a helper that converts a list of URL objects to the string that represents them
func urisToStrings(uris []*url.URL) []string {
	res := make([]string, 0, len(uris))
	for _, uri := range uris {
		res = append(res, uri.String())
	}

	return res
}

// checkCertConstraint tests that the provided test values match the allowed values of the constraint.
// All allowed values must be met one-to-one to be considered a successful match.
func checkCertConstraint(attributeName string, constraints, values []string) error {
	// If the only constraint is to allow all, the check succeeds
	if len(constraints) == 1 && constraints[0] == AllowAllConstraint {
		return nil
	}

	if len(constraints) == 1 && constraints[0] == "" {
		constraints = []string{}
	}

	if len(values) == 1 && values[0] == "" {
		values = []string{}
	}

	// If no constraints are specified, but the certificate has values for the attribute, then the check fails
	if len(constraints) == 0 && len(values) > 0 {
		return fmt.Errorf("not expecting any %s(s), but cert has %d %s(s)", attributeName, len(values), attributeName)
	}

	unmet := NewSet(constraints...)
	for _, v := range values {
		// if the cert has a value we didn't expect, fail early
		if !unmet.Has(v) {
			return fmt.Errorf("cert has an unexpected %s %s given constraints %+q", attributeName, v, constraints)
		}

		// consider the constraint met
		unmet.Remove(v)
	}

	// if we have any unmet left after going through each test value, fail.
	if len(unmet) > 0 {
		return fmt.Errorf("cert with %s(s) %+q did not pass all constraints %+q", attributeName, values, constraints)
	}

	return nil
}
