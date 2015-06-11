package opts

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestValidateIPAddress(t *testing.T) {
	if ret, err := ValidateIPAddress(`1.2.3.4`); err != nil || ret == "" {
		t.Fatalf("ValidateIPAddress(`1.2.3.4`) got %s %s", ret, err)
	}

	if ret, err := ValidateIPAddress(`127.0.0.1`); err != nil || ret == "" {
		t.Fatalf("ValidateIPAddress(`127.0.0.1`) got %s %s", ret, err)
	}

	if ret, err := ValidateIPAddress(`::1`); err != nil || ret == "" {
		t.Fatalf("ValidateIPAddress(`::1`) got %s %s", ret, err)
	}

	if ret, err := ValidateIPAddress(`127`); err == nil || ret != "" {
		t.Fatalf("ValidateIPAddress(`127`) got %s %s", ret, err)
	}

	if ret, err := ValidateIPAddress(`random invalid string`); err == nil || ret != "" {
		t.Fatalf("ValidateIPAddress(`random invalid string`) got %s %s", ret, err)
	}

}

func TestMapOpts(t *testing.T) {
	tmpMap := make(map[string]string)
	o := newMapOpt(tmpMap, logOptsValidator)
	o.Set("max-size=1")
	if o.String() != "map[max-size:1]" {
		t.Errorf("%s != [map[max-size:1]", o.String())
	}

	o.Set("max-file=2")
	if len(tmpMap) != 2 {
		t.Errorf("map length %d != 2", len(tmpMap))
	}

	if tmpMap["max-file"] != "2" {
		t.Errorf("max-file = %s != 2", tmpMap["max-file"])
	}

	if tmpMap["max-size"] != "1" {
		t.Errorf("max-size = %s != 1", tmpMap["max-size"])
	}
	if o.Set("dummy-val=3") == nil {
		t.Errorf("validator is not being called")
	}
}

func TestValidateMACAddress(t *testing.T) {
	if _, err := ValidateMACAddress(`92:d0:c6:0a:29:33`); err != nil {
		t.Fatalf("ValidateMACAddress(`92:d0:c6:0a:29:33`) got %s", err)
	}

	if _, err := ValidateMACAddress(`92:d0:c6:0a:33`); err == nil {
		t.Fatalf("ValidateMACAddress(`92:d0:c6:0a:33`) succeeded; expected failure on invalid MAC")
	}

	if _, err := ValidateMACAddress(`random invalid string`); err == nil {
		t.Fatalf("ValidateMACAddress(`random invalid string`) succeeded; expected failure on invalid MAC")
	}
}

func TestListOpts(t *testing.T) {
	o := NewListOpts(nil)
	o.Set("foo")
	if o.String() != "[foo]" {
		t.Errorf("%s != [foo]", o.String())
	}
	o.Set("bar")
	if o.Len() != 2 {
		t.Errorf("%d != 2", o.Len())
	}
	if !o.Get("bar") {
		t.Error("o.Get(\"bar\") == false")
	}
	if o.Get("baz") {
		t.Error("o.Get(\"baz\") == true")
	}
	o.Delete("foo")
	if o.String() != "[bar]" {
		t.Errorf("%s != [bar]", o.String())
	}
}

func TestValidateDnsSearch(t *testing.T) {
	valid := []string{
		`.`,
		`a`,
		`a.`,
		`1.foo`,
		`17.foo`,
		`foo.bar`,
		`foo.bar.baz`,
		`foo.bar.`,
		`foo.bar.baz`,
		`foo1.bar2`,
		`foo1.bar2.baz`,
		`1foo.2bar.`,
		`1foo.2bar.baz`,
		`foo-1.bar-2`,
		`foo-1.bar-2.baz`,
		`foo-1.bar-2.`,
		`foo-1.bar-2.baz`,
		`1-foo.2-bar`,
		`1-foo.2-bar.baz`,
		`1-foo.2-bar.`,
		`1-foo.2-bar.baz`,
	}

	invalid := []string{
		``,
		` `,
		`  `,
		`17`,
		`17.`,
		`.17`,
		`17-.`,
		`17-.foo`,
		`.foo`,
		`foo-.bar`,
		`-foo.bar`,
		`foo.bar-`,
		`foo.bar-.baz`,
		`foo.-bar`,
		`foo.-bar.baz`,
		`foo.bar.baz.this.should.fail.on.long.name.beause.it.is.longer.thanisshouldbethis.should.fail.on.long.name.beause.it.is.longer.thanisshouldbethis.should.fail.on.long.name.beause.it.is.longer.thanisshouldbethis.should.fail.on.long.name.beause.it.is.longer.thanisshouldbe`,
	}

	for _, domain := range valid {
		if ret, err := ValidateDnsSearch(domain); err != nil || ret == "" {
			t.Fatalf("ValidateDnsSearch(`"+domain+"`) got %s %s", ret, err)
		}
	}

	for _, domain := range invalid {
		if ret, err := ValidateDnsSearch(domain); err == nil || ret != "" {
			t.Fatalf("ValidateDnsSearch(`"+domain+"`) got %s %s", ret, err)
		}
	}
}

func TestValidateExtraHosts(t *testing.T) {
	valid := []string{
		`myhost:192.168.0.1`,
		`thathost:10.0.2.1`,
		`anipv6host:2003:ab34:e::1`,
		`ipv6local:::1`,
	}

	invalid := map[string]string{
		`myhost:192.notanipaddress.1`:  `invalid IP`,
		`thathost-nosemicolon10.0.0.1`: `bad format`,
		`anipv6host:::::1`:             `invalid IP`,
		`ipv6local:::0::`:              `invalid IP`,
	}

	for _, extrahost := range valid {
		if _, err := ValidateExtraHost(extrahost); err != nil {
			t.Fatalf("ValidateExtraHost(`"+extrahost+"`) should succeed: error %v", err)
		}
	}

	for extraHost, expectedError := range invalid {
		if _, err := ValidateExtraHost(extraHost); err == nil {
			t.Fatalf("ValidateExtraHost(`%q`) should have failed validation", extraHost)
		} else {
			if !strings.Contains(err.Error(), expectedError) {
				t.Fatalf("ValidateExtraHost(`%q`) error should contain %q", extraHost, expectedError)
			}
		}
	}
}

func TestIpOptString(t *testing.T) {
	addresses := []string{"", "0.0.0.0"}
	var ip net.IP

	for _, address := range addresses {
		stringAddress := NewIpOpt(&ip, address).String()
		if stringAddress != address {
			t.Fatalf("IpOpt string should be `%s`, not `%s`", address, stringAddress)
		}
	}
}

func logOptsValidator(val string) (string, error) {
	allowedKeys := map[string]string{"max-size": "1", "max-file": "2"}
	vals := strings.Split(val, "=")
	if allowedKeys[vals[0]] != "" {
		return val, nil
	}
	return "", fmt.Errorf("invalid key %s", vals[0])
}
