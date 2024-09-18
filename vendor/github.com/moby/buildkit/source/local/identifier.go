package local

import (
	"github.com/moby/buildkit/solver/llbsolver/provenance"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/tonistiigi/fsutil"
)

type LocalIdentifier struct {
	Name            string
	SessionID       string
	IncludePatterns []string
	ExcludePatterns []string
	FollowPaths     []string
	SharedKeyHint   string
	Differ          fsutil.DiffType
}

func NewLocalIdentifier(str string) (*LocalIdentifier, error) {
	return &LocalIdentifier{Name: str}, nil
}

func (*LocalIdentifier) Scheme() string {
	return srctypes.LocalScheme
}

var _ source.Identifier = (*LocalIdentifier)(nil)

func (id *LocalIdentifier) Capture(c *provenance.Capture, pin string) error {
	c.AddLocal(provenancetypes.LocalSource{
		Name: id.Name,
	})
	return nil
}
