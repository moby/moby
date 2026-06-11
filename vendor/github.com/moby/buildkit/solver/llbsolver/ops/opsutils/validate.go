package opsutils

import (
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"
)

func Validate(op *pb.Op) error {
	if op == nil {
		return errors.Errorf("invalid nil op")
	}

	inputCount := len(op.Inputs)
	switch op := op.Op.(type) {
	case *pb.Op_Source:
		if op.Source == nil {
			return errors.Errorf("invalid nil source op")
		}
	case *pb.Op_Exec:
		if op.Exec == nil {
			return errors.Errorf("invalid nil exec op")
		}
		if op.Exec.Meta == nil {
			return errors.Errorf("invalid exec op with no meta")
		}
		if len(op.Exec.Meta.Args) == 0 {
			return errors.Errorf("invalid exec op with no args")
		}
		if len(op.Exec.Mounts) == 0 {
			return errors.Errorf("invalid exec op with no mounts")
		}

		isRoot := false
		for _, m := range op.Exec.Mounts {
			if m.Dest == pb.RootMount {
				isRoot = true
				break
			}
		}
		if !isRoot {
			return errors.Errorf("invalid exec op with no rootfs")
		}
	case *pb.Op_File:
		if op.File == nil {
			return errors.Errorf("invalid nil file op")
		}
		if len(op.File.Actions) == 0 {
			return errors.Errorf("invalid file op with no actions")
		}
	case *pb.Op_Build:
		if op.Build == nil {
			return errors.Errorf("invalid nil build op")
		}
	case *pb.Op_Merge:
		if op.Merge == nil {
			return errors.Errorf("invalid nil merge op")
		}
	case *pb.Op_Diff:
		if op.Diff == nil {
			return errors.Errorf("invalid nil diff op")
		}
	case *pb.Op_Passthrough:
		if op.Passthrough == nil {
			return errors.Errorf("invalid nil passthrough op")
		}
		if op.Passthrough.Id == "" {
			return errors.Errorf("invalid passthrough op with no id")
		}
		if len(op.Passthrough.Outputs) == 0 {
			return errors.Errorf("invalid passthrough op with no outputs")
		}
		for _, input := range op.Passthrough.Outputs {
			if input < 0 || inputCount > 0 && input >= int64(inputCount) {
				return errors.Errorf("invalid passthrough output input index %d", input)
			}
		}
	}
	return nil
}
