package trustpinning

import (
	"crypto/x509"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/docker/notary/tuf/utils"
	"strings"
)

// TrustPinConfig represents the configuration under the trust_pinning section of the config file
// This struct represents the preferred way to bootstrap trust for this repository
type TrustPinConfig struct {
	CA          map[string]string
	Certs       map[string][]string
	DisableTOFU bool
}

type trustPinChecker struct {
	gun           string
	config        TrustPinConfig
	pinnedCAPool  *x509.CertPool
	pinnedCertIDs []string
}

// CertChecker is a function type that will be used to check leaf certs against pinned trust
type CertChecker func(leafCert *x509.Certificate, intCerts []*x509.Certificate) bool

// NewTrustPinChecker returns a new certChecker function from a TrustPinConfig for a GUN
func NewTrustPinChecker(trustPinConfig TrustPinConfig, gun string, firstBootstrap bool) (CertChecker, error) {
	t := trustPinChecker{gun: gun, config: trustPinConfig}
	// Determine the mode, and if it's even valid
	if pinnedCerts, ok := trustPinConfig.Certs[gun]; ok {
		logrus.Debugf("trust-pinning using Cert IDs")
		t.pinnedCertIDs = pinnedCerts
		return t.certsCheck, nil
	}

	if caFilepath, err := getPinnedCAFilepathByPrefix(gun, trustPinConfig); err == nil {
		logrus.Debugf("trust-pinning using root CA bundle at: %s", caFilepath)

		// Try to add the CA certs from its bundle file to our certificate store,
		// and use it to validate certs in the root.json later
		caCerts, err := utils.LoadCertBundleFromFile(caFilepath)
		if err != nil {
			return nil, fmt.Errorf("could not load root cert from CA path")
		}
		// Now only consider certificates that are direct children from this CA cert chain
		caRootPool := x509.NewCertPool()
		for _, caCert := range caCerts {
			if err = utils.ValidateCertificate(caCert, true); err != nil {
				logrus.Debugf("ignoring root CA certificate with CN %s in bundle: %s", caCert.Subject.CommonName, err)
				continue
			}
			caRootPool.AddCert(caCert)
		}
		// If we didn't have any valid CA certs, error out
		if len(caRootPool.Subjects()) == 0 {
			return nil, fmt.Errorf("invalid CA certs provided")
		}
		t.pinnedCAPool = caRootPool
		return t.caCheck, nil
	}

	// If TOFUs is disabled and we don't have any previous trusted root data for this GUN, we error out
	if trustPinConfig.DisableTOFU && firstBootstrap {
		return nil, fmt.Errorf("invalid trust pinning specified")

	}
	return t.tofusCheck, nil
}

func (t trustPinChecker) certsCheck(leafCert *x509.Certificate, intCerts []*x509.Certificate) bool {
	// reconstruct the leaf + intermediate cert chain, which is bundled as {leaf, intermediates...},
	// in order to get the matching id in the root file
	key, err := utils.CertBundleToKey(leafCert, intCerts)
	if err != nil {
		logrus.Debug("error creating cert bundle: ", err.Error())
		return false
	}
	return utils.StrSliceContains(t.pinnedCertIDs, key.ID())
}

func (t trustPinChecker) caCheck(leafCert *x509.Certificate, intCerts []*x509.Certificate) bool {
	// Use intermediate certificates included in the root TUF metadata for our validation
	caIntPool := x509.NewCertPool()
	for _, intCert := range intCerts {
		caIntPool.AddCert(intCert)
	}
	// Attempt to find a valid certificate chain from the leaf cert to CA root
	// Use this certificate if such a valid chain exists (possibly using intermediates)
	var err error
	if _, err = leafCert.Verify(x509.VerifyOptions{Roots: t.pinnedCAPool, Intermediates: caIntPool}); err == nil {
		return true
	}
	logrus.Debugf("unable to find a valid certificate chain from leaf cert to CA root: %s", err)
	return false
}

func (t trustPinChecker) tofusCheck(leafCert *x509.Certificate, intCerts []*x509.Certificate) bool {
	return true
}

// Will return the CA filepath corresponding to the most specific (longest) entry in the map that is still a prefix
// of the provided gun.  Returns an error if no entry matches this GUN as a prefix.
func getPinnedCAFilepathByPrefix(gun string, t TrustPinConfig) (string, error) {
	specificGUN := ""
	specificCAFilepath := ""
	foundCA := false
	for gunPrefix, caFilepath := range t.CA {
		if strings.HasPrefix(gun, gunPrefix) && len(gunPrefix) >= len(specificGUN) {
			specificGUN = gunPrefix
			specificCAFilepath = caFilepath
			foundCA = true
		}
	}
	if !foundCA {
		return "", fmt.Errorf("could not find pinned CA for GUN: %s\n", gun)
	}
	return specificCAFilepath, nil
}
