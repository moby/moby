package pb

import "github.com/pkg/errors"

func ValidateSecurityMode(mode SecurityMode) error {
	switch mode {
	case SecurityMode_SANDBOX, SecurityMode_INSECURE:
		return nil
	default:
		return errors.Errorf("invalid security mode %d", mode)
	}
}
