package nftables

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
)

func testSetup(t *testing.T) func() {
	t.Helper()
	if !Enable() {
		// Make sure it didn't fail because of a bug in the text/template.
		assert.NilError(t, parseTemplate())
		// If this is not CI, skip.
		if _, ok := os.LookupEnv("CI"); !ok {
			t.Skip("Cannot enable nftables, no 'nft' command in $PATH ?")
		}
		// In CI, nft should always be installed, fail the test.
		t.Fatal("Failed to enable nftables")
	}
	cleanupContext := netnsutils.SetupTestOSContext(t)
	return func() {
		cleanupContext()
		disable()
	}
}

// disable undoes Enable
func disable() {
	incrementalUpdateTempl = nil
	nftPath = ""
	enableOnce = sync.Once{}
}

func applyAndCheck(t *testing.T, tbl TableRef, goldenFilename string) {
	t.Helper()
	err := tbl.Apply(context.Background())
	assert.Check(t, err)
	res := icmd.RunCommand("nft", "list", "ruleset")
	assert.Check(t, is.Equal(res.ExitCode, 0))
	golden.Assert(t, res.Combined(), goldenFilename)
}

func TestTable(t *testing.T) {
	defer testSetup(t)()

	tbl4, err := NewTable(IPv4, "ipv4_table")
	assert.NilError(t, err)
	tbl6, err := NewTable(IPv6, "ipv6_table")
	assert.NilError(t, err)

	assert.Check(t, is.Equal(tbl4.Family(), IPv4))
	assert.Check(t, is.Equal(tbl6.Family(), IPv6))

	// Update nftables and check what happened.
	applyAndCheck(t, tbl4, t.Name()+"_created4.golden")
	applyAndCheck(t, tbl6, t.Name()+"_created46.golden")
}

func TestChain(t *testing.T) {
	defer testSetup(t)()

	// Create a table.
	tbl, err := NewTable(IPv4, "this_is_a_table")
	assert.NilError(t, err)

	// Create a base chain.
	const bcName = "this_is_a_base_chain"
	bc1, err := tbl.BaseChain(bcName, BaseChainTypeFilter, BaseChainHookForward, BaseChainPriorityFilter+10)
	assert.NilError(t, err)

	// Check that it's an error to add a new base chain with the same name.
	_, err = tbl.BaseChain(bcName, BaseChainTypeNAT, BaseChainHookPrerouting, BaseChainPriorityDstNAT)
	assert.Check(t, is.ErrorContains(err, "already exists"))

	// Add a rule.
	err = bc1.AppendRule(0, "counter")
	assert.NilError(t, err)

	// Add a regular chain.
	const regularChainName = "this_is_a_regular_chain"
	_ = tbl.Chain(regularChainName)

	// Add a rule to the regular chain, use string formatting and a func retrieved
	// from the table.
	f := tbl.ChainUpdateFunc(regularChainName, true)
	err = f(0, "counter %s", "accept")
	assert.Check(t, err)

	// Fetch the base chain by name.
	bc1 = tbl.Chain(bcName)

	// Add another rule to the base chain, using the newly-retrieved handle.
	err = bc1.AppendRule(0, "jump %s", regularChainName)
	assert.Check(t, err)

	// Update nftables and check what happened.
	applyAndCheck(t, tbl, t.Name()+"_created.golden")

	// Delete a rule from the base chain.
	f = tbl.ChainUpdateFunc(bcName, false)
	err = f(0, "counter")
	assert.Check(t, err)

	// Check it's an error to delete that rule again. This time, call the delete
	// function directly on a newly retrieved handle.
	err = tbl.Chain(bcName).DeleteRule(0, "counter")
	assert.Check(t, is.ErrorContains(err, "does not exist"))

	// Update the base chain's policy.
	err = tbl.Chain(bcName).SetPolicy("drop")
	assert.Check(t, err)

	// Check it's an error to set a policy on a regular chain.
	err = tbl.Chain(regularChainName).SetPolicy("drop")
	assert.Check(t, is.ErrorContains(err, "not a base chain"))

	// Update nftables and check what happened.
	applyAndCheck(t, tbl, t.Name()+"_modified.golden")

	// Delete the base chain.
	err = tbl.DeleteChain(bcName)
	assert.Check(t, err)

	// Delete the regular chain.
	err = tbl.DeleteChain(regularChainName)
	assert.Check(t, err)

	// Check that it's an error to delete it again.
	err = tbl.DeleteChain(regularChainName)
	assert.Check(t, is.ErrorContains(err, "does not exist"))

	// Update nftables and check what happened.
	applyAndCheck(t, tbl, t.Name()+"_deleted.golden")
}

func TestChainRuleGroups(t *testing.T) {
	defer testSetup(t)()

	tbl, err := NewTable(IPv4, "testtable")
	assert.NilError(t, err)
	c := tbl.Chain("testchain")
	err = c.AppendRule(100, "hello100")
	assert.Check(t, err)
	err = c.AppendRule(200, "hello200")
	assert.Check(t, err)
	err = c.AppendRule(100, "hello101")
	assert.Check(t, err)
	err = c.AppendRule(200, "hello201")
	assert.Check(t, err)
	err = c.AppendRule(100, "hello102")
	assert.Check(t, err)

	assert.Check(t, is.DeepEqual(c.c.Rules(), []string{
		"hello100", "hello101", "hello102",
		"hello200", "hello201",
	}))
}

func TestVMap(t *testing.T) {
	defer testSetup(t)()

	// Create a table.
	tbl, err := NewTable(IPv6, "this_is_a_table")
	assert.NilError(t, err)

	// Create a verdict map.
	const mapName = "this_is_a_vmap"
	m := tbl.InterfaceVMap(mapName)

	// Add an element.
	err = m.AddElement("eth0", "return")
	assert.Check(t, err)

	// Check that it's an error to add the element again.
	err = m.AddElement("eth0", "return")
	assert.Check(t, is.ErrorContains(err, "already contains element"))

	// Fetch the existing vmap.
	m = tbl.InterfaceVMap(mapName)

	// Add another element.
	err = m.AddElement("eth1", "drop")
	assert.Check(t, err)

	// Update nftables and check what happened.
	applyAndCheck(t, tbl, t.Name()+"_created.golden")

	// Delete an element.
	err = m.DeleteElement("eth1")
	assert.Check(t, err)

	// Check it's an error to delete it again.
	err = m.DeleteElement("eth1")
	assert.Check(t, is.ErrorContains(err, "does not contain element"))

	// Update nftables and check what happened.
	applyAndCheck(t, tbl, t.Name()+"_deleted.golden")
}

func TestSet(t *testing.T) {
	defer testSetup(t)()

	// Create v4 and v6 tables.
	tbl4, err := NewTable(IPv4, "table4")
	assert.NilError(t, err)
	tbl6, err := NewTable(IPv6, "table6")
	assert.NilError(t, err)

	// Create a set in each table.
	s4 := tbl4.PrefixSet("set4")
	s6 := tbl6.PrefixSet("set6")

	// Add elements to each set.
	err = s4.AddElement("192.0.2.1/24")
	assert.Check(t, err)
	err = s6.AddElement("2001:db8::1/64")
	assert.Check(t, err)

	// Check it's an error to add those elements again.
	err = s4.AddElement("192.0.2.1/24")
	assert.Check(t, is.ErrorContains(err, "already contains element"))
	err = s6.AddElement("2001:db8::1/64")
	assert.Check(t, is.ErrorContains(err, "already contains element"))

	// Update nftables and check what happened.
	applyAndCheck(t, tbl4, t.Name()+"_created4.golden")
	applyAndCheck(t, tbl6, t.Name()+"_created46.golden")

	// Delete elements.
	err = s4.DeleteElement("192.0.2.1/24")
	assert.Check(t, err)
	err = s6.DeleteElement("2001:db8::1/64")
	assert.Check(t, err)

	// Check it's an error to delete those elements again.
	err = s4.DeleteElement("192.0.2.1/24")
	assert.Check(t, is.ErrorContains(err, "does not contain element"))
	err = s6.DeleteElement("2001:db8::1/64")
	assert.Check(t, is.ErrorContains(err, "does not contain element"))

	// Update nftables and check what happened.
	applyAndCheck(t, tbl4, t.Name()+"_deleted4.golden")
	applyAndCheck(t, tbl6, t.Name()+"_deleted46.golden")
}
