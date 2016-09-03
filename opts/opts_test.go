package opts

import (
	"fmt"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestValidateIPAddress(c *check.C) {
	if ret, err := ValidateIPAddress(`1.2.3.4`); err != nil || ret == "" {
		c.Fatalf("ValidateIPAddress(`1.2.3.4`) got %s %s", ret, err)
	}

	if ret, err := ValidateIPAddress(`127.0.0.1`); err != nil || ret == "" {
		c.Fatalf("ValidateIPAddress(`127.0.0.1`) got %s %s", ret, err)
	}

	if ret, err := ValidateIPAddress(`::1`); err != nil || ret == "" {
		c.Fatalf("ValidateIPAddress(`::1`) got %s %s", ret, err)
	}

	if ret, err := ValidateIPAddress(`127`); err == nil || ret != "" {
		c.Fatalf("ValidateIPAddress(`127`) got %s %s", ret, err)
	}

	if ret, err := ValidateIPAddress(`random invalid string`); err == nil || ret != "" {
		c.Fatalf("ValidateIPAddress(`random invalid string`) got %s %s", ret, err)
	}

}

func (s *DockerSuite) TestMapOpts(c *check.C) {
	tmpMap := make(map[string]string)
	o := NewMapOpts(tmpMap, logOptsValidator)
	o.Set("max-size=1")
	if o.String() != "map[max-size:1]" {
		c.Errorf("%s != [map[max-size:1]", o.String())
	}

	o.Set("max-file=2")
	if len(tmpMap) != 2 {
		c.Errorf("map length %d != 2", len(tmpMap))
	}

	if tmpMap["max-file"] != "2" {
		c.Errorf("max-file = %s != 2", tmpMap["max-file"])
	}

	if tmpMap["max-size"] != "1" {
		c.Errorf("max-size = %s != 1", tmpMap["max-size"])
	}
	if o.Set("dummy-val=3") == nil {
		c.Errorf("validator is not being called")
	}
}

func (s *DockerSuite) TestListOptsWithoutValidator(c *check.C) {
	o := NewListOpts(nil)
	o.Set("foo")
	if o.String() != "[foo]" {
		c.Errorf("%s != [foo]", o.String())
	}
	o.Set("bar")
	if o.Len() != 2 {
		c.Errorf("%d != 2", o.Len())
	}
	o.Set("bar")
	if o.Len() != 3 {
		c.Errorf("%d != 3", o.Len())
	}
	if !o.Get("bar") {
		c.Error("o.Get(\"bar\") == false")
	}
	if o.Get("baz") {
		c.Error("o.Get(\"baz\") == true")
	}
	o.Delete("foo")
	if o.String() != "[bar bar]" {
		c.Errorf("%s != [bar bar]", o.String())
	}
	listOpts := o.GetAll()
	if len(listOpts) != 2 || listOpts[0] != "bar" || listOpts[1] != "bar" {
		c.Errorf("Expected [[bar bar]], got [%v]", listOpts)
	}
	mapListOpts := o.GetMap()
	if len(mapListOpts) != 1 {
		c.Errorf("Expected [map[bar:{}]], got [%v]", mapListOpts)
	}

}

func (s *DockerSuite) TestListOptsWithValidator(c *check.C) {
	// Re-using logOptsvalidator (used by MapOpts)
	o := NewListOpts(logOptsValidator)
	o.Set("foo")
	if o.String() != "[]" {
		c.Errorf("%s != []", o.String())
	}
	o.Set("foo=bar")
	if o.String() != "[]" {
		c.Errorf("%s != []", o.String())
	}
	o.Set("max-file=2")
	if o.Len() != 1 {
		c.Errorf("%d != 1", o.Len())
	}
	if !o.Get("max-file=2") {
		c.Error("o.Get(\"max-file=2\") == false")
	}
	if o.Get("baz") {
		c.Error("o.Get(\"baz\") == true")
	}
	o.Delete("max-file=2")
	if o.String() != "[]" {
		c.Errorf("%s != []", o.String())
	}
}

func (s *DockerSuite) TestValidateDNSSearch(c *check.C) {
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
		if ret, err := ValidateDNSSearch(domain); err != nil || ret == "" {
			c.Fatalf("ValidateDNSSearch(`"+domain+"`) got %s %s", ret, err)
		}
	}

	for _, domain := range invalid {
		if ret, err := ValidateDNSSearch(domain); err == nil || ret != "" {
			c.Fatalf("ValidateDNSSearch(`"+domain+"`) got %s %s", ret, err)
		}
	}
}

func (s *DockerSuite) TestValidateLabel(c *check.C) {
	if _, err := ValidateLabel("label"); err == nil || err.Error() != "bad attribute format: label" {
		c.Fatalf("Expected an error [bad attribute format: label], go %v", err)
	}
	if actual, err := ValidateLabel("key1=value1"); err != nil || actual != "key1=value1" {
		c.Fatalf("Expected [key1=value1], got [%v,%v]", actual, err)
	}
	// Validate it's working with more than one =
	if actual, err := ValidateLabel("key1=value1=value2"); err != nil {
		c.Fatalf("Expected [key1=value1=value2], got [%v,%v]", actual, err)
	}
	// Validate it's working with one more
	if actual, err := ValidateLabel("key1=value1=value2=value3"); err != nil {
		c.Fatalf("Expected [key1=value1=value2=value2], got [%v,%v]", actual, err)
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

func (s *DockerSuite) TestNamedListOpts(c *check.C) {
	var v []string
	o := NewNamedListOptsRef("foo-name", &v, nil)

	o.Set("foo")
	if o.String() != "[foo]" {
		c.Errorf("%s != [foo]", o.String())
	}
	if o.Name() != "foo-name" {
		c.Errorf("%s != foo-name", o.Name())
	}
	if len(v) != 1 {
		c.Errorf("expected foo to be in the values, got %v", v)
	}
}

func (s *DockerSuite) TestNamedMapOpts(c *check.C) {
	tmpMap := make(map[string]string)
	o := NewNamedMapOpts("max-name", tmpMap, nil)

	o.Set("max-size=1")
	if o.String() != "map[max-size:1]" {
		c.Errorf("%s != [map[max-size:1]", o.String())
	}
	if o.Name() != "max-name" {
		c.Errorf("%s != max-name", o.Name())
	}
	if _, exist := tmpMap["max-size"]; !exist {
		c.Errorf("expected map-size to be in the values, got %v", tmpMap)
	}
}
