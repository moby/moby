package client

import (
	pb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver/result"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func AttestationToPB[T any](a *result.Attestation[T]) (*pb.Attestation, error) {
	if a.ContentFunc != nil {
		return nil, errors.Errorf("attestation callback cannot be sent through gateway")
	}

	subjects := make([]*pb.InTotoSubject, len(a.InToto.Subjects))
	for i, subject := range a.InToto.Subjects {
		subjects[i] = &pb.InTotoSubject{
			Kind:   subject.Kind,
			Name:   subject.Name,
			Digest: digestSliceToPB(subject.Digest),
		}
	}

	return &pb.Attestation{
		Kind:                a.Kind,
		Metadata:            a.Metadata,
		Path:                a.Path,
		InTotoPredicateType: a.InToto.PredicateType,
		InTotoSubjects:      subjects,
	}, nil
}

func AttestationFromPB[T any](a *pb.Attestation) (*result.Attestation[T], error) {
	if a == nil {
		return nil, errors.Errorf("invalid nil attestation")
	}
	subjects := make([]result.InTotoSubject, len(a.InTotoSubjects))
	for i, subject := range a.InTotoSubjects {
		if subject == nil {
			return nil, errors.Errorf("invalid nil attestation subject")
		}
		subjects[i] = result.InTotoSubject{
			Kind:   subject.Kind,
			Name:   subject.Name,
			Digest: digestSliceFromPB(subject.Digest),
		}
	}

	return &result.Attestation[T]{
		Kind:     a.Kind,
		Metadata: a.Metadata,
		Path:     a.Path,
		InToto: result.InTotoAttestation{
			PredicateType: a.InTotoPredicateType,
			Subjects:      subjects,
		},
	}, nil
}

func digestSliceToPB(elems []digest.Digest) []string {
	clone := make([]string, len(elems))
	for i, e := range elems {
		clone[i] = string(e)
	}
	return clone
}

func digestSliceFromPB(elems []string) []digest.Digest {
	clone := make([]digest.Digest, len(elems))
	for i, e := range elems {
		clone[i] = digest.Digest(e)
	}
	return clone
}
