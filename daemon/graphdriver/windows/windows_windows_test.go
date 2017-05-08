package windows

import "testing"

func TestAddAceToSddlDacl(t *testing.T) {
	cases := [][3]string{
		{"D:", "(A;;;)", "D:(A;;;)"},
		{"D:(A;;;)", "(A;;;)", "D:(A;;;)"},
		{"O:D:(A;;;stuff)", "(A;;;new)", "O:D:(A;;;new)(A;;;stuff)"},
		{"O:D:(D;;;no)(A;;;stuff)", "(A;;;new)", "O:D:(D;;;no)(A;;;new)(A;;;stuff)"},
	}

	for _, c := range cases {
		if newSddl, worked := addAceToSddlDacl(c[0], c[1]); !worked || newSddl != c[2] {
			t.Errorf("%s + %s == %s, expected %s (%v)", c[0], c[1], newSddl, c[2], worked)
		}
	}
}
