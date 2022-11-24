package attestation

import (
	"context"
	"encoding/json"
	"os"
	"path"

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

// Unbundle iterates over all provided result attestations and un-bundles any
// bundled attestations by loading them from the provided refs map.
func Unbundle(ctx context.Context, s session.Group, bundled []exporter.Attestation) ([]exporter.Attestation, error) {
	eg, ctx := errgroup.WithContext(ctx)
	unbundled := make([][]exporter.Attestation, len(bundled))

	for i, att := range bundled {
		i, att := i, att
		eg.Go(func() error {
			switch att.Kind {
			case gatewaypb.AttestationKindInToto:
				unbundled[i] = append(unbundled[i], att)
			case gatewaypb.AttestationKindBundle:
				if att.ContentFunc != nil {
					return errors.New("attestation bundle cannot have callback")
				}
				if att.Ref == nil {
					return errors.Errorf("no ref provided for attestation bundle")
				}

				mount, err := att.Ref.Mount(ctx, true, s)
				if err != nil {
					return err
				}
				lm := snapshot.LocalMounter(mount)
				src, err := lm.Mount()
				if err != nil {
					return err
				}
				defer lm.Unmount()

				atts, err := unbundle(ctx, src, att)
				if err != nil {
					return err
				}
				unbundled[i] = append(unbundled[i], atts...)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	var joined []exporter.Attestation
	for _, atts := range unbundled {
		joined = append(joined, atts...)
	}
	for _, att := range joined {
		if err := validate(att); err != nil {
			return nil, err
		}
	}
	return joined, nil
}

func unbundle(ctx context.Context, root string, bundle exporter.Attestation) ([]exporter.Attestation, error) {
	dir, err := fs.RootPath(root, bundle.Path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var unbundled []exporter.Attestation
	for _, entry := range entries {
		p, err := fs.RootPath(dir, entry.Name())
		if err != nil {
			return nil, err
		}
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(f)
		var stmt intoto.Statement
		if err := dec.Decode(&stmt); err != nil {
			return nil, errors.Wrap(err, "cannot decode in-toto statement")
		}
		if bundle.InToto.PredicateType != "" && stmt.PredicateType != bundle.InToto.PredicateType {
			return nil, errors.Errorf("bundle entry %s does not match required predicate type %s", stmt.PredicateType, bundle.InToto.PredicateType)
		}

		predicate, err := json.Marshal(stmt.Predicate)
		if err != nil {
			return nil, err
		}

		subjects := make([]result.InTotoSubject, len(stmt.Subject))
		for i, subject := range stmt.Subject {
			subjects[i] = result.InTotoSubject{
				Kind:   gatewaypb.InTotoSubjectKindRaw,
				Name:   subject.Name,
				Digest: result.FromDigestMap(subject.Digest),
			}
		}
		unbundled = append(unbundled, exporter.Attestation{
			Kind:        gatewaypb.AttestationKindInToto,
			Path:        path.Join(bundle.Path, entry.Name()),
			ContentFunc: func() ([]byte, error) { return predicate, nil },
			InToto: result.InTotoAttestation{
				PredicateType: stmt.PredicateType,
				Subjects:      subjects,
			},
		})
	}
	return unbundled, nil
}

func validate(att exporter.Attestation) error {
	if att.Path == "" {
		return errors.New("attestation does not have set path")
	}
	if att.Ref == nil && att.ContentFunc == nil {
		return errors.New("attestation does not have available content")
	}
	return nil
}
