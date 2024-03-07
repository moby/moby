package resolvconf

import (
	"bytes"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/internal/sliceutil"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
)

func TestRCOption(t *testing.T) {
	testcases := []struct {
		name     string
		options  string
		search   string
		expFound bool
		expValue string
	}{
		{
			name:    "Empty options",
			options: "",
			search:  "ndots",
		},
		{
			name:    "Not found",
			options: "ndots:0 edns0",
			search:  "trust-ad",
		},
		{
			name:     "Found with value",
			options:  "ndots:0 edns0",
			search:   "ndots",
			expFound: true,
			expValue: "0",
		},
		{
			name:     "Found without value",
			options:  "ndots:0 edns0",
			search:   "edns0",
			expFound: true,
			expValue: "",
		},
		{
			name:     "Found last value",
			options:  "ndots:0 edns0 ndots:1",
			search:   "ndots",
			expFound: true,
			expValue: "1",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			rc, err := Parse(bytes.NewBuffer([]byte("options "+tc.options)), "")
			assert.NilError(t, err)
			value, found := rc.Option(tc.search)
			assert.Check(t, is.Equal(found, tc.expFound))
			assert.Check(t, is.Equal(value, tc.expValue))
		})
	}
}

func TestRCWrite(t *testing.T) {
	testcases := []struct {
		name            string
		fileName        string
		perm            os.FileMode
		hashFileName    string
		modify          bool
		expUserModified bool
	}{
		{
			name:         "Write with hash",
			fileName:     "testfile",
			hashFileName: "testfile.hash",
		},
		{
			name:            "Write with hash and modify",
			fileName:        "testfile",
			hashFileName:    "testfile.hash",
			modify:          true,
			expUserModified: true,
		},
		{
			name:            "Write without hash and modify",
			fileName:        "testfile",
			modify:          true,
			expUserModified: false,
		},
		{
			name:     "Write perm",
			fileName: "testfile",
			perm:     0640,
		},
	}

	rc, err := Parse(bytes.NewBuffer([]byte("nameserver 1.2.3.4")), "")
	assert.NilError(t, err)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			d := t.TempDir()
			path := filepath.Join(d, tc.fileName)
			var hashPath string
			if tc.hashFileName != "" {
				hashPath = filepath.Join(d, tc.hashFileName)
			}
			if tc.perm == 0 {
				tc.perm = 0644
			}
			err := rc.WriteFile(path, hashPath, tc.perm)
			assert.NilError(t, err)

			fi, err := os.Stat(path)
			assert.NilError(t, err)
			// Windows files won't have the expected perms.
			if runtime.GOOS != "windows" {
				assert.Check(t, is.Equal(fi.Mode(), tc.perm))
			}

			if tc.modify {
				err := os.WriteFile(path, []byte("modified"), 0644)
				assert.NilError(t, err)
			}

			um, err := UserModified(path, hashPath)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(um, tc.expUserModified))
		})
	}
}

var a2s = sliceutil.Mapper(netip.Addr.String)
var s2a = sliceutil.Mapper(netip.MustParseAddr)

// Test that a resolv.conf file can be modified using OverrideXXX() methods
// to modify nameservers/search/options directives, and tha options can be
// added via AddOption().
func TestRCModify(t *testing.T) {
	testcases := []struct {
		name            string
		inputNS         []string
		inputSearch     []string
		inputOptions    []string
		noOverrides     bool // Whether to apply overrides (empty lists are valid overrides).
		overrideNS      []string
		overrideSearch  []string
		overrideOptions []string
		addOption       string
	}{
		{
			name:    "No content no overrides",
			inputNS: []string{},
		},
		{
			name:         "No overrides",
			noOverrides:  true,
			inputNS:      []string{"1.2.3.4"},
			inputSearch:  []string{"invalid"},
			inputOptions: []string{"ndots:0"},
		},
		{
			name:         "Empty overrides",
			inputNS:      []string{"1.2.3.4"},
			inputSearch:  []string{"invalid"},
			inputOptions: []string{"ndots:0"},
		},
		{
			name:            "Overrides",
			inputNS:         []string{"1.2.3.4"},
			inputSearch:     []string{"invalid"},
			inputOptions:    []string{"ndots:0"},
			overrideNS:      []string{"2.3.4.5", "fdba:acdd:587c::53"},
			overrideSearch:  []string{"com", "invalid", "example"},
			overrideOptions: []string{"ndots:1", "edns0", "trust-ad"},
		},
		{
			name:         "Add option no overrides",
			noOverrides:  true,
			inputNS:      []string{"1.2.3.4"},
			inputSearch:  []string{"invalid"},
			inputOptions: []string{"ndots:0"},
			addOption:    "attempts:3",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			var input string
			if len(tc.inputNS) != 0 {
				for _, ns := range tc.inputNS {
					input += "nameserver " + ns + "\n"
				}
			}
			if len(tc.inputSearch) != 0 {
				input += "search " + strings.Join(tc.inputSearch, " ") + "\n"
			}
			if len(tc.inputOptions) != 0 {
				input += "options " + strings.Join(tc.inputOptions, " ") + "\n"
			}
			rc, err := Parse(bytes.NewBuffer([]byte(input)), "")
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(a2s(rc.NameServers()), tc.inputNS))
			assert.Check(t, is.DeepEqual(rc.Search(), tc.inputSearch))
			assert.Check(t, is.DeepEqual(rc.Options(), tc.inputOptions))

			if !tc.noOverrides {
				overrideNS := s2a(tc.overrideNS)
				rc.OverrideNameServers(overrideNS)
				rc.OverrideSearch(tc.overrideSearch)
				rc.OverrideOptions(tc.overrideOptions)

				assert.Check(t, is.DeepEqual(rc.NameServers(), overrideNS, cmpopts.EquateEmpty(), cmpopts.EquateComparable(netip.Addr{})))
				assert.Check(t, is.DeepEqual(rc.Search(), tc.overrideSearch, cmpopts.EquateEmpty()))
				assert.Check(t, is.DeepEqual(rc.Options(), tc.overrideOptions, cmpopts.EquateEmpty()))
			}

			if tc.addOption != "" {
				options := rc.Options()
				rc.AddOption(tc.addOption)
				assert.Check(t, is.DeepEqual(rc.Options(), append(options, tc.addOption), cmpopts.EquateEmpty()))
			}

			d := t.TempDir()
			path := filepath.Join(d, "resolv.conf")
			err = rc.WriteFile(path, "", 0644)
			assert.NilError(t, err)

			content, err := os.ReadFile(path)
			assert.NilError(t, err)
			assert.Check(t, golden.String(string(content), t.Name()+".golden"))
		})
	}
}

func TestRCTransformForLegacyNw(t *testing.T) {
	testcases := []struct {
		name       string
		input      string
		ipv6       bool
		overrideNS []string
	}{
		{
			name:  "Routable IPv4 only",
			input: "nameserver 10.0.0.1",
		},
		{
			name:  "Routable IPv4 and IPv6, ipv6 enabled",
			input: "nameserver 10.0.0.1\nnameserver fdb6:b8fe:b528::1",
			ipv6:  true,
		},
		{
			name:  "Routable IPv4 and IPv6, ipv6 disabled",
			input: "nameserver 10.0.0.1\nnameserver fdb6:b8fe:b528::1",
			ipv6:  false,
		},
		{
			name:  "IPv4 localhost, ipv6 disabled",
			input: "nameserver 127.0.0.53",
			ipv6:  false,
		},
		{
			name:  "IPv4 localhost, ipv6 enabled",
			input: "nameserver 127.0.0.53",
			ipv6:  true,
		},
		{
			name:  "IPv4 and IPv6 localhost, ipv6 disabled",
			input: "nameserver 127.0.0.53\nnameserver ::1",
			ipv6:  false,
		},
		{
			name:  "IPv4 and IPv6 localhost, ipv6 enabled",
			input: "nameserver 127.0.0.53\nnameserver ::1",
			ipv6:  true,
		},
		{
			name:  "IPv4 localhost, IPv6 routeable, ipv6 enabled",
			input: "nameserver 127.0.0.53\nnameserver fd3e:2d1a:1f5a::1",
			ipv6:  true,
		},
		{
			name:  "IPv4 localhost, IPv6 routeable, ipv6 disabled",
			input: "nameserver 127.0.0.53\nnameserver fd3e:2d1a:1f5a::1",
			ipv6:  false,
		},
		{
			name:       "Override nameservers",
			input:      "nameserver 127.0.0.53",
			overrideNS: []string{"127.0.0.1", "::1"},
			ipv6:       false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			rc, err := Parse(bytes.NewBuffer([]byte(tc.input)), "/etc/resolv.conf")
			assert.NilError(t, err)
			if tc.overrideNS != nil {
				rc.OverrideNameServers(s2a(tc.overrideNS))
			}

			rc.TransformForLegacyNw(tc.ipv6)

			d := t.TempDir()
			path := filepath.Join(d, "resolv.conf")
			err = rc.WriteFile(path, "", 0644)
			assert.NilError(t, err)

			content, err := os.ReadFile(path)
			assert.NilError(t, err)
			assert.Check(t, golden.String(string(content), t.Name()+".golden"))
		})
	}
}

func TestRCTransformForIntNS(t *testing.T) {
	mke := func(addr string, hostLoopback bool) ExtDNSEntry {
		return ExtDNSEntry{
			Addr:         netip.MustParseAddr(addr),
			HostLoopback: hostLoopback,
		}
	}

	testcases := []struct {
		name            string
		input           string
		intNameServer   string
		ipv6            bool
		overrideNS      []string
		overrideOptions []string
		reqdOptions     []string
		expExtServers   []ExtDNSEntry
		expErr          string
	}{
		{
			name:          "IPv4 only",
			input:         "nameserver 10.0.0.1",
			expExtServers: []ExtDNSEntry{mke("10.0.0.1", false)},
		},
		{
			name:          "IPv4 and IPv6, ipv6 enabled",
			input:         "nameserver 10.0.0.1\nnameserver fdb6:b8fe:b528::1",
			ipv6:          true,
			expExtServers: []ExtDNSEntry{mke("10.0.0.1", false)},
		},
		{
			name:          "IPv4 and IPv6, ipv6 disabled",
			input:         "nameserver 10.0.0.1\nnameserver fdb6:b8fe:b528::1",
			ipv6:          false,
			expExtServers: []ExtDNSEntry{mke("10.0.0.1", false)},
		},
		{
			name:          "IPv4 localhost",
			input:         "nameserver 127.0.0.53",
			ipv6:          false,
			expExtServers: []ExtDNSEntry{mke("127.0.0.53", true)},
		},
		{
			// Overriding the nameserver with a localhost address means use the container's
			// loopback interface, not the host's.
			name:          "IPv4 localhost override",
			input:         "nameserver 10.0.0.1",
			ipv6:          false,
			overrideNS:    []string{"127.0.0.53"},
			expExtServers: []ExtDNSEntry{mke("127.0.0.53", false)},
		},
		{
			name:          "IPv4 localhost, ipv6 enabled",
			input:         "nameserver 127.0.0.53",
			ipv6:          true,
			expExtServers: []ExtDNSEntry{mke("127.0.0.53", true)},
		},
		{
			name:  "IPv6 addr, IPv6 enabled",
			input: "nameserver fd14:6e0e:f855::1",
			ipv6:  true,
			// Note that there are no ext servers in this case, the internal resolver
			// will only look up container names. The default nameservers aren't added
			// because the host's IPv6 nameserver remains in the container's resolv.conf,
			// (because only IPv4 ext servers are currently allowed).
		},
		{
			name:          "IPv4 and IPv6 localhost, IPv6 disabled",
			input:         "nameserver 127.0.0.53\nnameserver ::1",
			ipv6:          false,
			expExtServers: []ExtDNSEntry{mke("127.0.0.53", true)},
		},
		{
			name:          "IPv4 and IPv6 localhost, ipv6 enabled",
			input:         "nameserver 127.0.0.53\nnameserver ::1",
			ipv6:          true,
			expExtServers: []ExtDNSEntry{mke("127.0.0.53", true)},
		},
		{
			name:          "IPv4 localhost, IPv6 private, IPv6 enabled",
			input:         "nameserver 127.0.0.53\nnameserver fd3e:2d1a:1f5a::1",
			ipv6:          true,
			expExtServers: []ExtDNSEntry{mke("127.0.0.53", true)},
		},
		{
			name:          "IPv4 localhost, IPv6 private, IPv6 disabled",
			input:         "nameserver 127.0.0.53\nnameserver fd3e:2d1a:1f5a::1",
			ipv6:          false,
			expExtServers: []ExtDNSEntry{mke("127.0.0.53", true)},
		},
		{
			name:  "No host nameserver, no iv6",
			input: "",
			ipv6:  false,
			expExtServers: []ExtDNSEntry{
				mke("8.8.8.8", false),
				mke("8.8.4.4", false),
			},
		},
		{
			name:  "No host nameserver, iv6",
			input: "",
			ipv6:  true,
			expExtServers: []ExtDNSEntry{
				mke("8.8.8.8", false),
				mke("8.8.4.4", false),
				mke("2001:4860:4860::8888", false),
				mke("2001:4860:4860::8844", false),
			},
		},
		{
			name:          "ndots present and required",
			input:         "nameserver 127.0.0.53\noptions ndots:1",
			reqdOptions:   []string{"ndots:0"},
			expExtServers: []ExtDNSEntry{mke("127.0.0.53", true)},
		},
		{
			name:          "ndots missing but required",
			input:         "nameserver 127.0.0.53",
			reqdOptions:   []string{"ndots:0"},
			expExtServers: []ExtDNSEntry{mke("127.0.0.53", true)},
		},
		{
			name:            "ndots host, override and required",
			input:           "nameserver 127.0.0.53",
			reqdOptions:     []string{"ndots:0"},
			overrideOptions: []string{"ndots:2"},
			expExtServers:   []ExtDNSEntry{mke("127.0.0.53", true)},
		},
		{
			name:          "Extra required options",
			input:         "nameserver 127.0.0.53\noptions trust-ad",
			reqdOptions:   []string{"ndots:0", "attempts:3", "edns0", "trust-ad"},
			expExtServers: []ExtDNSEntry{mke("127.0.0.53", true)},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			rc, err := Parse(bytes.NewBuffer([]byte(tc.input)), "/etc/resolv.conf")
			assert.NilError(t, err)

			if tc.intNameServer == "" {
				tc.intNameServer = "127.0.0.11"
			}
			if len(tc.overrideNS) > 0 {
				rc.OverrideNameServers(s2a(tc.overrideNS))
			}
			if len(tc.overrideOptions) > 0 {
				rc.OverrideOptions(tc.overrideOptions)
			}
			intNS := netip.MustParseAddr(tc.intNameServer)
			extNameServers, err := rc.TransformForIntNS(tc.ipv6, intNS, tc.reqdOptions)
			if tc.expErr != "" {
				assert.Check(t, is.ErrorContains(err, tc.expErr))
				return
			}
			assert.NilError(t, err)

			d := t.TempDir()
			path := filepath.Join(d, "resolv.conf")
			err = rc.WriteFile(path, "", 0644)
			assert.NilError(t, err)

			content, err := os.ReadFile(path)
			assert.NilError(t, err)
			assert.Check(t, golden.String(string(content), t.Name()+".golden"))
			assert.Check(t, is.DeepEqual(extNameServers, tc.expExtServers,
				cmpopts.EquateComparable(netip.Addr{})))
		})
	}
}

// Check that invalid ndots options in the host's file are ignored, unless
// starting the internal resolver (which requires an ndots option), in which
// case invalid ndots should be replaced.
func TestRCTransformForIntNSInvalidNdots(t *testing.T) {
	testcases := []struct {
		name         string
		options      string
		reqdOptions  []string
		expVal       string
		expOptions   []string
		expNDotsFrom string
	}{
		{
			name:         "Negative value",
			options:      "options ndots:-1",
			expOptions:   []string{"ndots:-1"},
			expVal:       "-1",
			expNDotsFrom: "host",
		},
		{
			name:         "Invalid values with reqd ndots",
			options:      "options ndots:-1 foo:bar ndots ndots:",
			reqdOptions:  []string{"ndots:2"},
			expVal:       "2",
			expNDotsFrom: "internal",
			expOptions:   []string{"foo:bar", "ndots:2"},
		},
		{
			name:         "Valid value with reqd ndots",
			options:      "options ndots:1 foo:bar ndots ndots:",
			reqdOptions:  []string{"ndots:2"},
			expVal:       "1",
			expNDotsFrom: "host",
			expOptions:   []string{"ndots:1", "foo:bar"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			content := "nameserver 8.8.8.8\n" + tc.options
			rc, err := Parse(bytes.NewBuffer([]byte(content)), "/etc/resolv.conf")
			assert.NilError(t, err)
			_, err = rc.TransformForIntNS(false, netip.MustParseAddr("127.0.0.11"), tc.reqdOptions)
			assert.NilError(t, err)

			val, found := rc.Option("ndots")
			assert.Check(t, is.Equal(found, true))
			assert.Check(t, is.Equal(val, tc.expVal))
			assert.Check(t, is.Equal(rc.md.NDotsFrom, tc.expNDotsFrom))
			assert.Check(t, is.DeepEqual(rc.options, tc.expOptions))
		})
	}
}

func TestRCRead(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "resolv.conf")

	// Try to read a nonexistent file, equivalent to an empty file.
	_, err := Load(path)
	assert.Check(t, is.ErrorIs(err, fs.ErrNotExist))

	err = os.WriteFile(path, []byte("options edns0"), 0644)
	assert.NilError(t, err)

	// Read that file in the constructor.
	rc, err := Load(path)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(rc.Options(), []string{"edns0"}))

	// Pass in an os.File, check the path is extracted.
	file, err := os.Open(path)
	assert.NilError(t, err)
	defer file.Close()
	rc, err = Parse(file, "")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(rc.md.SourcePath, path))
}

func TestRCInvalidNS(t *testing.T) {
	d := t.TempDir()

	// A resolv.conf with an invalid nameserver address.
	rc, err := Parse(bytes.NewBuffer([]byte("nameserver 1.2.3.4.5")), "")
	assert.NilError(t, err)

	path := filepath.Join(d, "resolv.conf")
	err = rc.WriteFile(path, "", 0644)
	assert.NilError(t, err)

	content, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.Check(t, golden.String(string(content), t.Name()+".golden"))
}

func TestRCSetHeader(t *testing.T) {
	rc, err := Parse(bytes.NewBuffer([]byte("nameserver 127.0.0.53")), "/etc/resolv.conf")
	assert.NilError(t, err)

	rc.SetHeader("# This is a comment.")
	d := t.TempDir()
	path := filepath.Join(d, "resolv.conf")
	err = rc.WriteFile(path, "", 0644)
	assert.NilError(t, err)

	content, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.Check(t, golden.String(string(content), t.Name()+".golden"))
}

func TestRCUnknownDirectives(t *testing.T) {
	const input = `
something unexpected
nameserver 127.0.0.53
options ndots:1
unrecognised thing
`
	rc, err := Parse(bytes.NewBuffer([]byte(input)), "/etc/resolv.conf")
	assert.NilError(t, err)

	d := t.TempDir()
	path := filepath.Join(d, "resolv.conf")
	err = rc.WriteFile(path, "", 0644)
	assert.NilError(t, err)

	content, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.Check(t, golden.String(string(content), t.Name()+".golden"))
}
