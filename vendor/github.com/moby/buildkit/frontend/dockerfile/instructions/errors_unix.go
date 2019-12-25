// +build !windows

package instructions

import "fmt"

func errNotJSON(command, _ string) error {
	return fmt.Errorf("%s requires the arguments to be in JSON form", command)
}
