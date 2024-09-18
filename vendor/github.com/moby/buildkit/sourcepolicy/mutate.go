package sourcepolicy

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
	spb "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/util/bklog"
	"github.com/pkg/errors"
)

// mutate is a MutateFn which converts the source operation to the identifier and attributes provided by the policy.
// If there is no change, then the return value should be false and is not considered an error.
func mutate(ctx context.Context, op *pb.SourceOp, rule *spb.Rule, selector *selectorCache, ref string) (bool, error) {
	if rule.Updates == nil {
		return false, errors.Errorf("missing destination for convert rule")
	}

	dest := rule.Updates.Identifier
	if dest == "" {
		dest = rule.Selector.Identifier
	}
	dest, err := selector.Format(ref, dest)
	if err != nil {
		return false, errors.Wrap(err, "error formatting destination")
	}

	bklog.G(ctx).Debugf("sourcepolicy: converting %s to %s, pattern: %s", ref, dest, rule.Updates.Identifier)

	var mutated bool
	if op.Identifier != dest && dest != "" {
		mutated = true
		op.Identifier = dest
	}

	if rule.Updates.Attrs != nil {
		if op.Attrs == nil {
			op.Attrs = make(map[string]string, len(rule.Updates.Attrs))
		}
		for k, v := range rule.Updates.Attrs {
			if op.Attrs[k] != v {
				bklog.G(ctx).Debugf("setting attr %s=%s", k, v)
				op.Attrs[k] = v
				mutated = true
			}
		}
	}

	return mutated, nil
}
