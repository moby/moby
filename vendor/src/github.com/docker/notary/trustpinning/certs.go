package trustpinning

import (
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/signed"
)

// ErrValidationFail is returned when there is no valid trusted certificates
// being served inside of the roots.json
type ErrValidationFail struct {
	Reason string
}

// ErrValidationFail is returned when there is no valid trusted certificates
// being served inside of the roots.json
func (err ErrValidationFail) Error() string {
	return fmt.Sprintf("could not validate the path to a trusted root: %s", err.Reason)
}

// ErrRootRotationFail is returned when we fail to do a full root key rotation
// by either failing to add the new root certificate, or delete the old ones
type ErrRootRotationFail struct {
	Reason string
}

// ErrRootRotationFail is returned when we fail to do a full root key rotation
// by either failing to add the new root certificate, or delete the old ones
func (err ErrRootRotationFail) Error() string {
	return fmt.Sprintf("could not rotate trust to a new trusted root: %s", err.Reason)
}

func prettyFormatCertIDs(certs []*x509.Certificate) string {
	ids := make([]string, 0, len(certs))
	for _, cert := range certs {
		id, err := trustmanager.FingerprintCert(cert)
		if err != nil {
			id = fmt.Sprintf("[Error %s]", err)
		}
		ids = append(ids, id)
	}
	return strings.Join(ids, ", ")
}

/*
ValidateRoot receives a new root, validates its correctness and attempts to
do root key rotation if needed.

First we list the current trusted certificates we have for a particular GUN. If
that list is non-empty means that we've already seen this repository before, and
have a list of trusted certificates for it. In this case, we use this list of
certificates to attempt to validate this root file.

If the previous validation succeeds, we check the integrity of the root by
making sure that it is validated by itself. This means that we will attempt to
validate the root data with the certificates that are included in the root keys
themselves.

However, if we do not have any current trusted certificates for this GUN, we
check if there are any pinned certificates specified in the trust_pinning section
of the notary client config.  If this section specifies a Certs section with this
GUN, we attempt to validate that the certificates present in the downloaded root
file match the pinned ID.

If the Certs section is empty for this GUN, we check if the trust_pinning
section specifies a CA section specified in the config for this GUN.  If so, we check
that the specified CA is valid and has signed a certificate included in the downloaded
root file.  The specified CA can be a prefix for this GUN.

If both the Certs and CA configs do not match this GUN, we fall back to the TOFU
section in the config: if true, we trust certificates specified in the root for
this GUN. If later we see a different certificate for that certificate, we return
an ErrValidationFailed error.

Note that since we only allow trust data to be downloaded over an HTTPS channel
we are using the current public PKI to validate the first download of the certificate
adding an extra layer of security over the normal (SSH style) trust model.
We shall call this: TOFUS.

Validation failure at any step will result in an ErrValidationFailed error.
*/
func ValidateRoot(certStore trustmanager.X509Store, root *data.Signed, gun string, trustPinning TrustPinConfig) error {
	logrus.Debugf("entered ValidateRoot with dns: %s", gun)
	signedRoot, err := data.RootFromSigned(root)
	if err != nil {
		return err
	}

	rootRole, err := signedRoot.BuildBaseRole(data.CanonicalRootRole)
	if err != nil {
		return err
	}

	// Retrieve all the leaf and intermediate certificates in root for which the CN matches the GUN
	allLeafCerts, allIntCerts := parseAllCerts(signedRoot)
	certsFromRoot, err := validRootLeafCerts(allLeafCerts, gun)
	if err != nil {
		logrus.Debugf("error retrieving valid leaf certificates for: %s, %v", gun, err)
		return &ErrValidationFail{Reason: "unable to retrieve valid leaf certificates"}
	}

	// Retrieve all the trusted certificates that match this gun
	trustedCerts, err := certStore.GetCertificatesByCN(gun)
	if err != nil {
		// If the error that we get back is different than ErrNoCertificatesFound
		// we couldn't check if there are any certificates with this CN already
		// trusted. Let's take the conservative approach and return a failed validation
		if _, ok := err.(*trustmanager.ErrNoCertificatesFound); !ok {
			logrus.Debugf("error retrieving trusted certificates for: %s, %v", gun, err)
			return &ErrValidationFail{Reason: "unable to retrieve trusted certificates"}
		}
	}
	// If we have certificates that match this specific GUN, let's make sure to
	// use them first to validate that this new root is valid.
	if len(trustedCerts) != 0 {
		logrus.Debugf("found %d valid root certificates for %s: %s", len(trustedCerts), gun,
			prettyFormatCertIDs(trustedCerts))
		err = signed.VerifySignatures(
			root, data.BaseRole{Keys: trustmanager.CertsToKeys(trustedCerts, allIntCerts), Threshold: 1})
		if err != nil {
			logrus.Debugf("failed to verify TUF data for: %s, %v", gun, err)
			return &ErrValidationFail{Reason: "failed to validate data with current trusted certificates"}
		}
	} else {
		logrus.Debugf("found no currently valid root certificates for %s, using trust_pinning config to bootstrap trust", gun)
		trustPinCheckFunc, err := NewTrustPinChecker(trustPinning, gun)
		if err != nil {
			return &ErrValidationFail{Reason: err.Error()}
		}

		validPinnedCerts := []*x509.Certificate{}
		for _, cert := range certsFromRoot {
			certID, err := trustmanager.FingerprintCert(cert)
			if err != nil {
				continue
			}
			if ok := trustPinCheckFunc(cert, allIntCerts[certID]); !ok {
				continue
			}
			validPinnedCerts = append(validPinnedCerts, cert)
		}
		if len(validPinnedCerts) == 0 {
			return &ErrValidationFail{Reason: "unable to match any certificates to trust_pinning config"}
		}
		certsFromRoot = validPinnedCerts
	}

	// Validate the integrity of the new root (does it have valid signatures)
	// Note that certsFromRoot is guaranteed to be unchanged only if we had prior cert data for this GUN or enabled TOFUS
	// If we attempted to pin a certain certificate or CA, certsFromRoot could have been pruned accordingly
	err = signed.VerifySignatures(root, data.BaseRole{
		Keys: trustmanager.CertsToKeys(certsFromRoot, allIntCerts), Threshold: rootRole.Threshold})
	if err != nil {
		logrus.Debugf("failed to verify TUF data for: %s, %v", gun, err)
		return &ErrValidationFail{Reason: "failed to validate integrity of roots"}
	}

	// Getting here means:
	// A) we had trusted certificates and both the old and new validated this root.
	// or
	// B) we had no trusted certificates but the new set of certificates has integrity (self-signed).
	logrus.Debugf("entering root certificate rotation for: %s", gun)

	// Do root certificate rotation: we trust only the certs present in the new root
	// First we add all the new certificates (even if they already exist)
	for _, cert := range certsFromRoot {
		err := certStore.AddCert(cert)
		if err != nil {
			// If the error is already exists we don't fail the rotation
			if _, ok := err.(*trustmanager.ErrCertExists); ok {
				logrus.Debugf("ignoring certificate addition to: %s", gun)
				continue
			}
			logrus.Debugf("error adding new trusted certificate for: %s, %v", gun, err)
		}
	}

	// Now we delete old certificates that aren't present in the new root
	oldCertsToRemove, err := certsToRemove(trustedCerts, certsFromRoot)
	if err != nil {
		logrus.Debugf("inconsistency when removing old certificates: %v", err)
		return err
	}
	for certID, cert := range oldCertsToRemove {
		logrus.Debugf("removing certificate with certID: %s", certID)
		err = certStore.RemoveCert(cert)
		if err != nil {
			logrus.Debugf("failed to remove trusted certificate with keyID: %s, %v", certID, err)
			return &ErrRootRotationFail{Reason: "failed to rotate root keys"}
		}
	}

	logrus.Debugf("Root validation succeeded for %s", gun)
	return nil
}

// validRootLeafCerts returns a list of non-expired, non-sha1 certificates
// found in root whose Common-Names match the provided GUN. Note that this
// "validity" alone does not imply any measure of trust.
func validRootLeafCerts(allLeafCerts map[string]*x509.Certificate, gun string) ([]*x509.Certificate, error) {
	var validLeafCerts []*x509.Certificate

	// Go through every leaf certificate and check that the CN matches the gun
	for _, cert := range allLeafCerts {
		// Validate that this leaf certificate has a CN that matches the exact gun
		if cert.Subject.CommonName != gun {
			logrus.Debugf("error leaf certificate CN: %s doesn't match the given GUN: %s",
				cert.Subject.CommonName, gun)
			continue
		}
		// Make sure the certificate is not expired
		if time.Now().After(cert.NotAfter) {
			logrus.Debugf("error leaf certificate is expired")
			continue
		}

		// We don't allow root certificates that use SHA1
		if cert.SignatureAlgorithm == x509.SHA1WithRSA ||
			cert.SignatureAlgorithm == x509.DSAWithSHA1 ||
			cert.SignatureAlgorithm == x509.ECDSAWithSHA1 {

			logrus.Debugf("error certificate uses deprecated hashing algorithm (SHA1)")
			continue
		}

		validLeafCerts = append(validLeafCerts, cert)
	}

	if len(validLeafCerts) < 1 {
		logrus.Debugf("didn't find any valid leaf certificates for %s", gun)
		return nil, errors.New("no valid leaf certificates found in any of the root keys")
	}

	logrus.Debugf("found %d valid leaf certificates for %s: %s", len(validLeafCerts), gun,
		prettyFormatCertIDs(validLeafCerts))
	return validLeafCerts, nil
}

// parseAllCerts returns two maps, one with all of the leafCertificates and one
// with all the intermediate certificates found in signedRoot
func parseAllCerts(signedRoot *data.SignedRoot) (map[string]*x509.Certificate, map[string][]*x509.Certificate) {
	leafCerts := make(map[string]*x509.Certificate)
	intCerts := make(map[string][]*x509.Certificate)

	// Before we loop through all root keys available, make sure any exist
	rootRoles, ok := signedRoot.Signed.Roles["root"]
	if !ok {
		logrus.Debugf("tried to parse certificates from invalid root signed data")
		return nil, nil
	}

	logrus.Debugf("found the following root keys: %v", rootRoles.KeyIDs)
	// Iterate over every keyID for the root role inside of roots.json
	for _, keyID := range rootRoles.KeyIDs {
		// check that the key exists in the signed root keys map
		key, ok := signedRoot.Signed.Keys[keyID]
		if !ok {
			logrus.Debugf("error while getting data for keyID: %s", keyID)
			continue
		}

		// Decode all the x509 certificates that were bundled with this
		// Specific root key
		decodedCerts, err := trustmanager.LoadCertBundleFromPEM(key.Public())
		if err != nil {
			logrus.Debugf("error while parsing root certificate with keyID: %s, %v", keyID, err)
			continue
		}

		// Get all non-CA certificates in the decoded certificates
		leafCertList := trustmanager.GetLeafCerts(decodedCerts)

		// If we got no leaf certificates or we got more than one, fail
		if len(leafCertList) != 1 {
			logrus.Debugf("invalid chain due to leaf certificate missing or too many leaf certificates for keyID: %s", keyID)
			continue
		}
		// If we found a leaf certificate, assert that the cert bundle started with a leaf
		if decodedCerts[0].IsCA {
			logrus.Debugf("invalid chain due to leaf certificate not being first certificate for keyID: %s", keyID)
			continue
		}

		// Get the ID of the leaf certificate
		leafCert := leafCertList[0]
		leafID, err := trustmanager.FingerprintCert(leafCert)
		if err != nil {
			logrus.Debugf("error while fingerprinting root certificate with keyID: %s, %v", keyID, err)
			continue
		}

		// Store the leaf cert in the map
		leafCerts[leafID] = leafCert

		// Get all the remainder certificates marked as a CA to be used as intermediates
		intermediateCerts := trustmanager.GetIntermediateCerts(decodedCerts)
		intCerts[leafID] = intermediateCerts
	}

	return leafCerts, intCerts
}

// certsToRemove returns all the certificates from oldCerts that aren't present
// in newCerts.  Note that newCerts should never be empty, else this function will error.
// We expect newCerts to come from validateRootLeafCerts, which does not return empty sets.
func certsToRemove(oldCerts, newCerts []*x509.Certificate) (map[string]*x509.Certificate, error) {
	certsToRemove := make(map[string]*x509.Certificate)

	// Populate a map with all the IDs from newCert
	var newCertMap = make(map[string]struct{})
	for _, cert := range newCerts {
		certID, err := trustmanager.FingerprintCert(cert)
		if err != nil {
			logrus.Debugf("error while fingerprinting root certificate with keyID: %s, %v", certID, err)
			continue
		}
		newCertMap[certID] = struct{}{}
	}

	// We don't want to "rotate" certificates to an empty set, nor keep old certificates if the
	// new root does not trust them.  newCerts should come from validRootLeafCerts, which refuses
	// to return an empty set, and they should all be fingerprintable, so this should never happen
	// - fail just to be sure.
	if len(newCertMap) == 0 {
		return nil, &ErrRootRotationFail{Reason: "internal error, got no certificates to rotate to"}
	}

	// Iterate over all the old certificates and check to see if we should remove them
	for _, cert := range oldCerts {
		certID, err := trustmanager.FingerprintCert(cert)
		if err != nil {
			logrus.Debugf("error while fingerprinting root certificate with certID: %s, %v", certID, err)
			continue
		}
		if _, ok := newCertMap[certID]; !ok {
			certsToRemove[certID] = cert
		}
	}

	return certsToRemove, nil
}
