package resolvconf

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestGet(t *testing.T) {
	resolvConfUtils, err := Get()
	if err != nil {
		t.Fatal(err)
	}
	resolvConfSystem, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		t.Fatal(err)
	}
	if string(resolvConfUtils) != string(resolvConfSystem) {
		t.Fatalf("/etc/resolv.conf and GetResolvConf have different content.")
	}
}

func TestGetNameservers(t *testing.T) {
	for resolv, result := range map[string][]string{`
nameserver 1.2.3.4
nameserver 40.3.200.10
search example.com`: {"1.2.3.4", "40.3.200.10"},
		`search example.com`: {},
		`nameserver 1.2.3.4
search example.com
nameserver 4.30.20.100`: {"1.2.3.4", "4.30.20.100"},
		``: {},
		`  nameserver 1.2.3.4   `: {"1.2.3.4"},
		`search example.com
nameserver 1.2.3.4
#nameserver 4.3.2.1`: {"1.2.3.4"},
		`search example.com
nameserver 1.2.3.4 # not 4.3.2.1`: {"1.2.3.4"},
	} {
		test := GetNameservers([]byte(resolv))
		if !strSlicesEqual(test, result) {
			t.Fatalf("Wrong nameserver string {%s} should be %v. Input: %s", test, result, resolv)
		}
	}
}

func TestGetNameserversAsCIDR(t *testing.T) {
	for resolv, result := range map[string][]string{`
nameserver 1.2.3.4
nameserver 40.3.200.10
search example.com`: {"1.2.3.4/32", "40.3.200.10/32"},
		`search example.com`: {},
		`nameserver 1.2.3.4
search example.com
nameserver 4.30.20.100`: {"1.2.3.4/32", "4.30.20.100/32"},
		``: {},
		`  nameserver 1.2.3.4   `: {"1.2.3.4/32"},
		`search example.com
nameserver 1.2.3.4
#nameserver 4.3.2.1`: {"1.2.3.4/32"},
		`search example.com
nameserver 1.2.3.4 # not 4.3.2.1`: {"1.2.3.4/32"},
	} {
		test := GetNameserversAsCIDR([]byte(resolv))
		if !strSlicesEqual(test, result) {
			t.Fatalf("Wrong nameserver string {%s} should be %v. Input: %s", test, result, resolv)
		}
	}
}

func TestGetSearchDomains(t *testing.T) {
	for resolv, result := range map[string][]string{
		`search example.com`:           {"example.com"},
		`search example.com # ignored`: {"example.com"},
		` 	  search 	 example.com 	  `: {"example.com"},
		` 	  search 	 example.com 	  # ignored`: {"example.com"},
		`search foo.example.com example.com`: {"foo.example.com", "example.com"},
		`	   search   	   foo.example.com 	 example.com 	`: {"foo.example.com", "example.com"},
		`	   search   	   foo.example.com 	 example.com 	# ignored`: {"foo.example.com", "example.com"},
		``:          {},
		`# ignored`: {},
		`nameserver 1.2.3.4
search foo.example.com example.com`: {"foo.example.com", "example.com"},
		`nameserver 1.2.3.4
search dup1.example.com dup2.example.com
search foo.example.com example.com`: {"foo.example.com", "example.com"},
		`nameserver 1.2.3.4
search foo.example.com example.com
nameserver 4.30.20.100`: {"foo.example.com", "example.com"},
	} {
		test := GetSearchDomains([]byte(resolv))
		if !strSlicesEqual(test, result) {
			t.Fatalf("Wrong search domain string {%s} should be %v. Input: %s", test, result, resolv)
		}
	}
}

func strSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}

	return true
}

func TestBuild(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), []string{"ns1", "ns2", "ns3"}, []string{"search1"})
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "nameserver ns1\nnameserver ns2\nnameserver ns3\nsearch search1\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestBuildWithZeroLengthDomainSearch(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), []string{"ns1", "ns2", "ns3"}, []string{"."})
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "nameserver ns1\nnameserver ns2\nnameserver ns3\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
	if notExpected := "search ."; bytes.Contains(content, []byte(notExpected)) {
		t.Fatalf("Expected to not find '%s' got '%s'", notExpected, content)
	}
}
