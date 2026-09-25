/*
Wrapper APIs for in-toto attestation Statement layer protos.
*/

package v1

import (
	"errors"
)

const statementTypeUriPrefix = "https://in-toto.io/Statement/"
const statementTypeUriLegacy = statementTypeUriPrefix + "v0.1"
const StatementTypeUri = statementTypeUriPrefix + "v1"

var (
	ErrInvalidStatementType  = errors.New("wrong statement type")
	ErrSubjectRequired       = errors.New("at least one subject required")
	ErrDigestRequired        = errors.New("at least one digest required")
	ErrPredicateTypeRequired = errors.New("predicate type required")
	ErrPredicateRequired     = errors.New("predicate object required")
)

func (s *Statement) Validate() error {
	if !s.isValidType() {
		return ErrInvalidStatementType
	}

	if s.GetSubject() == nil || len(s.GetSubject()) == 0 {
		return ErrSubjectRequired
	}

	// check all resource descriptors in the subject
	subject := s.GetSubject()
	for _, rd := range subject {
		if err := rd.Validate(); err != nil {
			return err
		}

		// v1 statements require the digest to be set in the subject
		if len(rd.GetDigest()) == 0 {
			return ErrDigestRequired
		}
	}

	if s.GetPredicateType() == "" {
		return ErrPredicateTypeRequired
	}

	if s.GetPredicate() == nil {
		return ErrPredicateRequired
	}

	return nil
}

func (s *Statement) isValidType() bool {
	return s.GetType() == StatementTypeUri || s.GetType() == statementTypeUriLegacy
}
