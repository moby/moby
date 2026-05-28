package pgpsign

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"io"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// VerifyPolicy defines validation policy for OpenPGP signature verification.
type VerifyPolicy struct {
	RejectExpiredKeys bool
}

// ParseArmoredDetachedSignature parses a detached armored OpenPGP signature and
// returns the first signature packet and the decoded binary signature payload.
func ParseArmoredDetachedSignature(data []byte) (*packet.Signature, []byte, error) {
	block, err := armor.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to decode armored signature")
	}
	sigBlock, err := io.ReadAll(block.Body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to read armored signature body")
	}

	pr := packet.NewReader(bytes.NewReader(sigBlock))
	for {
		p, err := pr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to read next packet")
		}
		sig, ok := p.(*packet.Signature)
		if !ok {
			continue
		}
		return sig, sigBlock, nil
	}
	return nil, nil, errors.New("no signature packet found")
}

// ReadAllArmoredKeyRings parses one or more concatenated armored OpenPGP key
// blocks and returns a combined entity list.
func ReadAllArmoredKeyRings(pubKeyData []byte) (openpgp.EntityList, error) {
	var ents openpgp.EntityList
	r := bytes.NewReader(pubKeyData)

	for {
		block, err := armor.Decode(r)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode armored public key")
		}

		if block.Type != openpgp.PublicKeyType && block.Type != openpgp.PrivateKeyType {
			return nil, errors.Errorf("expected public or private key block, got: %s", block.Type)
		}

		el, err := openpgp.ReadKeyRing(block.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read armored public key")
		}
		ents = append(ents, el...)
	}

	if len(ents) == 0 {
		return nil, errors.New("failed to read armored public key: no armored data found")
	}

	return ents, nil
}

// VerifyArmoredDetachedSignature verifies an armored detached OpenPGP
// signature against signedData using one or more armored public key blocks.
func VerifyArmoredDetachedSignature(signedData io.Reader, signatureData, pubKeyData []byte, policy *VerifyPolicy) error {
	sig, sigBlock, err := ParseArmoredDetachedSignature(signatureData)
	if err != nil {
		return err
	}

	ents, err := ReadAllArmoredKeyRings(pubKeyData)
	if err != nil {
		return err
	}

	if err := checkAlgoPolicy(sig); err != nil {
		return err
	}

	config := &packet.Config{}
	if policy == nil || !policy.RejectExpiredKeys {
		config.Time = func() time.Time {
			return sig.CreationTime
		}
	}

	signer, err := openpgp.CheckDetachedSignature(
		ents,
		signedData,
		bytes.NewReader(sigBlock),
		config,
	)
	if err != nil {
		if sig.IssuerKeyId != nil {
			return errors.Wrapf(err, "signature by %X", *sig.IssuerKeyId)
		}
		return err
	}

	now := time.Now()
	if err := checkEntityUsableForSigning(signer, now, policy); err != nil {
		return err
	}
	if err := checkCreationTime(sig.CreationTime, now); err != nil {
		return err
	}
	return nil
}

// VerifySignatureWithDigest verifies a parsed signature against a digest of
// the signed payload plus OpenPGP hash suffix (payload || suffix) using the
// provided keyring.
func VerifySignatureWithDigest(sig *packet.Signature, keyring openpgp.EntityList, dgst digest.Digest) error {
	if sig == nil {
		return errors.New("nil signature")
	}

	expectedAlgo, err := signatureDigestAlgorithm(sig.Hash)
	if err != nil {
		return err
	}
	if dgst.Algorithm() != expectedAlgo {
		return errors.Errorf("digest algorithm mismatch: %s != %s", dgst.Algorithm(), expectedAlgo)
	}

	sum, err := hex.DecodeString(dgst.Encoded())
	if err != nil {
		return errors.Wrap(err, "invalid digest hex")
	}

	h := &staticHash{sum: sum, algo: sig.Hash}
	if len(sum) != h.Size() {
		return errors.Errorf("digest size mismatch: got %d, expected %d", len(sum), h.Size())
	}
	for _, e := range keyring {
		if e.PrimaryKey != nil && e.PrimaryKey.VerifySignature(h, sig) == nil {
			return nil
		}
		for _, sub := range e.Subkeys {
			if sub.PublicKey != nil && sub.PublicKey.VerifySignature(h, sig) == nil {
				return nil
			}
		}
	}

	return errors.New("failed to verify signature with checksum digest")
}

func signatureDigestAlgorithm(h crypto.Hash) (digest.Algorithm, error) {
	switch h {
	case crypto.SHA256:
		return digest.SHA256, nil
	case crypto.SHA384:
		return digest.SHA384, nil
	case crypto.SHA512:
		return digest.SHA512, nil
	default:
		return "", errors.Errorf("unsupported signature hash algorithm %v", h)
	}
}

type staticHash struct {
	sum  []byte
	algo crypto.Hash
}

func (s *staticHash) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (s *staticHash) Sum(b []byte) []byte {
	return append(b, s.sum...)
}

func (s *staticHash) Reset() {}

func (s *staticHash) Size() int {
	switch s.algo {
	case crypto.SHA256:
		return sha256.Size
	case crypto.SHA384:
		return sha512.Size384
	case crypto.SHA512:
		return sha512.Size
	default:
		return len(s.sum)
	}
}

func (s *staticHash) BlockSize() int {
	switch s.algo {
	case crypto.SHA256:
		return sha256.BlockSize
	case crypto.SHA384, crypto.SHA512:
		return sha512.BlockSize
	default:
		return 0
	}
}

func checkAlgoPolicy(sig *packet.Signature) error {
	switch sig.Hash {
	case crypto.SHA256, crypto.SHA384, crypto.SHA512:
		// ok
	default:
		return errors.Errorf("rejecting weak/unknown hash: %v", sig.Hash)
	}

	switch sig.PubKeyAlgo {
	case packet.PubKeyAlgoEdDSA, packet.PubKeyAlgoECDSA, packet.PubKeyAlgoRSA, packet.PubKeyAlgoRSASignOnly:
	default:
		return errors.Errorf("rejecting unsupported pubkey algorithm: %v", sig.PubKeyAlgo)
	}
	return nil
}

func checkEntityUsableForSigning(e *openpgp.Entity, now time.Time, policy *VerifyPolicy) error {
	if e == nil || e.PrimaryKey == nil {
		return errors.New("nil entity or key")
	}

	if policy != nil && policy.RejectExpiredKeys {
		if id := e.PrimaryIdentity(); id != nil && id.SelfSignature != nil {
			if exp := id.SelfSignature.KeyLifetimeSecs; exp != nil && *exp > 0 {
				expiry := e.PrimaryKey.CreationTime.Add(time.Duration(*exp) * time.Second)
				if now.After(expiry) {
					return errors.Errorf("key expired at %v", expiry)
				}
			}
		}
	}

	if err := checkEntityRevocation(e); err != nil {
		return err
	}

	if rsaPub, ok := e.PrimaryKey.PublicKey.(*rsa.PublicKey); ok {
		if rsaPub.N.BitLen() < 2048 {
			return errors.Errorf("RSA key too short: %d bits", rsaPub.N.BitLen())
		}
	}

	return nil
}

func checkEntityRevocation(e *openpgp.Entity) error {
	if e == nil {
		return nil
	}
	for _, r := range e.Revocations {
		if r == nil || r.SigType != packet.SigTypeKeyRevocation {
			continue
		}
		if err := e.PrimaryKey.VerifyRevocationSignature(r); err != nil {
			continue
		}
		if r.RevocationReasonText != "" {
			return errors.Errorf("key revoked: %s", r.RevocationReasonText)
		}
		return errors.New("key revoked")
	}
	return nil
}

func checkCreationTime(sigTime, now time.Time) error {
	if sigTime.After(now.Add(5 * time.Minute)) {
		return errors.Errorf("signature creation time is in the future: %v", sigTime)
	}
	return nil
}
