package nftables

import (
	"context"
	"os"
	"testing"

	"github.com/moby/moby/v2/internal/testutil/netnsutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
)

func testSetup(t *testing.T) func() {
	t.Helper()
	if err := Enable(); err != nil {
		// Make sure it didn't fail because of a bug in the text/template.
		assert.NilError(t, parseTemplate())
		// If this is not CI, skip.
		if _, ok := os.LookupEnv("CI"); !ok {
			t.Skip("Cannot enable nftables, no 'nft' command in $PATH ?")
		}
		// In CI, nft should always be installed, fail the test.
		t.Fatalf("Failed to enable nftables: %s", err)
	}
	cleanupContext := netnsutils.SetupTestOSContext(t)
	return func() {
		cleanupContext()
		Disable()
	}
}

func applyAndCheck(t *testing.T, tbl Table, tm Modifier, goldenFilename string) {
	t.Helper()
	err := tbl.Apply(context.Background(), tm)
	assert.Check(t, err)
	res := icmd.RunCommand("nft", "list", "table", string(tbl.Family()), tbl.Name())
	res.Assert(t, icmd.Success)
	golden.Assert(t, res.Combined(), goldenFilename)
}

func reloadAndCheck(t *testing.T, tbl Table, ipv Family, goldenFilename string) {
	t.Helper()
	err := tbl.Reload(context.Background())
	assert.Check(t, err)
	res := icmd.RunCommand("nft", "list", "table", string(ipv), tbl.t.Name)
	res.Assert(t, icmd.Success)
	golden.Assert(t, res.Combined(), goldenFilename)
}

func TestTable(t *testing.T) {
	defer testSetup(t)()

	tbl4, err := NewTable(IPv4, "ipv4_table")
	assert.NilError(t, err)
	defer tbl4.Close()
	tbl6, err := NewTable(IPv6, "ipv6_table")
	assert.NilError(t, err)
	defer tbl6.Close()

	// Update nftables and check what happened.
	applyAndCheck(t, tbl4, Modifier{}, t.Name()+"/created4.golden")
	applyAndCheck(t, tbl6, Modifier{}, t.Name()+"/created6.golden")
}

func TestChain(t *testing.T) {
	defer testSetup(t)()

	// Create a table.
	tbl, err := NewTable(IPv4, "this_is_a_table")
	assert.NilError(t, err)
	defer tbl.Close()

	// Create a base chain.
	const bcName = "this_is_a_base_chain"
	tm := Modifier{}
	bcDesc := BaseChain{
		Name:      bcName,
		ChainType: BaseChainTypeFilter,
		Hook:      BaseChainHookForward,
		Priority:  BaseChainPriorityFilter + 10,
		Policy:    "accept",
	}
	tm.Create(bcDesc)
	// Add a rule to the base chain.
	bcCounterRule := Rule{Chain: bcName, Group: 0, Rule: []string{"counter"}}
	tm.Create(bcCounterRule)

	// Add a regular chain.
	const regularChainName = "this_is_a_regular_chain"
	cDesc := Chain{Name: regularChainName}
	tm.Create(cDesc)
	// Add a rule to the regular chain.
	cRule := Rule{Chain: regularChainName, Group: 0, Rule: []string{"counter", "accept"}}
	tm.Create(cRule)

	// Add another rule to the base chain.
	bcJumpRule := Rule{Chain: bcName, Group: 0, Rule: []string{"jump", regularChainName}}
	tm.Create(bcJumpRule)

	// Update nftables and check what happened.
	applyAndCheck(t, tbl, tm, t.Name()+"/created.golden")

	// Delete a rule from the base chain.
	tm = Modifier{}
	tm.Delete(bcCounterRule)

	// Update nftables and check what happened.
	applyAndCheck(t, tbl, tm, t.Name()+"/modified.golden")

	// Delete the base chain.
	tm = Modifier{}
	tm.Delete(bcJumpRule)
	tm.Delete(bcDesc)
	tm.Delete(cRule)
	tm.Delete(cDesc)

	// Update nftables and check what happened.
	applyAndCheck(t, tbl, tm, t.Name()+"/deleted.golden")
}

func TestChainRuleGroups(t *testing.T) {
	defer testSetup(t)()

	tbl, err := NewTable(IPv4, "testtable")
	assert.NilError(t, err)
	defer tbl.Close()
	tm := Modifier{}
	chainName := "testchain"
	tm.Create(Chain{Name: chainName})
	tm.Create(Rule{Chain: chainName, Group: 100, Rule: []string{"iifname hello100 counter"}})
	tm.Create(Rule{Chain: chainName, Group: 200, Rule: []string{"iifname hello200 counter"}})
	tm.Create(Rule{Chain: chainName, Group: 100, Rule: []string{"iifname hello101 counter"}})
	tm.Create(Rule{Chain: chainName, Group: 200, Rule: []string{"iifname hello201 counter"}})
	tm.Create(Rule{Chain: chainName, Group: 100, Rule: []string{"iifname hello102 counter"}})
	applyAndCheck(t, tbl, tm, t.Name()+".golden")
}

func TestIgnoreExist(t *testing.T) {
	defer testSetup(t)()
	tbl, err := NewTable(IPv4, "this_is_a_table")
	assert.NilError(t, err)
	defer tbl.Close()
	tm := Modifier{}

	// Create a chain with a single rule, add the rule again but drop the duplicate.
	const chainName = "this_is_a_chain"
	tm.Create(Chain{Name: chainName})
	tm.Create(Rule{Chain: chainName, Rule: []string{"counter"}})
	tm.Create(Rule{Chain: chainName, Rule: []string{"counter"}, IgnoreExist: true})
	applyAndCheck(t, tbl, tm, t.Name()+"/created.golden")

	// Add the rule again, ignoring the duplicate, but in a modifier that has an
	// error - check that the existing rule isn't removed by rollback of this modifier.
	tmErr := Modifier{}
	tmErr.Create(Rule{Chain: chainName, Rule: []string{"counter"}, IgnoreExist: true})
	tmErr.Create(Rule{Chain: chainName})
	err = tbl.Apply(context.Background(), tm)
	assert.Check(t, err != nil, "Expected an error")

	// Reload, to flush table state.
	reloadAndCheck(t, tbl, IPv4, t.Name()+"/created.golden")

	// Delete the rule.
	tmDel := Modifier{}
	tmDel.Delete(Rule{Chain: chainName, Rule: []string{"counter"}})
	applyAndCheck(t, tbl, tmDel, t.Name()+"/deleted.golden")

	// Delete it again, in another chain that will roll back, to check it's not resurrected.
	tmReDel := Modifier{}
	tmReDel.Delete(Rule{Chain: chainName, Rule: []string{"counter"}, IgnoreExist: true})
	tmReDel.Create(Rule{Chain: chainName})
	err = tbl.Apply(context.Background(), tmReDel)
	assert.Check(t, err != nil, "Expected an error")

	// Reload, to flush table state.
	reloadAndCheck(t, tbl, IPv4, t.Name()+"/deleted.golden")
}

func TestVMap(t *testing.T) {
	defer testSetup(t)()

	// Create a table.
	tbl, err := NewTable(IPv6, "this_is_a_table")
	assert.NilError(t, err)
	defer tbl.Close()
	tm := Modifier{}

	// Create a verdict map.
	const mapName = "this_is_a_vmap"
	tm.Create(VMap{Name: mapName, ElementType: NftTypeIfname})
	tm.Create(VMapElement{VmapName: mapName, Key: "eth0", Verdict: "return"})
	tm.Create(VMapElement{VmapName: mapName, Key: "eth1", Verdict: "drop"})

	// Update nftables and check what happened.
	applyAndCheck(t, tbl, tm, t.Name()+"/created.golden")

	// Undo those changes by reversing the commands.
	tmRev := tm.Reverse()

	// Update nftables and check what happened.
	applyAndCheck(t, tbl, tmRev, t.Name()+"/deleted.golden")
}

func TestSet(t *testing.T) {
	defer testSetup(t)()

	// Create v4 and v6 tables.
	tbl4, err := NewTable(IPv4, "table4")
	assert.NilError(t, err)
	defer tbl4.Close()
	tbl6, err := NewTable(IPv6, "table6")
	assert.NilError(t, err)
	defer tbl6.Close()

	// Create a set in each table.
	const set4Name = "set4"
	tm4 := Modifier{}
	tm4.Create(Set{Name: set4Name, ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}})
	const set6Name = "set6"
	tm6 := Modifier{}
	tm6.Create(Set{Name: set6Name, ElementType: NftTypeIPv6Addr, Flags: []string{"interval"}})

	// Add elements to each set.
	tm4.Create(SetElement{SetName: set4Name, Element: "192.0.2.0/24"})
	tm6.Create(SetElement{SetName: set6Name, Element: "2001:db8::/64"})

	// Update nftables and check what happened.
	applyAndCheck(t, tbl4, tm4, t.Name()+"/created4.golden")
	applyAndCheck(t, tbl6, tm6, t.Name()+"/created6.golden")

	// Delete elements.
	applyAndCheck(t, tbl4, tm4.Reverse(), t.Name()+"/deleted4.golden")
	applyAndCheck(t, tbl6, tm6.Reverse(), t.Name()+"/deleted6.golden")
}

func TestReload(t *testing.T) {
	defer testSetup(t)()

	// Create a table with some stuff in it.
	const tableName = "this_is_a_table"
	tbl, err := NewTable(IPv4, tableName)
	assert.NilError(t, err)
	defer tbl.Close()
	tm := Modifier{}

	const bcName = "a_base_chain"
	tm.Create(BaseChain{
		Name:      bcName,
		ChainType: BaseChainTypeFilter,
		Hook:      BaseChainHookForward,
		Priority:  BaseChainPriorityFilter,
		Policy:    "accept",
	})
	tm.Create(Rule{Chain: bcName, Group: 0, Rule: []string{"counter"}})

	const vmapName = "this_is_a_vmap"
	tm.Create(VMap{Name: vmapName, ElementType: NftTypeIfname})
	tm.Create(VMapElement{VmapName: vmapName, Key: "eth0", Verdict: "return"})
	tm.Create(VMapElement{VmapName: vmapName, Key: "eth1", Verdict: "return"})

	const setName = "this_is_a_set"
	tm.Create(Set{Name: setName, ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}})
	tm.Create(SetElement{SetName: setName, Element: "192.0.2.0/24"})

	applyAndCheck(t, tbl, tm, t.Name()+"/created.golden")

	// Delete the underlying nftables table.
	deleteTable := func() {
		t.Helper()
		res := icmd.RunCommand("nft", "delete", "table", string(IPv4), tableName)
		res.Assert(t, icmd.Success)
		res = icmd.RunCommand("nft", "list", "ruleset")
		res.Assert(t, icmd.Success)
		assert.Check(t, is.Equal(res.Combined(), ""))
	}
	deleteTable()

	// Reconstruct the nftables table.
	err = tbl.Reload(context.Background())
	assert.Check(t, err)
	res := icmd.RunCommand("nft", "list", "table", string(tbl.Family()), tbl.Name())
	res.Assert(t, icmd.Success)
	golden.Assert(t, res.Combined(), t.Name()+"/created.golden")

	// Delete again.
	deleteTable()

	// Check implicit/recovery reload - only deleting something that's gone missing
	// from a vmap/set will trigger this.
	tm = Modifier{}
	tm.Delete(SetElement{SetName: setName, Element: "192.0.2.0/24"})
	applyAndCheck(t, tbl, tm, t.Name()+"/recovered.golden")
}

func TestValidation(t *testing.T) {
	testcases := []struct {
		name   string
		cmds   []command
		expErr string
	}{
		// BaseChain
		{
			name: "create with missing base chain name",
			cmds: []command{
				{obj: BaseChain{ChainType: BaseChainTypeNAT, Hook: BaseChainHookPostrouting, Priority: BaseChainPrioritySrcNAT}},
			},
			expErr: "base chain must have a name",
		},
		{
			name: "create with missing base chain type",
			cmds: []command{
				{obj: BaseChain{Name: "achain", Hook: BaseChainHookPostrouting, Priority: BaseChainPrioritySrcNAT}},
			},
			expErr: "chain 'achain': fields ChainType and Hook are required",
		},
		{
			name: "create with missing base chain hook",
			cmds: []command{
				{obj: BaseChain{Name: "achain", ChainType: BaseChainTypeNAT, Priority: BaseChainPrioritySrcNAT}},
			},
			expErr: "chain 'achain': fields ChainType and Hook are required",
		},
		{
			name: "delete non-empty base chain",
			cmds: []command{
				{obj: BaseChain{
					Name: "achain", ChainType: BaseChainTypeNAT, Hook: BaseChainHookPostrouting, Priority: BaseChainPrioritySrcNAT,
				}},
				{obj: Rule{Chain: "achain", Group: 0, Rule: []string{"counter"}}},
				{
					obj: BaseChain{
						Name: "achain", ChainType: BaseChainTypeNAT, Hook: BaseChainHookPostrouting, Priority: BaseChainPrioritySrcNAT,
					},
					delete: true,
				},
			},
			expErr: "cannot delete chain 'achain', it is not empty",
		},
		// Chain
		{
			name: "duplicate chain",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Chain{Name: "achain"}},
			},
			expErr: "already exists",
		},
		{
			name: "delete missing chain",
			cmds: []command{
				{obj: Chain{Name: "achain"}, delete: true},
			},
			expErr: "does not exist",
		},
		{
			name: "missing chain name",
			cmds: []command{
				{obj: Chain{}},
			},
			expErr: "chain must have a name",
		},
		{
			name: "delete non-empty chain",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
				{obj: Chain{Name: "achain"}, delete: true},
			},
			expErr: "cannot delete chain 'achain', it is not empty",
		},
		// Rule
		{
			name: "bad rule",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"this is nonsense"}}},
			},
			expErr: "syntax error",
		},
		{
			name: "duplicate rule",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
			},
			expErr: "rule exists",
		},
		{
			name: "delete missing rule",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}, delete: true},
			},
			expErr: "does not exist",
		},
		{
			name: "duplicate rule delete",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}, delete: true},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}, delete: true},
			},
			expErr: "does not exist",
		},
		{
			name: "create rule with missing chain name",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Rule: []string{"counter"}}},
			},
			expErr: "chain '' does not exist",
		},
		{
			name: "delete rule with missing chain name",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Rule: []string{"counter"}}, delete: true},
			},
			expErr: "chain '' does not exist",
		},
		{
			name: "create rule with nonexistent chain",
			cmds: []command{
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
			},
			expErr: "chain 'achain' does not exist",
		},
		{
			name: "delete rule with nonexistent chain",
			cmds: []command{
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}, delete: true},
			},
			expErr: "chain 'achain' does not exist",
		},
		{
			name: "create rule with no rule",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain"}},
			},
			expErr: "cannot add empty rule",
		},
		{
			name: "delete rule with no rule",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain"}, delete: true},
			},
			expErr: "cannot delete empty rule",
		},
		{
			name: "bad rule mid-sequence",
			cmds: []command{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}, delete: true},
				{obj: Rule{Chain: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
			},
			expErr: "chain 'achain', cannot add empty rule",
		},
		// VMap
		{
			name: "duplicate vmap",
			cmds: []command{
				{obj: VMap{Name: "avmap", ElementType: NftTypeIfname}},
				{obj: VMap{Name: "avmap", ElementType: NftTypeIfname}},
			},
			expErr: "vmap 'avmap' already exists",
		},
		{
			name: "delete nonexistent vmap",
			cmds: []command{
				{obj: VMap{Name: "avmap", ElementType: NftTypeIfname}, delete: true},
			},
			expErr: "cannot delete vmap 'avmap', it does not exist",
		},
		{
			name:   "missing vmap name",
			cmds:   []command{{obj: VMap{ElementType: NftTypeIfname}}},
			expErr: "vmap must have a name",
		},
		{
			name:   "missing vmap element type",
			cmds:   []command{{obj: VMap{Name: "avmap"}}},
			expErr: "vmap 'avmap' has no element type",
		},
		{
			name: "delete non-empty vmap",
			cmds: []command{
				{obj: VMap{Name: "avmap", ElementType: NftTypeIfname}},
				{obj: VMapElement{VmapName: "avmap", Key: "eth0", Verdict: "drop"}},
				{obj: VMap{Name: "avmap", ElementType: NftTypeIfname}, delete: true},
			},
			expErr: "cannot delete vmap 'avmap', it contains 1 elements",
		},
		// VMapElement
		{
			name: "duplicate vmap element",
			cmds: []command{
				{obj: VMap{Name: "avmap", ElementType: NftTypeIfname}},
				{obj: VMapElement{VmapName: "avmap", Key: "eth0", Verdict: "drop"}},
				{obj: VMapElement{VmapName: "avmap", Key: "eth0", Verdict: "drop"}},
			},
			expErr: "verdict map 'avmap' already contains element 'eth0'",
		},
		{
			name: "add to vmap that does not exist",
			cmds: []command{
				{obj: VMapElement{VmapName: "avmap", Key: "eth0", Verdict: "drop"}},
			},
			expErr: "cannot add to vmap 'avmap', it does not exist",
		},
		{
			name: "delete nonexistent vmap element",
			cmds: []command{
				{obj: VMap{Name: "avmap", ElementType: NftTypeIfname}},
				{obj: VMapElement{VmapName: "avmap", Key: "eth0", Verdict: "drop"}, delete: true},
			},
			expErr: "verdict map 'avmap' does not contain element 'eth0'",
		},
		{
			name: "vmap element with no named vmap",
			cmds: []command{
				{obj: VMap{Name: "avmap", ElementType: NftTypeIfname}},
				{obj: VMapElement{Key: "eth0", Verdict: "drop"}},
			},
			expErr: "cannot add element to unnamed vmap",
		},
		{
			name: "vmap element with no key",
			cmds: []command{
				{obj: VMap{Name: "avmap", ElementType: NftTypeIfname}},
				{obj: VMapElement{VmapName: "avmap", Verdict: "drop"}},
			},
			expErr: "cannot add to vmap 'avmap', element must have key and verdict",
		},
		{
			name: "vmap element with no verdict",
			cmds: []command{
				{obj: VMap{Name: "avmap", ElementType: NftTypeIfname}},
				{obj: VMapElement{VmapName: "avmap", Key: "eth0"}},
			},
			expErr: "cannot add to vmap 'avmap', element must have key and verdict",
		},
		// Set
		{
			name: "duplicate set",
			cmds: []command{
				{obj: Set{Name: "aset", ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}},
				{obj: Set{Name: "aset", ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}},
			},
			expErr: "set 'aset' already exists",
		},
		{
			name: "delete nonexistent set",
			cmds: []command{
				{obj: Set{Name: "aset", ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}, delete: true},
			},
			expErr: "cannot delete set 'aset', it does not exist",
		},
		{
			name: "missing set name",
			cmds: []command{
				{obj: Set{ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}},
			},
			expErr: "set must have a name",
		},
		{
			name: "missing set element type",
			cmds: []command{
				{obj: Set{Name: "aset", Flags: []string{"interval"}}},
			},
			expErr: "set 'aset' must have a type",
		},
		{
			name: "delete non-empty set",
			cmds: []command{
				{obj: Set{Name: "aset", ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset", Element: "192.0.2.0/24"}},
				{obj: Set{Name: "aset", ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}, delete: true},
			},
			expErr: "cannot delete set 'aset', it contains 1 elements",
		},
		// SetElement
		{
			name: "duplicate set element",
			cmds: []command{
				{obj: Set{Name: "aset", ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset", Element: "192.0.2.0/24"}},
				{obj: SetElement{SetName: "aset", Element: "192.0.2.0/24"}},
			},
			expErr: "set 'aset' already contains element '192.0.2.0/24'",
		},
		{
			name: "delete nonexistent set element",
			cmds: []command{
				{obj: Set{Name: "aset", ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset", Element: "192.0.2.0/24"}, delete: true},
			},
			expErr: "cannot delete '192.0.2.0/24' from set 'aset', it does not exist",
		},
		{
			name: "add set element to unnamed set",
			cmds: []command{
				{obj: Set{Name: "aset", ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{Element: "192.0.2.0/24"}},
			},
			expErr: "cannot add to set '', it does not exist",
		},
		{
			name: "add set element with no element",
			cmds: []command{
				{obj: Set{Name: "aset", ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset"}},
			},
			expErr: "cannot add to set 'aset', element not specified",
		},
		{
			name: "mismatched set element type",
			cmds: []command{
				{obj: Set{Name: "aset", ElementType: NftTypeIPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset", Element: "2001:db8::/64"}},
			},
			expErr: "Address family for hostname not supported",
		},
	}

	testName := t.Name()
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			defer testSetup(t)()
			tbl, err := NewTable(IPv4, "tablename")
			assert.NilError(t, err)
			defer tbl.Close()
			tm := Modifier{cmds: tc.cmds}
			err = tbl.Apply(context.Background(), tm)
			assert.Check(t, err != nil, "expected error containing '%s'", tc.expErr)
			assert.Check(t, is.ErrorContains(err, tc.expErr))
			// Check the table wasn't created.
			res := icmd.RunCommand("nft", "list", "table", string(IPv4), "tablename")
			res.Assert(t, icmd.Expected{ExitCode: 1})
			// Check the empty table can be created (the Table structure is still healthy).
			applyAndCheck(t, tbl, Modifier{}, testName+"/empty.golden")
		})
	}
}
