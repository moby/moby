// +build !linux

package file

import (
	"github.com/moby/buildkit/solver/llbsolver/ops/fileoptypes"
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"
	copy "github.com/tonistiigi/fsutil/copy"
)

func readUser(chopt *pb.ChownOpt, mu, mg fileoptypes.Mount) (*copy.User, error) {
	if chopt == nil {
		return nil, nil
	}
	return nil, errors.New("only implemented in linux")
}
