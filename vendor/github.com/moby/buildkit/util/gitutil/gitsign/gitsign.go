package gitsign

import (
	"bytes"
	"encoding/pem"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/hiddeco/sshsig"
	"github.com/moby/buildkit/util/gitutil/gitobject"
	"github.com/moby/buildkit/util/pgpsign"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

type Signature struct {
	PGPSignature *packet.Signature
	SSHSignature *sshsig.Signature
}

func VerifySignature(obj *gitobject.GitObject, pubKeyData []byte, policy *pgpsign.VerifyPolicy) error {
	if len(obj.Signature) == 0 {
		return errors.New("git object is not signed")
	}

	s, err := ParseSignature([]byte(obj.Signature))
	if err != nil {
		return err
	}
	if s.PGPSignature != nil {
		return verifyPGPSignature(obj, pubKeyData, policy)
	} else if s.SSHSignature != nil {
		return verifySSHSignature(obj, s.SSHSignature, pubKeyData)
	}
	return errors.New("no valid signature found")
}

func verifyPGPSignature(obj *gitobject.GitObject, pubKeyData []byte, policy *pgpsign.VerifyPolicy) error {
	return pgpsign.VerifyArmoredDetachedSignature(
		bytes.NewReader([]byte(obj.SignedData)),
		[]byte(obj.Signature),
		pubKeyData,
		policy,
	)
}

func verifySSHSignature(obj *gitobject.GitObject, sig *sshsig.Signature, pubKeyData []byte) error {
	// future proofing
	if sig.Version != 1 {
		return errors.Errorf("unsupported SSH signature version: %d", sig.Version)
	}

	switch sig.HashAlgorithm {
	case sshsig.HashSHA256, sshsig.HashSHA512:
		// OK
	default:
		return errors.Errorf("unsupported SSH signature hash algorithm: %s", sig.HashAlgorithm)
	}
	if sig.Namespace != "git" {
		return errors.Errorf("unexpected SSH signature namespace: %q", sig.Namespace)
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(pubKeyData)
	if err != nil {
		return errors.Wrap(err, "failed to parse ssh public key")
	}

	if err := sshsig.Verify(strings.NewReader(obj.SignedData), sig, pubKey, sig.HashAlgorithm, sig.Namespace); err != nil {
		return errors.Wrap(err, "failed to verify ssh signature")
	}
	return nil
}

func parseSignatureBlock(data []byte) ([]byte, error) {
	if strings.HasPrefix(string(data), "-----BEGIN SSH SIGNATURE-----") {
		block, _ := pem.Decode(data)
		if block == nil || block.Type != "SSH SIGNATURE" {
			return nil, errors.New("failed to decode ssh signature PEM block")
		}
		return block.Bytes, nil
	}
	return nil, errors.Errorf("invalid signature format")
}

func ParseSignature(data []byte) (*Signature, error) {
	if strings.HasPrefix(string(data), "-----BEGIN PGP SIGNATURE-----") {
		sig, _, err := pgpsign.ParseArmoredDetachedSignature(data)
		if err != nil {
			return nil, err
		}
		return &Signature{PGPSignature: sig}, nil
	}
	if strings.HasPrefix(string(data), "-----BEGIN SSH SIGNATURE-----") {
		sigBlock, err := parseSignatureBlock(data)
		if err != nil {
			return nil, err
		}
		sig, err := sshsig.ParseSignature(sigBlock)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse ssh signature")
		}
		return &Signature{SSHSignature: sig}, nil
	}
	return nil, errors.Errorf("invalid signature format")
}
