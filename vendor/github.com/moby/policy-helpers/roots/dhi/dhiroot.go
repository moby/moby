package dhi

import (
	_ "embed"
	"time"

	"github.com/pkg/errors"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

//go:embed dhi.pub
var pubkeyPEM string

// This may need to be updated if key rotation occurs.
const dhiEpoch = 1743595200 // 2025-04-02

func TrustedRoot(fulcioTrustedRoot root.TrustedMaterial) (root.TrustedMaterial, error) {
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(pubkeyPEM))
	if err != nil {
		return nil, err
	}
	v, err := signature.LoadVerifierWithOpts(pubKey)
	if err != nil {
		return nil, errors.Wrap(err, "loading DHI public key verifier")
	}
	return &dhiTrustedMaterial{
		dhiVerifier: &dhiVerifier{v},
		fulcio:      fulcioTrustedRoot,
	}, nil
}

type dhiTrustedMaterial struct {
	*dhiVerifier
	fulcio root.TrustedMaterial
}

var _ root.TrustedMaterial = &dhiTrustedMaterial{}

func (d *dhiTrustedMaterial) PublicKeyVerifier(_ string) (root.TimeConstrainedVerifier, error) {
	return d.dhiVerifier, nil
}

func (d *dhiTrustedMaterial) TimestampingAuthorities() []root.TimestampingAuthority {
	return d.fulcio.TimestampingAuthorities()
}

func (d *dhiTrustedMaterial) FulcioCertificateAuthorities() []root.CertificateAuthority {
	return nil
}

func (d *dhiTrustedMaterial) RekorLogs() map[string]*root.TransparencyLog {
	return d.fulcio.RekorLogs()
}

func (d *dhiTrustedMaterial) CTLogs() map[string]*root.TransparencyLog {
	return nil
}

type dhiVerifier struct {
	signature.Verifier
}

func (d *dhiVerifier) ValidAtTime(t time.Time) bool {
	return t.Unix() >= dhiEpoch
}
