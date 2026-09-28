package etchosts

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func emptyFile(t testing.TB) string {
	t.Helper()
	tmpFile := filepath.Join(t.TempDir(), "etchosts")
	assert.NilError(t, os.WriteFile(tmpFile, nil, 0o644))
	return tmpFile
}

func TestBuildDefault(t *testing.T) {
	tmpFile := emptyFile(t)

	// check that /etc/hosts has consistent ordering
	for i := 0; i <= 5; i++ {
		assert.NilError(t, Build(tmpFile, nil))

		content, err := os.ReadFile(tmpFile)
		assert.NilError(t, err)
		expected := "127.0.0.1\tlocalhost\n::1\tlocalhost ip6-localhost ip6-loopback\nfe00::\tip6-localnet\nff00::\tip6-mcastprefix\nff02::1\tip6-allnodes\nff02::2\tip6-allrouters\n"

		actual := string(content)
		assert.Check(t, is.Equal(actual, expected))
	}
}

func TestBuildNoIPv6(t *testing.T) {
	d := t.TempDir()
	filename := filepath.Join(d, "hosts")

	err := BuildNoIPv6(filename, []Record{
		{
			Hosts: "another.example",
			IP:    netip.MustParseAddr("fdbb:c59c:d015::3"),
		},
		{
			Hosts: "another.example",
			IP:    netip.MustParseAddr("10.11.12.13"),
		},
	})
	assert.NilError(t, err)
	content, err := os.ReadFile(filename)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(string(content), "127.0.0.1\tlocalhost\n10.11.12.13\tanother.example\n"))
}

func TestUpdate(t *testing.T) {
	tmpFile := emptyFile(t)

	err := Build(tmpFile, []Record{{
		Hosts: "testhostname.testdomainname testhostname",
		IP:    netip.MustParseAddr("10.11.12.13"),
	}})
	assert.NilError(t, err)

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	if expected := "10.11.12.13\ttesthostname.testdomainname testhostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	err = Update(tmpFile, "1.1.1.1", "testhostname")
	assert.NilError(t, err)

	content, err = os.ReadFile(tmpFile)
	assert.NilError(t, err)

	if expected := "1.1.1.1\ttesthostname.testdomainname testhostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

// This regression test ensures that when a host is given a new IP
// via the Update function that other hosts which start with the
// same name as the targeted host are not erroneously updated as well.
// In the test example, if updating a host called "prefix", unrelated
// hosts named "prefixAndMore" or "prefix2" or anything else starting
// with "prefix" should not be changed. For more information see
// GitHub issue #603.
func TestUpdateIgnoresPrefixedHostname(t *testing.T) {
	tmpFile := emptyFile(t)
	err := Build(tmpFile, []Record{
		{
			Hosts: "prefix",
			IP:    netip.MustParseAddr("2.2.2.2"),
		},
		{
			Hosts: "prefixAndMore",
			IP:    netip.MustParseAddr("3.3.3.3"),
		},
		{
			Hosts: "unaffectedHost",
			IP:    netip.MustParseAddr("4.4.4.4"),
		},
	})
	assert.NilError(t, err)

	content, err := os.ReadFile(tmpFile)
	assert.NilError(t, err)

	if expected := "2.2.2.2\tprefix\n3.3.3.3\tprefixAndMore\n4.4.4.4\tunaffectedHost\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	err = Update(tmpFile, "5.5.5.5", "prefix")
	assert.NilError(t, err)

	content, err = os.ReadFile(tmpFile)
	assert.NilError(t, err)

	if expected := "5.5.5.5\tprefix\n3.3.3.3\tprefixAndMore\n4.4.4.4\tunaffectedHost\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

// This regression test covers the host prefix issue for the
// Delete function. In the test example, if deleting a host called
// "prefix", an unrelated host called "prefixAndMore" should not
// be deleted. For more information see GitHub issue #603.
func TestDeleteIgnoresPrefixedHostname(t *testing.T) {
	tmpFile := emptyFile(t)

	err := Build(tmpFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(tmpFile, []Record{
		{
			Hosts: "prefix",
			IP:    netip.MustParseAddr("1.1.1.1"),
		},
		{
			Hosts: "prefixAndMore",
			IP:    netip.MustParseAddr("2.2.2.2"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := Delete(tmpFile, []Record{
		{
			Hosts: "prefix",
			IP:    netip.MustParseAddr("1.1.1.1"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	if expected := "2.2.2.2\tprefixAndMore\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	if expected := "1.1.1.1\tprefix\n"; bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Did not expect to find '%s' got '%s'", expected, content)
	}
}

func TestAddEmpty(t *testing.T) {
	tmpFile := emptyFile(t)
	assert.NilError(t, Build(tmpFile, nil))
	assert.NilError(t, Add(tmpFile, []Record{}))
}

func TestAdd(t *testing.T) {
	tmpFile := emptyFile(t)
	assert.NilError(t, Build(tmpFile, nil))

	err := Add(tmpFile, []Record{{
		Hosts: "testhostname",
		IP:    netip.MustParseAddr("2.2.2.2"),
	}})
	assert.NilError(t, err)

	content, err := os.ReadFile(tmpFile)
	assert.NilError(t, err)

	if expected := "2.2.2.2\ttesthostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestDeleteEmpty(t *testing.T) {
	tmpFile := emptyFile(t)
	assert.NilError(t, Build(tmpFile, nil))
	assert.NilError(t, Delete(tmpFile, []Record{}))
}

func TestDeleteNewline(t *testing.T) {
	tmpFile := emptyFile(t)
	assert.NilError(t, os.WriteFile(tmpFile, []byte("\n"), 0o644))

	err := Delete(tmpFile, []Record{{
		Hosts: "prefix",
		IP:    netip.MustParseAddr("2.2.2.2"),
	}})
	assert.NilError(t, err)
}

func TestDelete(t *testing.T) {
	tmpFile := emptyFile(t)
	assert.NilError(t, Build(tmpFile, nil))

	err := Add(tmpFile, []Record{
		{
			Hosts: "testhostname1",
			IP:    netip.MustParseAddr("1.1.1.1"),
		},
		{
			Hosts: "testhostname2",
			IP:    netip.MustParseAddr("2.2.2.2"),
		},
		{
			Hosts: "testhostname3",
			IP:    netip.MustParseAddr("3.3.3.3"),
		},
	})
	assert.NilError(t, err)

	err = Delete(tmpFile, []Record{
		{
			Hosts: "testhostname1",
			IP:    netip.MustParseAddr("1.1.1.1"),
		},
		{
			Hosts: "testhostname3",
			IP:    netip.MustParseAddr("3.3.3.3"),
		},
	})
	assert.NilError(t, err)

	content, err := os.ReadFile(tmpFile)
	assert.NilError(t, err)

	if expected := "2.2.2.2\ttesthostname2\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	if expected := "1.1.1.1\ttesthostname1\n"; bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Did not expect to find '%s' got '%s'", expected, content)
	}
}

func TestConcurrentWrites(t *testing.T) {
	tmpFile := emptyFile(t)
	assert.NilError(t, Build(tmpFile, nil))

	err := Add(tmpFile, []Record{{
		Hosts: "inithostname",
		IP:    netip.MustParseAddr("172.17.0.1"),
	}})
	assert.NilError(t, err)

	group := new(errgroup.Group)
	for i := range byte(10) {
		group.Go(func() error {
			addr, ok := netip.AddrFromSlice([]byte{i, i, i, i})
			assert.Assert(t, ok)

			rec := []Record{{
				IP:    addr,
				Hosts: fmt.Sprintf("testhostname%d", i),
			}}

			for range 25 {
				assert.NilError(t, Add(tmpFile, rec))
				assert.NilError(t, Delete(tmpFile, rec))
			}
			return nil
		})
	}

	err = group.Wait()
	assert.NilError(t, err)

	content, err := os.ReadFile(tmpFile)
	assert.NilError(t, err)

	if expected := "172.17.0.1\tinithostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func BenchmarkDelete(b *testing.B) {
	for b.Loop() {
		b.StopTimer()

		var records, toDelete []Record
		for i := range byte(255) {
			addr := netip.AddrFrom4([4]byte{i, i, i, i})
			record := Record{
				Hosts: fmt.Sprintf("testhostname%d", i),
				IP:    addr,
			}
			records = append(records, record)
			if i%2 == 0 {
				toDelete = append(toDelete, record)
			}
		}

		tmpFile := emptyFile(b)
		assert.NilError(b, Build(tmpFile, nil))
		assert.NilError(b, Add(tmpFile, records))

		b.StartTimer()

		assert.NilError(b, Delete(tmpFile, toDelete))
	}
}
