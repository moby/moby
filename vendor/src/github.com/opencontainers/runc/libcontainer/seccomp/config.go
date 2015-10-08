package seccomp

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/configs"
)

// ConvertStringToOperator converts a string into a Seccomp comparison operator.
// Comparison operators use the names they are assigned by Libseccomp's header.
// Attempting to convert a string that is not a valid operator results in an
// error.
func ConvertStringToOperator(in string) (configs.Operator, error) {
	switch in {
	case "SCMP_CMP_NE":
		return configs.NotEqualTo, nil
	case "SCMP_CMP_LT":
		return configs.LessThan, nil
	case "SCMP_CMP_LE":
		return configs.LessThanOrEqualTo, nil
	case "SCMP_CMP_EQ":
		return configs.EqualTo, nil
	case "SCMP_CMP_GE":
		return configs.GreaterThan, nil
	case "SCMP_CMP_GT":
		return configs.GreaterThanOrEqualTo, nil
	case "SCMP_CMP_MASKED_EQ":
		return configs.MaskEqualTo, nil
	default:
		return 0, fmt.Errorf("string %s is not a valid operator for seccomp", in)
	}
}

// ConvertStringToAction converts a string into a Seccomp rule match action.
// Actions use the names they are assigned in Libseccomp's header, though some
// (notable, SCMP_ACT_TRACE) are not available in this implementation and will
// return errors.
// Attempting to convert a string that is not a valid action results in an
// error.
func ConvertStringToAction(in string) (configs.Action, error) {
	switch in {
	case "SCMP_ACT_KILL":
		return configs.Kill, nil
	case "SCMP_ACT_ERRNO":
		return configs.Errno, nil
	case "SCMP_ACT_TRAP":
		return configs.Trap, nil
	case "SCMP_ACT_ALLOW":
		return configs.Allow, nil
	default:
		return 0, fmt.Errorf("string %s is not a valid action for seccomp", in)
	}
}
