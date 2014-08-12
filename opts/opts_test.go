package opts

import (
	"testing"
)

func TestvalidateIPAddress(t *testing.T) {
	if ret, err := validateIPAddress(`1.2.3.4`); err != nil || ret == "" {
		t.Fatalf("validateIPAddress(`1.2.3.4`) got %s %s", ret, err)
	}

	if ret, err := validateIPAddress(`127.0.0.1`); err != nil || ret == "" {
		t.Fatalf("validateIPAddress(`127.0.0.1`) got %s %s", ret, err)
	}

	if ret, err := validateIPAddress(`::1`); err != nil || ret == "" {
		t.Fatalf("validateIPAddress(`::1`) got %s %s", ret, err)
	}

	if ret, err := validateIPAddress(`127`); err == nil || ret != "" {
		t.Fatalf("validateIPAddress(`127`) got %s %s", ret, err)
	}

	if ret, err := validateIPAddress(`random invalid string`); err == nil || ret != "" {
		t.Fatalf("validateIPAddress(`random invalid string`) got %s %s", ret, err)
	}

}

func TestvalidateDnsSearch(t *testing.T) {
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
	}

	for _, domain := range valid {
		if ret, err := validateDnsSearch(domain); err != nil || ret == "" {
			t.Fatalf("validateDnsSearch(`"+domain+"`) got %s %s", ret, err)
		}
	}

	for _, domain := range invalid {
		if ret, err := validateDnsSearch(domain); err == nil || ret != "" {
			t.Fatalf("validateDnsSearch(`"+domain+"`) got %s %s", ret, err)
		}
	}
}
