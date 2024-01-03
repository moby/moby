package client

import (
	pb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver/result"
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
			Digest: subject.Digest,
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
	subjects := make([]result.InTotoSubject, len(a.InTotoSubjects))
	for i, subject := range a.InTotoSubjects {
		subjects[i] = result.InTotoSubject{
			Kind:   subject.Kind,
			Name:   subject.Name,
			Digest: subject.Digest,
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
