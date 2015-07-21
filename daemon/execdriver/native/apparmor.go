// +build linux

package native

import (
	"bufio"
	"os"
	"strings"
)

func hasAppArmorProfileLoaded(profile string) error {
	file, err := os.Open("/sys/kernel/security/apparmor/profiles")
	if err != nil {
		return err
	}
	r := bufio.NewReader(file)
	for {
		p, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.HasPrefix(p, profile+" ") {
			return nil
		}
	}
}
