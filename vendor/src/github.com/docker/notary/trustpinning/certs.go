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

func prettyFormatCertIDs(certs map[string]*x509.Certificate) string {
	ids := make([]string, 0, len(certs))
	for id := range certs {
		ids = append(ids, id)
	}
	return strings.Join(ids, ", ")
}

/*
ValidateRoot receives a new root, validates its correctness and attempts to
do root key rotation if needed.

First we check if we have any trusted certificates for a particular GUN in
a previous root, if we have one. If the previous root is not nil and we find
certificates for this GUN, we've already seen this repository before, and
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
func ValidateRoot(prevRoot *data.SignedRoot, root *data.Signed, gun string, trustPinning TrustPinConfig) (*data.SignedRoot, error) {
	logrus.Debugf("entered ValidateRoot with dns: %s", gun)
	signedRoot, err := data.RootFromSigned(root)
	if err != nil {
		return nil, err
	}

	rootRole, err := signedRoot.BuildBaseRole(data.CanonicalRootRole)
	if err != nil {
		return nil, err
	}

	// Retrieve all the leaf and intermediate certificates in root for which the CN matches the GUN
	allLeafCerts, allIntCerts := parseAllCerts(signedRoot)
	certsFromRoot, err := validRootLeafCerts(allLeafCerts, gun, true)

	if err != nil {
		logrus.Debugf("error retrieving valid leaf certificates for: %s, %v", gun, err)
		return nil, &ErrValidationFail{Reason: "unable to retrieve valid leaf certificates"}
	}

	// If we have a previous root, let's try to use it to validate that this new root is valid.
	if prevRoot != nil {
		// Retrieve all the trusted certificates from our previous root
		// Note that we do not validate expiries here since our originally trusted root might have expired certs
		allTrustedLeafCerts, allTrustedIntCerts := parseAllCerts(prevRoot)
		trustedLeafCerts, err := validRootLeafCerts(allTrustedLeafCerts, gun, false)

		// Use the certificates we found in the previous root for the GUN to verify its signatures
		// This could potentially be an empty set, in which case we will fail to verify
		logrus.Debugf("found %d valid root leaf certificates for %s: %s", len(trustedLeafCerts), gun,
			prettyFormatCertIDs(trustedLeafCerts))

		// Extract the previous root's threshold for signature verification
		prevRootRoleData, ok := prevRoot.Signed.Roles[data.CanonicalRootRole]
		if !ok {
			return nil, &ErrValidationFail{Reason: "could not retrieve previous root role data"}
		}

		err = signed.VerifySignatures(
			root, data.BaseRole{Keys: trustmanager.CertsToKeys(trustedLeafCerts, allTrustedIntCerts), Threshold: prevRootRoleData.Threshold})
		if err != nil {
			logrus.Debugf("failed to verify TUF data for: %s, %v", gun, err)
			return nil, &ErrRootRotationFail{Reason: "failed to validate data with current trusted certificates"}
		}
	} else {
		logrus.Debugf("found no currently valid root certificates for %s, using trust_pinning config to bootstrap trust", gun)
		trustPinCheckFunc, err := NewTrustPinChecker(trustPinning, gun)
		if err != nil {
			return nil, &ErrValidationFail{Reason: err.Error()}
		}

		validPinnedCerts := map[string]*x509.Certificate{}
		for id, cert := range certsFromRoot {
			if ok := trustPinCheckFunc(cert, allIntCerts[id]); !ok {
				continue
			}
			validPinnedCerts[id] = cert
		}
		if len(validPinnedCerts) == 0 {
			return nil, &ErrValidationFail{Reason: "unable to match any certificates to trust_pinning config"}
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
		return nil, &ErrValidationFail{Reason: "failed to validate integrity of roots"}
	}

	logrus.Debugf("Root validation succeeded for %s", gun)
	return signedRoot, nil
}

// validRootLeafCerts returns a list of possibly (if checkExpiry is true) non-expired, non-sha1 certificates
// found in root whose Common-Names match the provided GUN. Note that this
// "validity" alone does not imply any measure of trust.
func validRootLeafCerts(allLeafCerts map[string]*x509.Certificate, gun string, checkExpiry bool) (map[string]*x509.Certificate, error) {
	validLeafCerts := make(map[string]*x509.Certificate)

	// Go through every leaf certificate and check that the CN matches the gun
	for id, cert := range allLeafCerts {
		// Validate that this leaf certificate has a CN that matches the exact gun
		if cert.Subject.CommonName != gun {
			logrus.Debugf("error leaf certificate CN: %s doesn't match the given GUN: %s",
				cert.Subject.CommonName, gun)
			continue
		}
		// Make sure the certificate is not expired if checkExpiry is true
		if checkExpiry && time.Now().After(cert.NotAfter) {
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

		validLeafCerts[id] = cert
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
	if signedRoot == nil {
		return nil, nil
	}

	leafCerts := make(map[string]*x509.Certificate)
	intCerts := make(map[string][]*x509.Certificate)

	// Before we loop through all root keys available, make sure any exist
	rootRoles, ok := signedRoot.Signed.Roles[data.CanonicalRootRole]
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

		// Store the leaf cert in the map
		leafCerts[key.ID()] = leafCert

		// Get all the remainder certificates marked as a CA to be used as intermediates
		intermediateCerts := trustmanager.GetIntermediateCerts(decodedCerts)
		intCerts[key.ID()] = intermediateCerts
	}

	return leafCerts, intCerts
}
