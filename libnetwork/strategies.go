package libnetwork

import "fmt"

type strategyParams map[string]interface{}
type strategyConstructor func(strategyParams) (Network, error)

var strategies = map[string]strategyConstructor{}

func RegisterNetworkType(name string, ctor strategyConstructor) error {
	if _, ok := strategies[name]; ok {
		return fmt.Errorf("network type %q is already registed", name)
	}
	strategies[name] = ctor
	return nil
}
