package apparmor

import (
	"fmt"
	"io/ioutil"
	"os"
)

func IsEnabled() bool {
	buf, err := ioutil.ReadFile("/sys/module/apparmor/parameters/enabled")
	return err == nil && len(buf) > 1 && buf[0] == 'Y'
}

func ApplyProfile(pid int, name string) error {
	if !IsEnabled() || name == "" {
		return nil
	}

	f, err := os.OpenFile(fmt.Sprintf("/proc/%d/attr/current", pid), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "changeprofile %s", name); err != nil {
		return err
	}
	return nil
}
