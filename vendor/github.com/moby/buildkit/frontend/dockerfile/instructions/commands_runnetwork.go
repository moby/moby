//go:build dfrunnetwork
// +build dfrunnetwork

package instructions

import (
	"github.com/pkg/errors"
)

const (
	NetworkDefault = "default"
	NetworkNone    = "none"
	NetworkHost    = "host"
)

var allowedNetwork = map[string]struct{}{
	NetworkDefault: {},
	NetworkNone:    {},
	NetworkHost:    {},
}

func isValidNetwork(value string) bool {
	_, ok := allowedNetwork[value]
	return ok
}

var networkKey = "dockerfile/run/network"

func init() {
	parseRunPreHooks = append(parseRunPreHooks, runNetworkPreHook)
	parseRunPostHooks = append(parseRunPostHooks, runNetworkPostHook)
}

func runNetworkPreHook(cmd *RunCommand, req parseRequest) error {
	st := &networkState{}
	st.flag = req.flags.AddString("network", NetworkDefault)
	cmd.setExternalValue(networkKey, st)
	return nil
}

func runNetworkPostHook(cmd *RunCommand, req parseRequest) error {
	st := cmd.getExternalValue(networkKey).(*networkState)
	if st == nil {
		return errors.Errorf("no network state")
	}

	value := st.flag.Value
	if !isValidNetwork(value) {
		return errors.Errorf("invalid network mode %q", value)
	}

	st.networkMode = value

	return nil
}

func GetNetwork(cmd *RunCommand) string {
	return cmd.getExternalValue(networkKey).(*networkState).networkMode
}

type networkState struct {
	flag        *Flag
	networkMode string
}
