// +build dfrunsecurity

package instructions

import (
	"encoding/csv"
	"strings"

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

type securityKeyT string

var securityKey = securityKeyT("dockerfile/run/security")

func init() {
	parseRunPreHooks = append(parseRunPreHooks, runSecurityPreHook)
	parseRunPostHooks = append(parseRunPostHooks, runSecurityPostHook)
}

func runSecurityPreHook(cmd *RunCommand, req parseRequest) error {
	st := &securityState{}
	st.flag = req.flags.AddStrings("security")
	cmd.setExternalValue(securityKey, st)
	return nil
}

func runSecurityPostHook(cmd *RunCommand, req parseRequest) error {
	st := getSecurityState(cmd)
	if st == nil {
		return errors.Errorf("no security state")
	}

	for _, value := range st.flag.StringValues {
		csvReader := csv.NewReader(strings.NewReader(value))
		fields, err := csvReader.Read()
		if err != nil {
			return errors.Wrap(err, "failed to parse csv security")
		}

		for _, field := range fields {
			if !isValidSecurity(field) {
				return errors.Errorf("security %q is not valid", field)
			}

			st.security = append(st.security, field)
		}
	}

	return nil
}

func getSecurityState(cmd *RunCommand) *securityState {
	v := cmd.getExternalValue(securityKey)
	if v == nil {
		return nil
	}
	return v.(*securityState)
}

func GetSecurity(cmd *RunCommand) []string {
	return getSecurityState(cmd).security
}

type securityState struct {
	flag     *Flag
	security []string
}
