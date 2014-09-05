// +build selinux,linux

package label

import (
	"testing"

	"github.com/docker/libcontainer/selinux"
)

func TestInit(t *testing.T) {
	if selinux.SelinuxEnabled() {
		var testNull []string
		plabel, mlabel, err := InitLabels(testNull)
		if err != nil {
			t.Log("InitLabels Failed")
			t.Fatal(err)
		}
		testDisabled := []string{"disable"}
		plabel, mlabel, err = InitLabels(testDisabled)
		if err != nil {
			t.Log("InitLabels Disabled Failed")
			t.Fatal(err)
		}
		if plabel != "" {
			t.Log("InitLabels Disabled Failed")
			t.Fatal()
		}
		testUser := []string{"user:user_u", "role:user_r", "type:user_t", "level:s0:c1,c15"}
		plabel, mlabel, err = InitLabels(testUser)
		if err != nil {
			t.Log("InitLabels User Failed")
			t.Fatal(err)
		}
		if plabel != "user_u:user_r:user_t:s0:c1,c15" || mlabel != "user_u:object_r:svirt_sandbox_file_t:s0:c1,c15" {
			t.Log("InitLabels User Failed")
			t.Log(plabel, mlabel)
			t.Fatal(err)
		}

		testBadData := []string{"user", "role:user_r", "type:user_t", "level:s0:c1,c15"}
		plabel, mlabel, err = InitLabels(testBadData)
		if err == nil {
			t.Log("InitLabels Bad Failed")
			t.Fatal(err)
		}
	}
}
