package opts

import (
	"testing"
)

func TestValidateIP4(t *testing.T) {
	if ret, err := ValidateIp4Address(`1.2.3.4`); err != nil || ret == "" {
		t.Fatalf("ValidateIp4Address(`1.2.3.4`) got %s %s", ret, err)
	}

	if ret, err := ValidateIp4Address(`127.0.0.1`); err != nil || ret == "" {
		t.Fatalf("ValidateIp4Address(`127.0.0.1`) got %s %s", ret, err)
	}

	if ret, err := ValidateIp4Address(`127`); err == nil || ret != "" {
		t.Fatalf("ValidateIp4Address(`127`) got %s %s", ret, err)
	}

	if ret, err := ValidateIp4Address(`random invalid string`); err == nil || ret != "" {
		t.Fatalf("ValidateIp4Address(`random invalid string`) got %s %s", ret, err)
	}

}

func TestValidateDomain(t *testing.T) {
	valid := []string{
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
		`.`,
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
	}

	for _, domain := range valid {
		if ret, err := ValidateDomain(domain); err != nil || ret == "" {
			t.Fatalf("ValidateDomain(`"+domain+"`) got %s %s", ret, err)
		}
	}

	for _, domain := range invalid {
		if ret, err := ValidateDomain(domain); err == nil || ret != "" {
			t.Fatalf("ValidateDomain(`"+domain+"`) got %s %s", ret, err)
		}
	}
}
