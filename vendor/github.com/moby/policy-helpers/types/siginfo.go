package types

import (
	"time"

	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
)

type Kind int

const (
	KindDockerGithubBuilder  Kind = 1
	KindDockerHardenedImage  Kind = 2
	KindSelfSignedGithubRepo Kind = 3
	KindSelfSigned           Kind = 4
	KindUntrusted            Kind = 1000
)

func (k Kind) String() string {
	switch k {
	case KindDockerGithubBuilder:
		return "Docker GitHub Builder"
	case KindDockerHardenedImage:
		return "Docker Hardened Image"
	case KindSelfSignedGithubRepo:
		return "GitHub Self-Signed"
	case KindSelfSigned:
		return "Self-Signed"
	case KindUntrusted:
		return "Untrusted"
	default:
		return "Invalid"
	}
}

type SignatureType int

const (
	SignatureBundleV03       SignatureType = 1
	SignatureSimpleSigningV1 SignatureType = 2
)

func (st SignatureType) String() string {
	switch st {
	case SignatureBundleV03:
		return "Sigstore Bundle"
	case SignatureSimpleSigningV1:
		return "SimpleSigning v1"
	default:
		return "Unknown"
	}
}

type TimestampVerificationResult struct {
	Type      string    `json:"type"`
	URI       string    `json:"uri"`
	Timestamp time.Time `json:"timestamp"`
}

type TrustRootStatus struct {
	Error       string     `json:"error,omitempty"`
	LastUpdated *time.Time `json:"lastUpdated,omitempty"`
}

type SignatureInfo struct {
	Kind            Kind                          `json:"kind"`
	SignatureType   SignatureType                 `json:"signatureType"`
	Signer          *certificate.Summary          `json:"signer,omitempty"`
	Timestamps      []TimestampVerificationResult `json:"timestamps,omitempty"`
	DockerReference string                        `json:"dockerReference,omitempty"`
	TrustRootStatus TrustRootStatus               `json:"trustRootStatus,omitzero"`
	IsDHI           bool                          `json:"isDHI,omitempty"`
}
