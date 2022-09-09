//go:build dfrunsecurity
// +build dfrunsecurity

package instructions

import (
	"github.com/pkg/errors"
)

const (
	SecurityInsecure = "insecure"
	SecuritySandbox  = "sandbox"
)

var allowedSecurity = map[string]struct{}{
	SecurityInsecure: {},
	SecuritySandbox:  {},
}

func isValidSecurity(value string) bool {
	_, ok := allowedSecurity[value]
	return ok
}

var securityKey = "dockerfile/run/security"

func init() {
	parseRunPreHooks = append(parseRunPreHooks, runSecurityPreHook)
	parseRunPostHooks = append(parseRunPostHooks, runSecurityPostHook)
}

func runSecurityPreHook(cmd *RunCommand, req parseRequest) error {
	st := &securityState{}
	st.flag = req.flags.AddString("security", SecuritySandbox)
	cmd.setExternalValue(securityKey, st)
	return nil
}

func runSecurityPostHook(cmd *RunCommand, req parseRequest) error {
	st := cmd.getExternalValue(securityKey).(*securityState)
	if st == nil {
		return errors.Errorf("no security state")
	}

	value := st.flag.Value
	if !isValidSecurity(value) {
		return errors.Errorf("security %q is not valid", value)
	}

	st.security = value

	return nil
}

func GetSecurity(cmd *RunCommand) string {
	return cmd.getExternalValue(securityKey).(*securityState).security
}

type securityState struct {
	flag     *Flag
	security string
}
