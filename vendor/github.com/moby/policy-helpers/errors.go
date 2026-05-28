package verifier

import (
	"fmt"

	digest "github.com/opencontainers/go-digest"
)

type NoSigChainError struct {
	Target         digest.Digest
	HasAttestation bool
}

var _ error = &NoSigChainError{}

func (e *NoSigChainError) Error() string {
	if e.HasAttestation {
		return fmt.Sprintf("no signature found for image %s", e.Target)
	}
	return fmt.Sprintf("no provenance attestation found for image %s", e.Target)
}
