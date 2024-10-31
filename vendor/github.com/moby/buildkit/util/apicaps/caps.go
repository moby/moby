package apicaps

import (
	"fmt"
	"sort"
	"strings"

	pb "github.com/moby/buildkit/util/apicaps/pb"
	"github.com/pkg/errors"
)

type PBCap = pb.APICap

// ExportedProduct is the name of the product using this package.
// Users vendoring this library may override it to provide better versioning hints
// for their users (or set it with a flag to buildkitd).
var ExportedProduct string

// CapStatus defines the stability properties of a capability
type CapStatus int

const (
	// CapStatusStable refers to a capability that should never be changed in
	// backwards incompatible manner unless there is a serious security issue.
	CapStatusStable CapStatus = iota
	// CapStatusExperimental refers to a capability that may be removed in the future.
	// If incompatible changes are made the previous ID is disabled and new is added.
	CapStatusExperimental
	// CapStatusPrerelease is same as CapStatusExperimental that can be used for new
	// features before they move to stable.
	CapStatusPrerelease
)

// CapID is type for capability identifier
type CapID string

// Cap describes an API feature
type Cap struct {
	ID                  CapID
	Name                string // readable name, may contain spaces but keep in one sentence
	Status              CapStatus
	Enabled             bool
	Deprecated          bool
	SupportedHint       map[string]string
	DisabledReason      string
	DisabledReasonMsg   string
	DisabledAlternative string
}

// CapList is a collection of capability definitions
type CapList struct {
	m map[CapID]Cap
}

// Init initializes definition for a new capability.
// Not safe to be called concurrently with other methods.
func (l *CapList) Init(cc ...Cap) {
	if l.m == nil {
		l.m = make(map[CapID]Cap, len(cc))
	}
	for _, c := range cc {
		l.m[c.ID] = c
	}
}

// All reports the configuration of all known capabilities
func (l *CapList) All() []pb.APICap {
	out := make([]pb.APICap, 0, len(l.m))
	for _, c := range l.m {
		out = append(out, pb.APICap{
			ID:                  string(c.ID),
			Enabled:             c.Enabled,
			Deprecated:          c.Deprecated,
			DisabledReason:      c.DisabledReason,
			DisabledReasonMsg:   c.DisabledReasonMsg,
			DisabledAlternative: c.DisabledAlternative,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// CapSet returns a CapSet for an capability configuration
func (l *CapList) CapSet(caps []pb.APICap) CapSet {
	m := make(map[string]*pb.APICap, len(caps))
	for _, c := range caps {
		if c.ID != "" {
			c := c // capture loop iterator
			m[c.ID] = &c
		}
	}
	return CapSet{
		list: l,
		set:  m,
	}
}

// CapSet is a configuration for detecting supported capabilities
type CapSet struct {
	list *CapList
	set  map[string]*pb.APICap
}

// Supports returns an error if capability is not supported
func (s *CapSet) Supports(id CapID) error {
	err := &CapError{ID: id}
	c, ok := s.list.m[id]
	if !ok {
		return errors.WithStack(err)
	}
	err.Definition = &c
	state, ok := s.set[string(id)]
	if !ok {
		return errors.WithStack(err)
	}
	err.State = state
	if !state.Enabled {
		return errors.WithStack(err)
	}
	return nil
}

// Contains checks if cap set contains cap. Note that unlike Supports() this
// function only checks capability existence in remote set, not if cap has been initialized.
func (s *CapSet) Contains(id CapID) bool {
	_, ok := s.set[string(id)]
	return ok
}

// CapError is an error for unsupported capability
type CapError struct {
	ID         CapID
	Definition *Cap
	State      *pb.APICap
}

func (e CapError) Error() string {
	if e.Definition == nil {
		return fmt.Sprintf("unknown API capability %s", e.ID)
	}
	typ := ""
	if e.Definition.Status == CapStatusExperimental {
		typ = "experimental "
	}
	if e.Definition.Status == CapStatusPrerelease {
		typ = "prerelease "
	}
	name := ""
	if e.Definition.Name != "" {
		name = "(" + e.Definition.Name + ")"
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "requested %sfeature %s %s", typ, e.ID, name)
	if e.State == nil {
		fmt.Fprint(b, " is not supported by build server")
		if hint, ok := e.Definition.SupportedHint[ExportedProduct]; ok {
			fmt.Fprintf(b, " (added in %s)", hint)
		}
		fmt.Fprintf(b, ", please update %s", ExportedProduct)
	} else {
		fmt.Fprint(b, " has been disabled on the build server")
		if e.State.DisabledReasonMsg != "" {
			fmt.Fprintf(b, ": %s", e.State.DisabledReasonMsg)
		}
	}
	return b.String()
}
