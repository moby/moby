package controlapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/log"
)

var minRootExpiration = 1 * helpers.OneYear

// determines whether an api.RootCA, api.RootRotation, or api.CAConfig has a signing key (local signer)
func hasSigningKey(a interface{}) bool {
	switch b := a.(type) {
	case api.RootCA:
		return len(b.CAKey) > 0
	case *api.RootRotation:
		return b != nil && len(b.CAKey) > 0
	case api.CAConfig:
		return len(b.SigningCACert) > 0 && len(b.SigningCAKey) > 0
	default:
		panic("needsExternalCAs should be called something of type api.RootCA, *api.RootRotation, or api.CAConfig")
	}
}

// Creates a cross-signed intermediate and new api.RootRotation object.
// This function assumes that the root cert and key and the external CAs have already been validated.
func newRootRotationObject(ctx context.Context, securityConfig *ca.SecurityConfig, cluster *api.Cluster, newRootCA ca.RootCA, version uint64) (*api.RootCA, error) {
	var (
		rootCert, rootKey, crossSignedCert []byte
		newRootHasSigner                   bool
		err                                error
	)

	rootCert = newRootCA.Certs
	if s, err := newRootCA.Signer(); err == nil {
		rootCert, rootKey = s.Cert, s.Key
		newRootHasSigner = true
	}

	// we have to sign with the original signer, not whatever is in the SecurityConfig's RootCA (which may have an intermediate signer, if
	// a root rotation is already in progress)
	switch {
	case hasSigningKey(cluster.RootCA):
		var oldRootCA ca.RootCA
		oldRootCA, err = ca.NewRootCA(cluster.RootCA.CACert, cluster.RootCA.CACert, cluster.RootCA.CAKey, ca.DefaultNodeCertExpiration, nil)
		if err == nil {
			crossSignedCert, err = oldRootCA.CrossSignCACertificate(rootCert)
		}
	case !newRootHasSigner: // the original CA and the new CA both require external CAs
		return nil, grpc.Errorf(codes.InvalidArgument, "rotating from one external CA to a different external CA is not supported")
	default:
		// We need the same credentials but to connect to the original URLs (in case we are in the middle of a root rotation already)
		externalCA := securityConfig.ExternalCA().Copy()
		var urls []string
		for _, c := range cluster.Spec.CAConfig.ExternalCAs {
			if c.Protocol == api.ExternalCA_CAProtocolCFSSL && bytes.Equal(c.CACert, cluster.RootCA.CACert) {
				urls = append(urls, c.URL)
			}
		}
		if len(urls) == 0 {
			return nil, grpc.Errorf(codes.InvalidArgument,
				"must provide an external CA for the current external root CA to generate a cross-signed certificate")
		}
		externalCA.UpdateURLs(urls...)
		crossSignedCert, err = externalCA.CrossSignRootCA(ctx, newRootCA)
	}

	if err != nil {
		log.G(ctx).WithError(err).Error("unable to generate a cross-signed certificate for root rotation")
		return nil, grpc.Errorf(codes.Internal, "unable to generate a cross-signed certificate for root rotation")
	}

	copied := cluster.RootCA.Copy()
	copied.RootRotation = &api.RootRotation{
		CACert:            rootCert,
		CAKey:             rootKey,
		CrossSignedCACert: crossSignedCert,
	}
	copied.LastForcedRotation = version
	return copied, nil
}

// Checks that a CA URL is connectable using the credentials we have and that its server certificate is signed by the
// root CA that we expect.  This uses a TCP dialer rather than an HTTP client; because we have custom TLS configuration,
// if we wanted to use an HTTP client we'd have to create a new transport for every connection.  The docs specify that
// Transports cache connections for future re-use, which could cause many open connections.
func validateExternalCAURL(dialer *net.Dialer, tlsOpts *tls.Config, caURL string) error {
	parsed, err := url.Parse(caURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return errors.New("invalid HTTP scheme")
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		// It either has no port or is otherwise invalid (e.g. too many colons).  If it's otherwise invalid the dialer
		// will error later, so just assume it's no port and set the port to the default HTTPS port.
		host = parsed.Host
		port = "443"
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, port), tlsOpts)
	if conn != nil {
		conn.Close()
	}
	return err
}

// Iterates over all the external CAs, and validates that there is at least 1 reachable, valid external CA for the
// given CA certificate.  Returns true if there is, false otherwise.
func hasAtLeastOneExternalCA(ctx context.Context, externalCAs []*api.ExternalCA, securityConfig *ca.SecurityConfig, wantedCert []byte) bool {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(wantedCert)
	dialer := net.Dialer{Timeout: 5 * time.Second}
	opts := tls.Config{
		RootCAs:      pool,
		Certificates: securityConfig.ClientTLSCreds.Config().Certificates,
	}
	for i, ca := range externalCAs {
		if ca.Protocol == api.ExternalCA_CAProtocolCFSSL && bytes.Equal(wantedCert, ca.CACert) {
			err := validateExternalCAURL(&dialer, &opts, ca.URL)
			if err == nil {
				return true
			}
			log.G(ctx).WithError(err).Warnf("external CA # %d is unreachable or invalid", i+1)
		}
	}
	return false
}

// All new external CA definitions must include the CA cert associated with the external CA.
// If the current root CA requires an external CA, then at least one, reachable valid external CA must be provided that
// corresponds with the current RootCA's certificate.
//
// Similarly for the desired CA certificate, if one is specified.  Similarly for the current outstanding root CA rotation,
// if one is specified and will not be replaced with the desired CA.
func validateHasRequiredExternalCAs(ctx context.Context, securityConfig *ca.SecurityConfig, cluster *api.Cluster) error {
	config := cluster.Spec.CAConfig
	for _, ca := range config.ExternalCAs {
		if len(ca.CACert) == 0 {
			return grpc.Errorf(codes.InvalidArgument, "must specify CA certificate for each external CA")
		}
	}

	if !hasSigningKey(cluster.RootCA) && !hasAtLeastOneExternalCA(ctx, config.ExternalCAs, securityConfig, cluster.RootCA.CACert) {
		return grpc.Errorf(codes.InvalidArgument, "there must be at least one valid, reachable external CA corresponding to the current CA certificate")
	}

	if len(config.SigningCACert) > 0 { // a signing cert is specified
		if !hasSigningKey(config) && !hasAtLeastOneExternalCA(ctx, config.ExternalCAs, securityConfig, config.SigningCACert) {
			return grpc.Errorf(codes.InvalidArgument, "there must be at least one valid, reachable external CA corresponding to the desired CA certificate")
		}
	} else if config.ForceRotate == cluster.RootCA.LastForcedRotation && cluster.RootCA.RootRotation != nil {
		// no cert is specified but force rotation hasn't changed (so we are happy with the current configuration) and there's an outstanding root rotation
		if !hasSigningKey(cluster.RootCA.RootRotation) && !hasAtLeastOneExternalCA(ctx, config.ExternalCAs, securityConfig, cluster.RootCA.RootRotation.CACert) {
			return grpc.Errorf(codes.InvalidArgument, "there must be at least one valid, reachable external CA corresponding to the next CA certificate")
		}
	}

	return nil
}

// validateAndUpdateCA validates a cluster's desired CA configuration spec, and returns a RootCA value on success representing
// current RootCA as it should be.  Validation logic and return values are as follows:
// 1. Validates that the contents are complete - e.g. a signing key is not provided without a signing cert, and that external
//    CAs are not removed if they are needed.  Otherwise, returns an error.
// 2. If no desired signing cert or key are provided, then either:
//    - we are happy with the current CA configuration (force rotation value has not changed), and we return the current RootCA
//      object as is
//    - we want to generate a new internal CA cert and key (force rotation value has changed), and we return the updated RootCA
//      object
// 3. Signing cert and key have been provided: validate that these match (the cert and key match). Otherwise, return an error.
// 4. Return the updated RootCA object according to the following criteria:
//    - If the desired cert is the same as the current CA cert then abort any outstanding rotations. The current signing key
//      is replaced with the desired signing key (this could lets us switch between external->internal or internal->external
//      without an actual CA rotation, which is not needed because any leaf cert issued with one CA cert can be validated using
//       the second CA certificate).
//    - If the desired cert is the same as the current to-be-rotated-to CA cert then a new root rotation is not needed. The
//      current to-be-rotated-to signing key is replaced with the desired signing key (this could lets us switch between
//      external->internal or internal->external without an actual CA rotation, which is not needed because any leaf cert
//      issued with one CA cert can be validated using the second CA certificate).
//    - Otherwise, start a new root rotation using the desired signing cert and desired signing key as the root rotation
//      signing cert and key.  If a root rotation is already in progress, just replace it and start over.
func validateCAConfig(ctx context.Context, securityConfig *ca.SecurityConfig, cluster *api.Cluster) (*api.RootCA, error) {
	newConfig := cluster.Spec.CAConfig

	if len(newConfig.SigningCAKey) > 0 && len(newConfig.SigningCACert) == 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, "if a signing CA key is provided, the signing CA cert must also be provided")
	}

	if err := validateHasRequiredExternalCAs(ctx, securityConfig, cluster); err != nil {
		return nil, err
	}

	// if the desired CA cert and key are not set, then we are happy with the current root CA configuration, unless
	// the ForceRotate version has changed
	if len(newConfig.SigningCACert) == 0 {
		if cluster.RootCA.LastForcedRotation != newConfig.ForceRotate {
			newRootCA, err := ca.CreateRootCA(ca.DefaultRootCN)
			if err != nil {
				return nil, grpc.Errorf(codes.Internal, err.Error())
			}
			return newRootRotationObject(ctx, securityConfig, cluster, newRootCA, newConfig.ForceRotate)
		}
		return &cluster.RootCA, nil // no change, return as is
	}

	// A desired cert and maybe key were provided - we need to make sure the cert and key (if provided) match.
	var signingCert []byte
	if hasSigningKey(newConfig) {
		signingCert = newConfig.SigningCACert
	}
	newRootCA, err := ca.NewRootCA(newConfig.SigningCACert, signingCert, newConfig.SigningCAKey, ca.DefaultNodeCertExpiration, nil)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, err.Error())
	}

	if len(newRootCA.Pool.Subjects()) != 1 {
		return nil, grpc.Errorf(codes.InvalidArgument, "the desired CA certificate cannot contain multiple certificates")
	}

	parsedCert, err := helpers.ParseCertificatePEM(newConfig.SigningCACert)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "could not parse the desired CA certificate")
	}

	// The new certificate's expiry must be at least one year away
	if parsedCert.NotAfter.Before(time.Now().Add(minRootExpiration)) {
		return nil, grpc.Errorf(codes.InvalidArgument, "CA certificate expires too soon")
	}

	// check if we can abort any existing root rotations
	if bytes.Equal(cluster.RootCA.CACert, cluster.Spec.CAConfig.SigningCACert) {
		copied := cluster.RootCA.Copy()
		copied.CAKey = newConfig.SigningCAKey
		copied.RootRotation = nil
		copied.LastForcedRotation = newConfig.ForceRotate
		return copied, nil
	}

	// check if this is the same desired cert as an existing root rotation
	if r := cluster.RootCA.RootRotation; r != nil && bytes.Equal(r.CACert, cluster.Spec.CAConfig.SigningCACert) {
		copied := cluster.RootCA.Copy()
		copied.RootRotation.CAKey = newConfig.SigningCAKey
		copied.LastForcedRotation = newConfig.ForceRotate
		return copied, nil
	}

	// ok, everything's different; we have to begin a new root rotation which means generating a new cross-signed cert
	return newRootRotationObject(ctx, securityConfig, cluster, newRootCA, newConfig.ForceRotate)
}
