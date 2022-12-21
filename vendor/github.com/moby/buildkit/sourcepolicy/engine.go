package sourcepolicy

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
	spb "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/util/bklog"
	"github.com/pkg/errors"
)

var (
	// ErrSourceDenied is returned by the policy engine when a source is denied by the policy.
	ErrSourceDenied = errors.New("source denied by policy")

	// ErrTooManyOps is returned by the policy engine when there are too many converts for a single source op.
	ErrTooManyOps = errors.New("too many operations")
)

// Engine is the source policy engine.
// It is responsible for evaluating a source policy against a source operation.
// Create one with `NewEngine`
//
// Rule matching is delegated to the `Matcher` interface.
// Mutations are delegated to the `Mutater` interface.
type Engine struct {
	pol     []*spb.Policy
	sources map[string]*selectorCache
}

// NewEngine creates a new source policy engine.
func NewEngine(pol []*spb.Policy) *Engine {
	return &Engine{
		pol: pol,
	}
}

// TODO: The key here can't be used to cache attr constraint regexes.
func (e *Engine) selectorCache(src *spb.Selector) *selectorCache {
	if e.sources == nil {
		e.sources = map[string]*selectorCache{}
	}

	key := src.MatchType.String() + " " + src.Identifier

	if s, ok := e.sources[key]; ok {
		return s
	}

	s := &selectorCache{Selector: src}

	e.sources[key] = s
	return s
}

// Evaluate evaluates a source operation against the policy.
//
// Policies are re-evaluated for each convert rule.
// Evaluate will error if the there are too many converts for a single source op to prevent infinite loops.
// This function may error out even if the op was mutated, in which case `true` will be returned along with the error.
//
// An error is returned when the source is denied by the policy.
func (e *Engine) Evaluate(ctx context.Context, op *pb.Op) (bool, error) {
	if len(e.pol) == 0 {
		return false, nil
	}

	var mutated bool
	const maxIterr = 20

	for i := 0; ; i++ {
		if i > maxIterr {
			return mutated, errors.Wrapf(ErrTooManyOps, "too many mutations on a single source")
		}

		srcOp := op.GetSource()
		if srcOp == nil {
			return false, nil
		}
		if i == 0 {
			ctx = bklog.WithLogger(ctx, bklog.G(ctx).WithField("orig", *srcOp).WithField("updated", op.GetSource()))
		}

		mut, err := e.evaluatePolicies(ctx, srcOp)
		if mut {
			mutated = true
		}
		if err != nil {
			return mutated, err
		}
		if !mut {
			break
		}
	}

	return mutated, nil
}

func (e *Engine) evaluatePolicies(ctx context.Context, srcOp *pb.SourceOp) (bool, error) {
	for _, pol := range e.pol {
		mut, err := e.evaluatePolicy(ctx, pol, srcOp)
		if mut || err != nil {
			return mut, err
		}
	}
	return false, nil
}

// evaluatePolicy evaluates a single policy against a source operation.
// If the source is mutated the policy is short-circuited and `true` is returned.
// If the source is denied, an error will be returned.
//
// For Allow/Deny rules, the last matching rule wins.
// E.g. `ALLOW foo; DENY foo` will deny `foo`, `DENY foo; ALLOW foo` will allow `foo`.
func (e *Engine) evaluatePolicy(ctx context.Context, pol *spb.Policy, srcOp *pb.SourceOp) (bool, error) {
	ident := srcOp.GetIdentifier()

	ctx = bklog.WithLogger(ctx, bklog.G(ctx).WithFields(map[string]interface{}{
		"ref": ident,
	}))

	var deny bool
	for _, rule := range pol.Rules {
		selector := e.selectorCache(rule.Selector)
		matched, err := match(ctx, selector, ident, srcOp.Attrs)
		if err != nil {
			return false, errors.Wrap(err, "error matching source policy")
		}
		if !matched {
			continue
		}

		switch rule.Action {
		case spb.PolicyAction_ALLOW:
			deny = false
		case spb.PolicyAction_DENY:
			deny = true
		case spb.PolicyAction_CONVERT:
			mut, err := mutate(ctx, srcOp, rule, selector, ident)
			if err != nil || mut {
				return mut, errors.Wrap(err, "error mutating source policy")
			}
		default:
			return false, errors.Errorf("source policy: rule %s %s: unknown type %q", rule.Action, rule.Selector.Identifier, ident)
		}
	}

	if deny {
		return false, errors.Wrapf(ErrSourceDenied, "source %q denied by policy", ident)
	}
	return false, nil
}
