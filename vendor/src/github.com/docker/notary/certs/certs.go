package certs

import (
	"crypto/x509"
	"errors"
	"fmt"
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

/*
ValidateRoot receives a new root, validates its correctness and attempts to
do root key rotation if needed.

First we list the current trusted certificates we have for a particular GUN. If
that list is non-empty means that we've already seen this repository before, and
have a list of trusted certificates for it. In this case, we use this list of
certificates to attempt to validate this root file.

If the previous validation succeeds, or in the case where we found no trusted
certificates for this particular GUN, we check the integrity of the root by
making sure that it is validated by itself. This means that we will attempt to
validate the root data with the certificates that are included in the root keys
themselves.

If this last steps succeeds, we attempt to do root rotation, by ensuring that
we only trust the certificates that are present in the new root.

This mechanism of operation is essentially Trust On First Use (TOFU): if we
have never seen a certificate for a particular CN, we trust it. If later we see
a different certificate for that certificate, we return an ErrValidationFailed error.

Note that since we only allow trust data to be downloaded over an HTTPS channel
we are using the current public PKI to validate the first download of the certificate
adding an extra layer of security over the normal (SSH style) trust model.
We shall call this: TOFUS.
*/
func ValidateRoot(certStore trustmanager.X509Store, root *data.Signed, gun string) error {
	logrus.Debugf("entered ValidateRoot with dns: %s", gun)
	signedRoot, err := data.RootFromSigned(root)
	if err != nil {
		return err
	}

	// Retrieve all the leaf certificates in root for which the CN matches the GUN
	allValidCerts, err := validRootLeafCerts(signedRoot, gun)
	if err != nil {
		logrus.Debugf("error retrieving valid leaf certificates for: %s, %v", gun, err)
		return &ErrValidationFail{Reason: "unable to retrieve valid leaf certificates"}
	}

	// Retrieve all the trusted certificates that match this gun
	certsForCN, err := certStore.GetCertificatesByCN(gun)
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
	if len(certsForCN) != 0 {
		logrus.Debugf("found %d valid root certificates for %s", len(certsForCN), gun)
		err = signed.VerifyRoot(root, 0, trustmanager.CertsToKeys(certsForCN))
		if err != nil {
			logrus.Debugf("failed to verify TUF data for: %s, %v", gun, err)
			return &ErrValidationFail{Reason: "failed to validate data with current trusted certificates"}
		}
	} else {
		logrus.Debugf("found no currently valid root certificates for %s", gun)
	}

	// Validate the integrity of the new root (does it have valid signatures)
	err = signed.VerifyRoot(root, 0, trustmanager.CertsToKeys(allValidCerts))
	if err != nil {
		logrus.Debugf("failed to verify TUF data for: %s, %v", gun, err)
		return &ErrValidationFail{Reason: "failed to validate integrity of roots"}
	}

	// Getting here means A) we had trusted certificates and both the
	// old and new validated this root; or B) we had no trusted certificates but
	// the new set of certificates has integrity (self-signed)
	logrus.Debugf("entering root certificate rotation for: %s", gun)

	// Do root certificate rotation: we trust only the certs present in the new root
	// First we add all the new certificates (even if they already exist)
	for _, cert := range allValidCerts {
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
	for certID, cert := range certsToRemove(certsForCN, allValidCerts) {
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

// validRootLeafCerts returns a list of non-exipired, non-sha1 certificates whose
// Common-Names match the provided GUN
func validRootLeafCerts(root *data.SignedRoot, gun string) ([]*x509.Certificate, error) {
	// Get a list of all of the leaf certificates present in root
	allLeafCerts, _ := parseAllCerts(root)
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

	logrus.Debugf("found %d valid leaf certificates for %s", len(validLeafCerts), gun)
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

// certsToRemove returns all the certifificates from oldCerts that aren't present
// in newCerts
func certsToRemove(oldCerts, newCerts []*x509.Certificate) map[string]*x509.Certificate {
	certsToRemove := make(map[string]*x509.Certificate)

	// If no newCerts were provided
	if len(newCerts) == 0 {
		return certsToRemove
	}

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

	return certsToRemove
}
