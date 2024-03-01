//go:build !windows

package resolvconf

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestGet(t *testing.T) {
	actual, err := Get()
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual.Content, expected) {
		t.Errorf("%s and GetResolvConf have different content.", Path())
	}
	hash := digest.FromBytes(expected)
	if !bytes.Equal(actual.Hash, []byte(hash)) {
		t.Errorf("%s and GetResolvConf have different hashes.", Path())
	}
}

func TestGetNameservers(t *testing.T) {
	for _, tc := range []struct {
		input  string
		result []string
	}{
		{
			input: ``,
		},
		{
			input: `search example.com`,
		},
		{
			input:  `  nameserver 1.2.3.4   `,
			result: []string{"1.2.3.4"},
		},
		{
			input: `
nameserver 1.2.3.4
nameserver 40.3.200.10
search example.com`,
			result: []string{"1.2.3.4", "40.3.200.10"},
		},
		{
			input: `nameserver 1.2.3.4
search example.com
nameserver 4.30.20.100`,
			result: []string{"1.2.3.4", "4.30.20.100"},
		},
		{
			input: `search example.com
nameserver 1.2.3.4
#nameserver 4.3.2.1`,
			result: []string{"1.2.3.4"},
		},
		{
			input: `search example.com
nameserver 1.2.3.4 # not 4.3.2.1`,
			result: []string{"1.2.3.4"},
		},
	} {
		test := GetNameservers([]byte(tc.input), IP)
		if !strSlicesEqual(test, tc.result) {
			t.Errorf("Wrong nameserver string {%s} should be %v. Input: %s", test, tc.result, tc.input)
		}
	}
}

func TestGetNameserversAsCIDR(t *testing.T) {
	for _, tc := range []struct {
		input  string
		result []string
	}{
		{
			input: ``,
		},
		{
			input: `search example.com`,
		},
		{
			input:  `  nameserver 1.2.3.4   `,
			result: []string{"1.2.3.4/32"},
		},
		{
			input: `
nameserver 1.2.3.4
nameserver 40.3.200.10
search example.com`,
			result: []string{"1.2.3.4/32", "40.3.200.10/32"},
		},
		{
			input: `nameserver 1.2.3.4
search example.com
nameserver 4.30.20.100`,
			result: []string{"1.2.3.4/32", "4.30.20.100/32"},
		},
		{
			input: `search example.com
nameserver 1.2.3.4
#nameserver 4.3.2.1`,
			result: []string{"1.2.3.4/32"},
		},
		{
			input: `search example.com
nameserver 1.2.3.4 # not 4.3.2.1`,
			result: []string{"1.2.3.4/32"},
		},
		{
			input:  `nameserver fd6f:c490:ec68::1`,
			result: []string{"fd6f:c490:ec68::1/128"},
		},
		{
			input:  `nameserver fe80::1234%eth0`,
			result: []string{"fe80::1234/128"},
		},
	} {
		test := GetNameserversAsCIDR([]byte(tc.input))
		if !strSlicesEqual(test, tc.result) {
			t.Errorf("Wrong nameserver string {%s} should be %v. Input: %s", test, tc.result, tc.input)
		}
	}
}

func TestGetSearchDomains(t *testing.T) {
	for _, tc := range []struct {
		input  string
		result []string
	}{
		{
			input: ``,
		},
		{
			input: `# ignored`,
		},
		{
			input:  `search example.com`,
			result: []string{"example.com"},
		},
		{
			input:  `search example.com # notignored`,
			result: []string{"example.com", "#", "notignored"},
		},
		{
			input:  `	  search	 example.com	  `,
			result: []string{"example.com"},
		},
		{
			input:  `	  search	 example.com	  # notignored`,
			result: []string{"example.com", "#", "notignored"},
		},
		{
			input:  `search foo.example.com example.com`,
			result: []string{"foo.example.com", "example.com"},
		},
		{
			input:  `	   search	   foo.example.com	 example.com	`,
			result: []string{"foo.example.com", "example.com"},
		},
		{
			input:  `	   search	   foo.example.com	 example.com	# notignored`,
			result: []string{"foo.example.com", "example.com", "#", "notignored"},
		},
		{
			input: `nameserver 1.2.3.4
search foo.example.com example.com`,
			result: []string{"foo.example.com", "example.com"},
		},
		{
			input: `nameserver 1.2.3.4
search dup1.example.com dup2.example.com
search foo.example.com example.com`,
			result: []string{"foo.example.com", "example.com"},
		},
		{
			input: `nameserver 1.2.3.4
search foo.example.com example.com
nameserver 4.30.20.100`,
			result: []string{"foo.example.com", "example.com"},
		},
		{
			input:  `domain an.example`,
			result: []string{"an.example"},
		},
	} {
		test := GetSearchDomains([]byte(tc.input))
		if !strSlicesEqual(test, tc.result) {
			t.Errorf("Wrong search domain string {%s} should be %v. Input: %s", test, tc.result, tc.input)
		}
	}
}

func TestGetOptions(t *testing.T) {
	for _, tc := range []struct {
		input  string
		result []string
	}{
		{
			input: ``,
		},
		{
			input: `# ignored`,
		},
		{
			input: `; ignored`,
		},
		{
			input: `nameserver 1.2.3.4`,
		},
		{
			input:  `options opt1`,
			result: []string{"opt1"},
		},
		{
			input:  `options opt1 # notignored`,
			result: []string{"opt1", "#", "notignored"},
		},
		{
			input:  `options opt1 ; notignored`,
			result: []string{"opt1", ";", "notignored"},
		},
		{
			input:  `	  options	 opt1	  `,
			result: []string{"opt1"},
		},
		{
			input:  `	  options	 opt1	  # notignored`,
			result: []string{"opt1", "#", "notignored"},
		},
		{
			input:  `options opt1 opt2 opt3`,
			result: []string{"opt1", "opt2", "opt3"},
		},
		{
			input:  `options opt1 opt2 opt3 # notignored`,
			result: []string{"opt1", "opt2", "opt3", "#", "notignored"},
		},
		{
			input:  `	   options	 opt1	 opt2	 opt3	`,
			result: []string{"opt1", "opt2", "opt3"},
		},
		{
			input:  `	   options	 opt1	 opt2	 opt3	# notignored`,
			result: []string{"opt1", "opt2", "opt3", "#", "notignored"},
		},
		{
			input: `nameserver 1.2.3.4
options opt1 opt2 opt3`,
			result: []string{"opt1", "opt2", "opt3"},
		},
		{
			input: `nameserver 1.2.3.4
options opt1 opt2
options opt3 opt4`,
			result: []string{"opt1", "opt2", "opt3", "opt4"},
		},
	} {
		test := GetOptions([]byte(tc.input))
		if !strSlicesEqual(test, tc.result) {
			t.Errorf("Wrong options string {%s} should be %v. Input: %s", test, tc.result, tc.input)
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
	tmpDir := t.TempDir()
	file, err := os.CreateTemp(tmpDir, "")
	if err != nil {
		t.Fatal(err)
	}

	f, err := Build(file.Name(), []string{"ns1", "ns2", "ns3"}, []string{"search1"}, []string{"opt1"})
	if err != nil {
		t.Fatal(err)
	}

	const expected = "search search1\nnameserver ns1\nnameserver ns2\nnameserver ns3\noptions opt1\n"
	if !bytes.Equal(f.Content, []byte(expected)) {
		t.Errorf("Expected to find '%s' got '%s'", expected, f.Content)
	}
	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, []byte(expected)) {
		t.Errorf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestBuildWithZeroLengthDomainSearch(t *testing.T) {
	tmpDir := t.TempDir()
	file, err := os.CreateTemp(tmpDir, "")
	if err != nil {
		t.Fatal(err)
	}

	f, err := Build(file.Name(), []string{"ns1", "ns2", "ns3"}, []string{"."}, []string{"opt1"})
	if err != nil {
		t.Fatal(err)
	}

	const expected = "nameserver ns1\nnameserver ns2\nnameserver ns3\noptions opt1\n"
	if !bytes.Equal(f.Content, []byte(expected)) {
		t.Errorf("Expected to find '%s' got '%s'", expected, f.Content)
	}
	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, []byte(expected)) {
		t.Errorf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestBuildWithNoOptions(t *testing.T) {
	tmpDir := t.TempDir()
	file, err := os.CreateTemp(tmpDir, "")
	if err != nil {
		t.Fatal(err)
	}

	f, err := Build(file.Name(), []string{"ns1", "ns2", "ns3"}, []string{"search1"}, []string{})
	if err != nil {
		t.Fatal(err)
	}

	const expected = "search search1\nnameserver ns1\nnameserver ns2\nnameserver ns3\n"
	if !bytes.Equal(f.Content, []byte(expected)) {
		t.Errorf("Expected to find '%s' got '%s'", expected, f.Content)
	}
	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, []byte(expected)) {
		t.Errorf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestFilterResolvDNS(t *testing.T) {
	testcases := []struct {
		name        string
		input       string
		ipv6Enabled bool
		expOut      string
	}{
		{
			name:   "No localhost",
			input:  "nameserver 10.16.60.14\nnameserver 10.16.60.21\n",
			expOut: "nameserver 10.16.60.14\nnameserver 10.16.60.21",
		},
		{
			name:   "Localhost last",
			input:  "nameserver 10.16.60.14\nnameserver 10.16.60.21\nnameserver 127.0.0.1\n",
			expOut: "nameserver 10.16.60.14\nnameserver 10.16.60.21",
		},
		{
			name:   "Localhost middle",
			input:  "nameserver 10.16.60.14\nnameserver 127.0.0.1\nnameserver 10.16.60.21\n",
			expOut: "nameserver 10.16.60.14\nnameserver 10.16.60.21",
		},
		{
			name:   "Localhost first",
			input:  "nameserver 127.0.1.1\nnameserver 10.16.60.14\nnameserver 10.16.60.21\n",
			expOut: "nameserver 10.16.60.14\nnameserver 10.16.60.21",
		},
		{
			name:   "IPv6 Localhost",
			input:  "nameserver ::1\nnameserver 10.16.60.14\nnameserver 127.0.2.1\nnameserver 10.16.60.21\n",
			expOut: "nameserver 10.16.60.14\nnameserver 10.16.60.21",
		},
		{
			name:   "Two IPv6 Localhosts",
			input:  "nameserver 10.16.60.14\nnameserver ::1\nnameserver 10.16.60.21\nnameserver ::1",
			expOut: "nameserver 10.16.60.14\nnameserver 10.16.60.21",
		},
		{
			name:   "IPv6 disabled",
			input:  "nameserver 10.16.60.14\nnameserver 2002:dead:beef::1\nnameserver 10.16.60.21\nnameserver ::1",
			expOut: "nameserver 10.16.60.14\nnameserver 10.16.60.21",
		},
		{
			name:   "IPv6 link-local disabled",
			input:  "nameserver 10.16.60.14\nnameserver FE80::BB1%1\nnameserver FE80::BB1%eth0\nnameserver 10.16.60.21",
			expOut: "nameserver 10.16.60.14\nnameserver 10.16.60.21",
		},
		{
			name:        "IPv6 enabled",
			input:       "nameserver 10.16.60.14\nnameserver 2002:dead:beef::1\nnameserver 10.16.60.21\nnameserver ::1\n",
			ipv6Enabled: true,
			expOut:      "nameserver 10.16.60.14\nnameserver 2002:dead:beef::1\nnameserver 10.16.60.21",
		},
		{
			// with IPv6 enabled, and no non-localhost servers, Google defaults (both IPv4+IPv6) should be added
			name:        "localhost only IPv6",
			input:       "nameserver 127.0.0.1\nnameserver ::1\nnameserver 127.0.2.1",
			ipv6Enabled: true,
			expOut:      "nameserver 8.8.8.8\nnameserver 8.8.4.4\nnameserver 2001:4860:4860::8888\nnameserver 2001:4860:4860::8844",
		},
		{
			// with IPv6 disabled, and no non-localhost servers, Google defaults (only IPv4) should be added
			name:   "localhost only no IPv6",
			input:  "nameserver 127.0.0.1\nnameserver ::1\nnameserver 127.0.2.1",
			expOut: "nameserver 8.8.8.8\nnameserver 8.8.4.4",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := FilterResolvDNS([]byte(tc.input), tc.ipv6Enabled)
			assert.Check(t, is.Nil(err))
			out := strings.TrimSpace(string(f.Content))
			assert.Check(t, is.Equal(out, tc.expOut))
		})
	}
}
