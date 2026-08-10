package attestation

import (
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/containerd/continuity/fs"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	"github.com/moby/buildkit/exporter"
	gatewaypb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/result"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const maxAttestationBytes int64 = 40 << 20

// ReadAll reads the content of an attestation.
func ReadAll(ctx context.Context, s session.Group, att exporter.Attestation) ([]byte, error) {
	var content []byte
	if att.ContentFunc != nil {
		data, err := att.ContentFunc(ctx)
		if err != nil {
			return nil, err
		}
		content = data
	} else if att.Ref != nil {
		mount, err := att.Ref.Mount(ctx, true, s)
		if err != nil {
			return nil, err
		}
		lm := snapshot.LocalMounter(mount)
		src, err := lm.Mount()
		if err != nil {
			return nil, err
		}
		defer lm.Unmount()

		p, err := fs.RootPath(src, att.Path)
		if err != nil {
			return nil, err
		}
		content, err = readRegularFile(p)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read in-toto attestation")
		}
	} else {
		return nil, errors.New("no available content for attestation")
	}
	if len(content) == 0 {
		content = nil
	}
	return content, nil
}

func readRegularFile(p string) ([]byte, error) {
	f, err := openRegularFile(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dt, err := readAllLimited(f, p, maxAttestationBytes)
	if err != nil {
		return nil, err
	}
	return dt, nil
}

func readAllLimited(r io.Reader, name string, limit int64) ([]byte, error) {
	limited := &io.LimitedReader{R: r, N: limit + 1}
	dt, err := io.ReadAll(limited)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if limited.N == 0 {
		return nil, errors.Errorf("%s exceeds %d bytes", name, limit)
	}
	return dt, nil
}

func openRegularFile(p string) (*os.File, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, errors.WithStack(err)
	}
	if !st.Mode().IsRegular() {
		f.Close()
		return nil, errors.Errorf("%s is not a regular file", p)
	}

	return f, nil
}

// MakeInTotoStatements iterates over all provided result attestations and
// generates intoto attestation statements.
func MakeInTotoStatements(ctx context.Context, s session.Group, attestations []exporter.Attestation, defaultSubjects []intoto.Subject) ([]intoto.Statement, error) {
	eg, ctx := errgroup.WithContext(ctx)
	statements := make([]intoto.Statement, len(attestations))

	for i, att := range attestations {
		eg.Go(func() error {
			content, err := ReadAll(ctx, s, att)
			if err != nil {
				return err
			}

			switch att.Kind {
			case gatewaypb.AttestationKind_InToto:
				stmt, err := makeInTotoStatement(content, att, defaultSubjects)
				if err != nil {
					return err
				}
				statements[i] = *stmt
			case gatewaypb.AttestationKind_Bundle:
				return errors.New("bundle attestation kind must be un-bundled first")
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return statements, nil
}

func makeInTotoStatement(content []byte, attestation exporter.Attestation, defaultSubjects []intoto.Subject) (*intoto.Statement, error) {
	if len(attestation.InToto.Subjects) == 0 {
		attestation.InToto.Subjects = []result.InTotoSubject{{
			Kind: gatewaypb.InTotoSubjectKind_Self,
		}}
	}
	subjects := []intoto.Subject{}
	for _, subject := range attestation.InToto.Subjects {
		subjectName := "_"
		if subject.Name != "" {
			subjectName = subject.Name
		}

		switch subject.Kind {
		case gatewaypb.InTotoSubjectKind_Self:
			for _, defaultSubject := range defaultSubjects {
				subjectNames := []string{}
				subjectNames = append(subjectNames, defaultSubject.Name)
				if subjectName != "_" {
					subjectNames = append(subjectNames, subjectName)
				}

				for _, name := range subjectNames {
					subjects = append(subjects, intoto.Subject{
						Name:   name,
						Digest: defaultSubject.Digest,
					})
				}
			}
		case gatewaypb.InTotoSubjectKind_Raw:
			subjects = append(subjects, intoto.Subject{
				Name:   subjectName,
				Digest: result.ToDigestMap(subject.Digest...),
			})
		default:
			return nil, errors.Errorf("unknown attestation subject type %T", subject)
		}
	}
	stmt := intoto.Statement{
		StatementHeader: intoto.StatementHeader{
			Type:          intoto.StatementInTotoV1,
			PredicateType: attestation.InToto.PredicateType,
			Subject:       subjects,
		},
		Predicate: json.RawMessage(content),
	}
	return &stmt, nil
}
