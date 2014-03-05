package apparmor

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

var AppArmorEnabled bool

var (
	ErrAppArmorDisabled = errors.New("Error: AppArmor is not enabled on this system")
)

func init() {
	buf, err := ioutil.ReadFile("/sys/module/apparmor/parameters/enabled")
	AppArmorEnabled = err == nil && len(buf) > 1 && buf[0] == 'Y'
}

func ApplyProfile(pid int, name string) error {
	if !AppArmorEnabled {
		return ErrAppArmorDisabled
	}

	f, err := os.OpenFile(fmt.Sprintf("/proc/%d/attr/current", pid), os.O_WRONLY, 0)
	if err != nil {
		log.Printf("error open: %s\n", err)
		return err
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "changeprofile %s", name); err != nil {
		log.Printf("changeprofile %s", name)
		log.Printf("Error write: %s\n", err)
		return err
	} else {
		log.Printf("Write success!")
	}
	return nil
}
