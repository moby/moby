package result

import (
	"reflect"

	pb "github.com/moby/buildkit/frontend/gateway/pb"
	digest "github.com/opencontainers/go-digest"
)

const (
	AttestationReasonKey     = "reason"
	AttestationSBOMCore      = "sbom-core"
	AttestationInlineOnlyKey = "inline-only"
)

const (
	AttestationReasonSBOM       = "sbom"
	AttestationReasonProvenance = "provenance"
)

type Attestation[T any] struct {
	Kind pb.AttestationKind

	Metadata map[string][]byte

	Ref         T
	Path        string
	ContentFunc func() ([]byte, error)

	InToto InTotoAttestation
}

type InTotoAttestation struct {
	PredicateType string
	Subjects      []InTotoSubject
}

type InTotoSubject struct {
	Kind pb.InTotoSubjectKind

	Name   string
	Digest []digest.Digest
}

func ToDigestMap(ds ...digest.Digest) map[string]string {
	m := map[string]string{}
	for _, d := range ds {
		m[d.Algorithm().String()] = d.Encoded()
	}
	return m
}

func FromDigestMap(m map[string]string) []digest.Digest {
	var ds []digest.Digest
	for k, v := range m {
		ds = append(ds, digest.NewDigestFromEncoded(digest.Algorithm(k), v))
	}
	return ds
}

func ConvertAttestation[U any, V any](a *Attestation[U], fn func(U) (V, error)) (*Attestation[V], error) {
	var ref V
	if reflect.ValueOf(a.Ref).IsValid() {
		var err error
		ref, err = fn(a.Ref)
		if err != nil {
			return nil, err
		}
	}

	return &Attestation[V]{
		Kind:        a.Kind,
		Metadata:    a.Metadata,
		Ref:         ref,
		Path:        a.Path,
		ContentFunc: a.ContentFunc,
		InToto:      a.InToto,
	}, nil
}
