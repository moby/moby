package nftables

import (
	"os"
	"testing"
	"time"

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

	incrementalUpdateFailedHook = func(applied string, err error) {
		t.Errorf("Incremental update failed\n%s\n---\n%s", applied, err)
	}
	return func() {
		incrementalUpdateFailedHook = nil
		cleanupContext()
		Disable()
	}
}

func applyAndCheck(t *testing.T, goldenFilename string, tbl *Table, changes ...Changes) {
	t.Helper()
	err := tbl.Apply(t.Context(), changes...)
	assert.Check(t, err)
	res := icmd.RunCommand("nft", "list", "table", string(tbl.Family()), tbl.Name())
	res.Assert(t, icmd.Success)
	golden.Assert(t, res.Combined(), goldenFilename)
}

func reloadAndCheck(t *testing.T, goldenFilename string, tbl *Table) {
	t.Helper()
	err := tbl.Reload(t.Context())
	assert.Check(t, err)
	res := icmd.RunCommand("nft", "list", "table", string(tbl.Family()), tbl.Name())
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
	applyAndCheck(t, t.Name()+"/created4.golden", tbl4, Modifier{})
	applyAndCheck(t, t.Name()+"/created6.golden", tbl6, Modifier{})
}

func TestTableClose(t *testing.T) {
	// No nftables setup needed, closing a table that's never been applied doesn't
	// run any "nft" commands.
	tbl, err := NewTable(IPv4, "this_is_a_table")
	assert.NilError(t, err)
	assert.Assert(t, tbl.IsValid())

	// Consumers hold references to the table, closing it must invalidate them all.
	ref := tbl

	assert.NilError(t, tbl.Close())
	assert.Check(t, !tbl.IsValid(), "closed table should not be valid")
	assert.Check(t, !ref.IsValid(), "reference to a closed table should not be valid")

	// Close is idempotent, and a nil *Table can be closed.
	assert.NilError(t, tbl.Close())
	var nilTbl *Table
	assert.NilError(t, nilTbl.Close())
	assert.Check(t, !nilTbl.IsValid())

	// A Table that didn't come from NewTable is not usable either.
	var zeroVal Table
	assert.Check(t, !zeroVal.IsValid())
	assert.NilError(t, zeroVal.Close())

	assert.Check(t, is.Equal(tbl.Name(), ""))
	assert.Check(t, is.Equal(tbl.Family(), Family("")))
}

func TestTableUseAfterClose(t *testing.T) {
	defer testSetup(t)()

	tbl, err := NewTable(IPv4, "this_is_a_table")
	assert.NilError(t, err)

	const chainName = "this_is_a_chain"
	var tm Modifier
	tm.Create(Chain{Name: chainName})
	assert.NilError(t, tbl.Apply(t.Context(), tm))

	ref := tbl
	assert.NilError(t, tbl.Close())

	// Operations on a closed table must be refused, rather than transparently
	// opening a new handle to the underlying nftables table.
	var tm2 Modifier
	tm2.Create(Rule{Chain: chainName, Rule: []string{"counter"}})
	assert.Check(t, is.ErrorIs(tbl.Apply(t.Context(), tm2), errTableClosed))
	assert.Check(t, is.ErrorIs(ref.Apply(t.Context(), tm2), errTableClosed))
	assert.Check(t, is.ErrorIs(tbl.Reload(t.Context()), errTableClosed))
	assert.Check(t, is.ErrorIs(tbl.SetBaseChainPolicy(t.Context(), chainName, BaseChainPolicyDrop),
		errTableClosed))
	// applyLock must be held to read nftHandle.
	tbl.t.applyLock.Lock()
	handle := tbl.t.nftHandle
	tbl.t.applyLock.Unlock()
	assert.Check(t, is.Nil(handle), "no nftables handle should have been opened")

	// The table itself is left alone by Close, but the rejected update must not
	// have reached it. (The table's name and family can't be read from the closed
	// handle.)
	res := icmd.RunCommand("nft", "list", "table", string(IPv4), "this_is_a_table")
	res.Assert(t, icmd.Success)
	golden.Assert(t, res.Combined(), t.Name()+".golden")

	// A nil *Table, and a Table that didn't come from NewTable, must be refused
	// rather than panicking on their unusable innards.
	var nilTbl *Table
	assert.Check(t, is.ErrorIs(nilTbl.Apply(t.Context(), tm2), errInvalidTable))
	assert.Check(t, is.ErrorIs(nilTbl.Reload(t.Context()), errInvalidTable))
	assert.Check(t, is.ErrorIs(nilTbl.SetBaseChainPolicy(t.Context(), chainName, BaseChainPolicyDrop),
		errInvalidTable))

	unusable := &Table{}
	assert.Check(t, is.ErrorIs(unusable.Apply(t.Context(), tm2), errInvalidTable))
	assert.Check(t, is.ErrorIs(unusable.Reload(t.Context()), errInvalidTable))
	assert.Check(t, is.ErrorIs(unusable.SetBaseChainPolicy(t.Context(), chainName, BaseChainPolicyDrop),
		errInvalidTable))
}

func TestSetBaseChainPolicy(t *testing.T) {
	defer testSetup(t)()

	tbl, err := NewTable(IPv4, "this_is_a_table")
	assert.NilError(t, err)
	defer tbl.Close()

	const bcName = "this_is_a_base_chain"
	var tm Modifier
	tm.Create(BaseChain{
		Name:      bcName,
		ChainType: BaseChainTypeFilter,
		Hook:      BaseChainHookForward,
		Priority:  BaseChainPriorityFilter,
		Policy:    BaseChainPolicyAccept,
	})
	const cName = "this_is_a_regular_chain"
	tm.Create(Chain{Name: cName})
	assert.NilError(t, tbl.Apply(t.Context(), tm))

	// The new policy is applied immediately. (Which must not deadlock - the table
	// is applied with applyLock held.)
	assert.NilError(t, tbl.SetBaseChainPolicy(t.Context(), bcName, BaseChainPolicyDrop))
	res := icmd.RunCommand("nft", "list", "table", string(tbl.Family()), tbl.Name())
	res.Assert(t, icmd.Success)
	golden.Assert(t, res.Combined(), t.Name()+".golden")

	assert.Check(t, is.ErrorContains(
		tbl.SetBaseChainPolicy(t.Context(), cName, BaseChainPolicyDrop), "it is not a base chain"))
	assert.Check(t, is.ErrorContains(
		tbl.SetBaseChainPolicy(t.Context(), "no_such_chain", BaseChainPolicyDrop), "it does not exist"))
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
	applyAndCheck(t, t.Name()+"/created.golden", tbl, tm)

	// Delete a rule from the base chain.
	tm = Modifier{}
	tm.Delete(bcCounterRule)

	// Update nftables and check what happened.
	applyAndCheck(t, t.Name()+"/modified.golden", tbl, tm)

	// Delete the base chain.
	tm = Modifier{}
	tm.Delete(bcJumpRule)
	tm.Delete(bcDesc)
	tm.Delete(cRule)
	tm.Delete(cDesc)

	// Update nftables and check what happened.
	applyAndCheck(t, t.Name()+"/deleted.golden", tbl, tm)
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
	applyAndCheck(t, t.Name()+".golden", tbl, tm)
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
	applyAndCheck(t, t.Name()+"/created.golden", tbl, tm)

	// Add the rule again, ignoring the duplicate, but in a modifier that has an
	// error - check that the existing rule isn't removed by rollback of this modifier.
	tmErr := Modifier{}
	tmErr.Create(Rule{Chain: chainName, Rule: []string{"counter"}, IgnoreExist: true})
	tmErr.Create(Rule{Chain: chainName})
	err = tbl.Apply(t.Context(), tm)
	assert.Check(t, err != nil, "Expected an error")

	// Reload, to flush table state.
	reloadAndCheck(t, t.Name()+"/created.golden", tbl)

	// Delete the rule.
	tmDel := Modifier{}
	tmDel.Delete(Rule{Chain: chainName, Rule: []string{"counter"}})
	applyAndCheck(t, t.Name()+"/deleted.golden", tbl, tmDel)

	// Delete it again, in another chain that will roll back, to check it's not resurrected.
	tmReDel := Modifier{}
	tmReDel.Delete(Rule{Chain: chainName, Rule: []string{"counter"}, IgnoreExist: true})
	tmReDel.Create(Rule{Chain: chainName})
	err = tbl.Apply(t.Context(), tmReDel)
	assert.Check(t, err != nil, "Expected an error")

	// Reload, to flush table state.
	reloadAndCheck(t, t.Name()+"/deleted.golden", tbl)
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
	tm.Create(Map{
		Name:        mapName,
		ElementType: Ifname.VMap(),
		Flags:       []string{"dynamic", "timeout"},
		Counter:     true,
	})
	tm.Create(MapElement{MapName: mapName, Key: "eth0", Value: "return"})
	tm.Create(MapElement{MapName: mapName, Key: "eth1", Value: "drop", Comment: `/// this is a comment on a map element \\\`})

	// Update nftables and check what happened.
	applyAndCheck(t, t.Name()+"/created.golden", tbl, tm)

	// Undo those changes by reversing the commands.
	tmRev := tm.Reverse()

	// Update nftables and check what happened.
	applyAndCheck(t, t.Name()+"/deleted.golden", tbl, tmRev)
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
	tm4.Create(Set{Name: set4Name, ElementType: IPv4Addr, Flags: []string{"interval"}, Counter: true})
	const set6Name = "set6"
	tm6 := Modifier{}
	tm6.Create(Set{Name: set6Name, ElementType: IPv6Addr, Flags: []string{"interval", "timeout"}})

	// Add elements to each set.
	tm4.Create(SetElement{SetName: set4Name, Element: "192.0.2.0/24"})
	tm6.Create(SetElement{SetName: set6Name, Element: "2001:db8::/64", Comment: `/// this is a comment on a set element \\\`})

	// Update nftables and check what happened.
	applyAndCheck(t, t.Name()+"/created4.golden", tbl4, tm4)
	applyAndCheck(t, t.Name()+"/created6.golden", tbl6, tm6)

	// Delete elements.
	applyAndCheck(t, t.Name()+"/deleted4.golden", tbl4, tm4.Reverse())
	applyAndCheck(t, t.Name()+"/deleted6.golden", tbl6, tm6.Reverse())
}

func TestMapElementAtomicReplace(t *testing.T) {
	defer testSetup(t)()

	tbl, err := NewTable(IPv4, "x")
	assert.NilError(t, err)
	defer tbl.Close()

	const mapName = "a_map"
	init := Modifier{}
	init.Create(Map{
		Name:        mapName,
		ElementType: Typeof("numgen random mod 1024").MapTo("ip daddr"),
		Flags:       []string{"interval"},
	})

	gen1 := Modifier{}
	gen1.Create(MapElement{MapName: mapName, Key: "0-511", Value: "127.0.0.1"})
	gen1.Create(MapElement{MapName: mapName, Key: "512-1023", Value: "192.168.0.1"})
	applyAndCheck(t, t.Name()+"/init.golden", tbl, init, gen1)

	gen2 := Modifier{}
	gen2.Create(MapElement{MapName: mapName, Key: "0-511", Value: "169.254.169.254"})
	gen2.Create(MapElement{MapName: mapName, Key: "512-1023", Value: "1.1.1.1"})
	applyAndCheck(t, t.Name()+"/updated.golden", tbl, gen1.Reverse(), gen2)
}

func TestMapElementDeleteCancelsCreate(t *testing.T) {
	// The final state of applying a sequence of creates and deletes should
	// be the same, whether applied from one transaction or multiple.

	defer testSetup(t)()

	tbl, err := NewTable(IPv4, "x")
	assert.NilError(t, err)
	defer tbl.Close()

	const mapName = "a_map"
	var init, create Modifier
	init.Create(Map{
		Name:        mapName,
		ElementType: Typeof("numgen random mod 1024").MapTo("ip daddr"),
		Flags:       []string{"interval"},
	})
	create.Create(MapElement{MapName: mapName, Key: "0-1023", Value: "127.0.0.1"})
	applyAndCheck(t, t.Name()+".golden", tbl, init, create, create.Reverse())
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
	tm.Create(Map{Name: vmapName, ElementType: Ifname.VMap()})
	tm.Create(MapElement{MapName: vmapName, Key: "eth0", Value: "return"})
	tm.Create(MapElement{MapName: vmapName, Key: "eth1", Value: "return", Comment: "{foo}"})

	const setName = "this_is_a_set"
	tm.Create(Set{Name: setName, ElementType: IPv4Addr, Flags: []string{"interval"}})
	tm.Create(SetElement{SetName: setName, Element: "192.0.2.0/24", Comment: "}bar{"})

	tm.Create(Map{Name: "dynamic_map", ElementType: IPv4Addr.MapTo(EtherAddr), Flags: []string{"dynamic", "timeout"}, Size: 1024, Timeout: 2*time.Minute + 30*time.Second + 500*time.Millisecond + 654*time.Microsecond})
	tm.Create(Set{Name: "dynamic_set", ElementType: IPv4Addr, Flags: []string{"dynamic", "timeout"}, Size: 4096, Timeout: 5*time.Minute + 10*time.Second + 250*time.Millisecond + 123*time.Microsecond})

	applyAndCheck(t, t.Name()+"/created.golden", tbl, tm)

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
	err = tbl.Reload(t.Context())
	assert.Check(t, err)
	res := icmd.RunCommand("nft", "list", "table", string(tbl.Family()), tbl.Name())
	res.Assert(t, icmd.Success)
	golden.Assert(t, res.Combined(), t.Name()+"/created.golden")

	// Delete again.
	deleteTable()

	// Check implicit/recovery reload - only deleting something that's gone missing
	// from a vmap/set will trigger this.
	incrementalUpdateFailedHook = nil
	tm = Modifier{}
	tm.Delete(SetElement{SetName: setName, Element: "192.0.2.0/24"})
	applyAndCheck(t, t.Name()+"/recovered.golden", tbl, tm)
}

func TestApplyMultipleModifiers(t *testing.T) {
	defer testSetup(t)()

	tbl, err := NewTable(IPv4, "this_is_a_table")
	assert.NilError(t, err)
	defer tbl.Close()

	const chainName = "this_is_a_chain"
	var tm1, tm2 Modifier
	tm1.Create(Chain{Name: chainName})
	tm1.Create(Rule{Chain: chainName, Rule: []string{"counter"}})

	tm2.Create(Rule{Chain: chainName, Rule: []string{"drop"}})
	// This rule should fail validation and trigger rollback.
	tm2.Create(Rule{Chain: "bogus", Rule: []string{"counter"}})
	tm2.Create(Rule{Chain: chainName, Rule: []string{"reject"}})

	err = tbl.Apply(t.Context(), tm1, tm2)
	assert.Check(t, err != nil, "Expected an error")

	// Verify the apply was a no-op: the table should not exist yet.
	res := icmd.RunCommand("nft", "list", "table", string(tbl.Family()), tbl.Name())
	res.Assert(t, icmd.Expected{ExitCode: 1})

	// Verify no traces of the failed apply remain in memory.
	reloadAndCheck(t, t.Name()+"/empty.golden", tbl)

	// A subsequent valid apply should still succeed.
	var tm3 Modifier
	tm3.Create(Rule{Chain: chainName, Rule: []string{"accept"}})
	applyAndCheck(t, t.Name()+"/created.golden", tbl, tm1, tm3)
}

func TestNetdevChain(t *testing.T) {
	defer testSetup(t)()

	tbl, err := NewTable(Netdev, "testtable")
	assert.NilError(t, err)
	defer tbl.Close()
	tm := Modifier{}

	const bcName = "this_is_a_netdev_chain"
	tm.Create(BaseChain{
		Name:      bcName,
		ChainType: BaseChainTypeFilter,
		Hook:      BaseChainHookIngress,
		Device:    "lo",
		Priority:  -123,
		Policy:    BaseChainPolicyAccept,
	})
	tm.Create(Rule{Chain: bcName, Rule: []string{"accept"}})
	applyAndCheck(t, t.Name()+"/created.golden", tbl, tm)

	icmd.RunCommand("nft", "flush", "ruleset").Assert(t, icmd.Success)
	err = tbl.Reload(t.Context())
	assert.Check(t, err)
	res := icmd.RunCommand("nft", "list", "table", string(tbl.Family()), tbl.Name())
	res.Assert(t, icmd.Success)
	golden.Assert(t, res.Combined(), t.Name()+"/created.golden")
}

func TestValidation(t *testing.T) {
	testcases := []struct {
		name   string
		cmds   []objCmd
		expErr string
	}{
		// BaseChain
		{
			name: "create with missing base chain name",
			cmds: []objCmd{
				{obj: BaseChain{ChainType: BaseChainTypeNAT, Hook: BaseChainHookPostrouting, Priority: BaseChainPrioritySrcNAT}},
			},
			expErr: "base chain must have a name",
		},
		{
			name: "create with missing base chain type",
			cmds: []objCmd{
				{obj: BaseChain{Name: "achain", Hook: BaseChainHookPostrouting, Priority: BaseChainPrioritySrcNAT}},
			},
			expErr: "chain 'achain': fields ChainType and Hook are required",
		},
		{
			name: "create with missing base chain hook",
			cmds: []objCmd{
				{obj: BaseChain{Name: "achain", ChainType: BaseChainTypeNAT, Priority: BaseChainPrioritySrcNAT}},
			},
			expErr: "chain 'achain': fields ChainType and Hook are required",
		},
		{
			name: "delete non-empty base chain",
			cmds: []objCmd{
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
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Chain{Name: "achain"}},
			},
			expErr: "already exists",
		},
		{
			name: "delete missing chain",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}, delete: true},
			},
			expErr: "does not exist",
		},
		{
			name: "missing chain name",
			cmds: []objCmd{
				{obj: Chain{}},
			},
			expErr: "chain must have a name",
		},
		{
			name: "delete non-empty chain",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
				{obj: Chain{Name: "achain"}, delete: true},
			},
			expErr: "cannot delete chain 'achain', it is not empty",
		},
		// Rule
		{
			name: "bad rule",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"this is nonsense"}}},
			},
			expErr: "syntax error",
		},
		{
			name: "duplicate rule",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
			},
			expErr: "rule exists",
		},
		{
			name: "delete missing rule",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}, delete: true},
			},
			expErr: "does not exist",
		},
		{
			name: "duplicate rule delete",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}, delete: true},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}, delete: true},
			},
			expErr: "does not exist",
		},
		{
			name: "create rule with missing chain name",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Rule: []string{"counter"}}},
			},
			expErr: "chain '' does not exist",
		},
		{
			name: "delete rule with missing chain name",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Rule: []string{"counter"}}, delete: true},
			},
			expErr: "chain '' does not exist",
		},
		{
			name: "create rule with nonexistent chain",
			cmds: []objCmd{
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
			},
			expErr: "chain 'achain' does not exist",
		},
		{
			name: "delete rule with nonexistent chain",
			cmds: []objCmd{
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}, delete: true},
			},
			expErr: "chain 'achain' does not exist",
		},
		{
			name: "create rule with no rule",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain"}},
			},
			expErr: "cannot add empty rule",
		},
		{
			name: "delete rule with no rule",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain"}, delete: true},
			},
			expErr: "cannot delete empty rule",
		},
		{
			name: "bad rule mid-sequence",
			cmds: []objCmd{
				{obj: Chain{Name: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}, delete: true},
				{obj: Rule{Chain: "achain"}},
				{obj: Rule{Chain: "achain", Rule: []string{"counter"}}},
			},
			expErr: "chain 'achain', cannot add empty rule",
		},
		// Map (verdict)
		{
			name: "duplicate map",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
			},
			expErr: "map 'avmap' already exists",
		},
		{
			name: "delete nonexistent map",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}, delete: true},
			},
			expErr: "cannot delete map 'avmap', it does not exist",
		},
		{
			name:   "missing map name",
			cmds:   []objCmd{{obj: Map{ElementType: Ifname.VMap()}}},
			expErr: "map must have a name",
		},
		{
			name:   "missing map element type",
			cmds:   []objCmd{{obj: Map{Name: "avmap"}}},
			expErr: "map 'avmap' has no element type",
		},
		{
			name: "delete non-empty map",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
				{obj: MapElement{MapName: "avmap", Key: "eth0", Value: "drop"}},
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}, delete: true},
			},
			expErr: "cannot delete map 'avmap', it contains 1 elements",
		},
		// MapElement
		{
			name: "duplicate map element",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
				{obj: MapElement{MapName: "avmap", Key: "eth0", Value: "drop"}},
				{obj: MapElement{MapName: "avmap", Key: "eth0", Value: "drop"}},
			},
			expErr: "map 'avmap' already contains element 'eth0'",
		},
		{
			name: "add to map that does not exist",
			cmds: []objCmd{
				{obj: MapElement{MapName: "avmap", Key: "eth0", Value: "drop"}},
			},
			expErr: "cannot add to map 'avmap', it does not exist",
		},
		{
			name: "delete nonexistent map element",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
				{obj: MapElement{MapName: "avmap", Key: "eth0", Value: "drop"}, delete: true},
			},
			expErr: "map 'avmap' does not contain element 'eth0'",
		},
		{
			name: "delete map element twice",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
				{obj: MapElement{MapName: "avmap", Key: "eth0", Value: "drop"}},
				{obj: MapElement{MapName: "avmap", Key: "eth0", Value: "drop"}, delete: true},
				{obj: MapElement{MapName: "avmap", Key: "eth0", Value: "drop"}, delete: true},
			},
			expErr: "map 'avmap' does not contain element 'eth0'",
		},
		{
			name: "map element with no named map",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
				{obj: MapElement{Key: "eth0", Value: "drop"}},
			},
			expErr: "cannot add element to unnamed map",
		},
		{
			name: "map element with no key",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
				{obj: MapElement{MapName: "avmap", Value: "drop"}},
			},
			expErr: "cannot add to map 'avmap', element must have key and value",
		},
		{
			name: "map element with no value",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
				{obj: MapElement{MapName: "avmap", Key: "eth0"}},
			},
			expErr: "cannot add to map 'avmap', element must have key and value",
		},
		{
			name: "map element with newline in comment",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
				{obj: MapElement{MapName: "avmap", Key: "eth0", Value: "drop", Comment: "new\nline"}},
			},
			expErr: `map 'avmap' element 'eth0' comment contains "\n"`,
		},
		{
			name: "map element with quote char in comment",
			cmds: []objCmd{
				{obj: Map{Name: "avmap", ElementType: Ifname.VMap()}},
				{obj: MapElement{MapName: "avmap", Key: "eth0", Value: "drop", Comment: `"quoted"`}},
			},
			expErr: `map 'avmap' element 'eth0' comment contains "\""`,
		},
		// Set
		{
			name: "duplicate set",
			cmds: []objCmd{
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}},
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}},
			},
			expErr: "set 'aset' already exists",
		},
		{
			name: "delete nonexistent set",
			cmds: []objCmd{
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}, delete: true},
			},
			expErr: "cannot delete set 'aset', it does not exist",
		},
		{
			name: "missing set name",
			cmds: []objCmd{
				{obj: Set{ElementType: IPv4Addr, Flags: []string{"interval"}}},
			},
			expErr: "set must have a name",
		},
		{
			name: "missing set element type",
			cmds: []objCmd{
				{obj: Set{Name: "aset", Flags: []string{"interval"}}},
			},
			expErr: "set 'aset' must have a type",
		},
		{
			name: "delete non-empty set",
			cmds: []objCmd{
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset", Element: "192.0.2.0/24"}},
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}, delete: true},
			},
			expErr: "cannot delete set 'aset', it contains 1 elements",
		},
		// SetElement
		{
			name: "duplicate set element",
			cmds: []objCmd{
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset", Element: "192.0.2.0/24"}},
				{obj: SetElement{SetName: "aset", Element: "192.0.2.0/24"}},
			},
			expErr: "set 'aset' already contains element '192.0.2.0/24'",
		},
		{
			name: "delete nonexistent set element",
			cmds: []objCmd{
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset", Element: "192.0.2.0/24"}, delete: true},
			},
			expErr: "cannot delete '192.0.2.0/24' from set 'aset', it does not exist",
		},
		{
			name: "add set element to unnamed set",
			cmds: []objCmd{
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{Element: "192.0.2.0/24"}},
			},
			expErr: "cannot add to set '', it does not exist",
		},
		{
			name: "add set element with no element",
			cmds: []objCmd{
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset"}},
			},
			expErr: "cannot add to set 'aset', element not specified",
		},
		{
			name: "mismatched set element type",
			cmds: []objCmd{
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset", Element: "2001:db8::/64"}},
			},
			expErr: "Address family for hostname not supported",
		},
		{
			name: "set element with newline in comment",
			cmds: []objCmd{
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset", Element: "192.0.2.0/24", Comment: "new\nline"}},
			},
			expErr: `set 'aset' element '192.0.2.0/24' comment contains "\n"`,
		},
		{
			name: "set element with quote char in comment",
			cmds: []objCmd{
				{obj: Set{Name: "aset", ElementType: IPv4Addr, Flags: []string{"interval"}}},
				{obj: SetElement{SetName: "aset", Element: "192.0.2.0/24", Comment: `"quoted"`}},
			},
			expErr: `set 'aset' element '192.0.2.0/24' comment contains "\""`,
		},
	}

	testName := t.Name()
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			defer testSetup(t)()
			if tc.expErr != "" {
				incrementalUpdateFailedHook = nil
			}
			tbl, err := NewTable(IPv4, "tablename")
			assert.NilError(t, err)
			defer tbl.Close()
			tm := Batch{}
			for _, c := range tc.cmds {
				tm.Append(c)
			}
			err = tbl.Apply(t.Context(), tm)
			assert.Check(t, err != nil, "expected error containing '%s'", tc.expErr)
			assert.Check(t, is.ErrorContains(err, tc.expErr))
			// Check the table wasn't created.
			res := icmd.RunCommand("nft", "list", "table", string(IPv4), "tablename")
			res.Assert(t, icmd.Expected{ExitCode: 1})
			// Check the empty table can be created (the Table structure is still healthy).
			applyAndCheck(t, testName+"/empty.golden", tbl, Modifier{})
		})
	}
}

func TestMapElementDeleteFunc(t *testing.T) {
	defer testSetup(t)()

	tbl, err := NewTable(IPv4, "x")
	assert.NilError(t, err)
	defer tbl.Close()

	// Keys are concatenated, as the load balancer's are: the first component
	// identifies the owner's destination, so different owners never share - or
	// overlap in - the interval space.
	const mapName = "a_map"
	init := Modifier{}
	init.Create(Map{
		Name:        mapName,
		ElementType: Typeof("ip daddr").Concat("numgen random mod 1024").MapTo("ip daddr"),
		Flags:       []string{"interval"},
	})
	// Two owners sharing one map, identified by the element comment.
	gen1 := Modifier{}
	gen1.Create(MapElement{MapName: mapName, Key: "10.0.0.1 . 0-511", Value: "127.0.0.1", Comment: "svc-a"})
	gen1.Create(MapElement{MapName: mapName, Key: "10.0.0.1 . 512-1023", Value: "127.0.0.2", Comment: "svc-a"})
	gen1.Create(MapElement{MapName: mapName, Key: "10.0.0.2 . 0-1023", Value: "127.0.0.3", Comment: "svc-b"})
	applyAndCheck(t, t.Name()+"/init.golden", tbl, init, gen1)

	// Replace svc-a's elements with a different number of intervals, in a single
	// Batch: the delete-func sweeps whatever it currently owns (including the
	// keys that are about to disappear, which is why they can't simply be
	// overwritten), then the new elements are created. svc-b must be left alone.
	gen2 := Batch{}
	gen2.Append(MapElementDeleteFunc{
		MapName: mapName,
		Fn:      func(e MapElement) bool { return e.Comment == "svc-a" },
	})
	gen2.Append(Create(MapElement{MapName: mapName, Key: "10.0.0.1 . 0-340", Value: "127.0.0.4", Comment: "svc-a"}))
	gen2.Append(Create(MapElement{MapName: mapName, Key: "10.0.0.1 . 341-1023", Value: "127.0.0.5", Comment: "svc-a"}))
	applyAndCheck(t, t.Name()+"/replaced.golden", tbl, gen2)

	// Sweeping an owner with nothing in the map is a no-op, not an error.
	gen3 := Batch{}
	gen3.Append(MapElementDeleteFunc{
		MapName: mapName,
		Fn:      func(e MapElement) bool { return e.Comment == "svc-nonexistent" },
	})
	applyAndCheck(t, t.Name()+"/replaced.golden", tbl, gen3)
}

func TestMapElementDeleteFuncRollback(t *testing.T) {
	defer testSetup(t)()
	// The failing Apply below is expected.
	incrementalUpdateFailedHook = nil

	tbl, err := NewTable(IPv4, "x")
	assert.NilError(t, err)
	defer tbl.Close()

	const mapName = "a_map"
	init := Modifier{}
	init.Create(Map{
		Name:        mapName,
		ElementType: Typeof("ip daddr").Concat("numgen random mod 1024").MapTo("ip daddr"),
		Flags:       []string{"interval"},
	})
	init.Create(MapElement{MapName: mapName, Key: "10.0.0.1 . 0-511", Value: "127.0.0.1", Comment: "svc-a"})
	init.Create(MapElement{MapName: mapName, Key: "10.0.0.1 . 512-1023", Value: "127.0.0.2", Comment: "svc-a"})
	applyAndCheck(t, t.Name()+"/init.golden", tbl, init)

	// Sweep the elements, then fail: deleting an element that doesn't exist is an
	// error, so the whole Batch is rolled back.
	tm := Batch{}
	tm.Append(MapElementDeleteFunc{
		MapName: mapName,
		Fn:      func(e MapElement) bool { return e.Comment == "svc-a" },
	})
	tm.Append(Delete(MapElement{MapName: mapName, Key: "10.0.0.9 . 0-1023", Value: "127.0.0.9"}))
	err = tbl.Apply(t.Context(), tm)
	assert.Check(t, err != nil, "expected the Apply to fail")

	// The swept elements must be restored, comments included - the expanded
	// deletes name the elements fully, so the normal rollback can recreate them.
	applyAndCheck(t, t.Name()+"/init.golden", tbl, Modifier{})
}

func TestMapElementDeleteFuncErrors(t *testing.T) {
	defer testSetup(t)()
	incrementalUpdateFailedHook = nil

	const mapName = "a_map"
	newTable := func(t *testing.T) *Table {
		t.Helper()
		tbl, err := NewTable(IPv4, "x")
		assert.NilError(t, err)
		t.Cleanup(func() { tbl.Close() })
		init := Modifier{}
		init.Create(Map{
			Name:        mapName,
			ElementType: Typeof("numgen random mod 1024").MapTo("ip daddr"),
			Flags:       []string{"interval"},
		})
		assert.NilError(t, tbl.Apply(t.Context(), init))
		return tbl
	}

	alwaysTrue := func(MapElement) bool { return true }

	tests := []struct {
		name   string
		tm     func() Batch
		expErr string
	}{
		// Neither a "create" nor a "reversed" case is testable any more, and that's
		// the point: MapElementDeleteFunc is a Cmd, not an Obj, so it can only reach a
		// Batch - which has no Create and no Reverse. Both misuses are compile errors.
		{
			name: "no function",
			tm: func() Batch {
				tm := Batch{}
				tm.Append(MapElementDeleteFunc{MapName: mapName})
				return tm
			},
			expErr: "no function",
		},
		{
			name: "unnamed map",
			tm: func() Batch {
				tm := Batch{}
				tm.Append(MapElementDeleteFunc{Fn: alwaysTrue})
				return tm
			},
			expErr: "unnamed map",
		},
		{
			name: "no such map",
			tm: func() Batch {
				tm := Batch{}
				tm.Append(MapElementDeleteFunc{MapName: "nope", Fn: alwaysTrue})
				return tm
			},
			expErr: "it does not exist",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tbl := newTable(t)
			err := tbl.Apply(t.Context(), tc.tm())
			assert.Check(t, err != nil, "expected error containing '%s'", tc.expErr)
			assert.Check(t, is.ErrorContains(err, tc.expErr))
		})
	}
}
