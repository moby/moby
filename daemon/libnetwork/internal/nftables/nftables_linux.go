// Package nftables provides methods to create an nftables table and manage its maps, sets,
// chains, and rules.
//
// To use it, the first step is to create a [TableRef] using [NewTable]. The table can
// then be populated and managed using that ref.
//
// Modifications to the table are only applied (sent to "nft") when [TableRef.Apply] is
// called. This means a number of updates can be made, for example, adding all the
// rules needed for a docker network - and those rules will then be applied atomically
// in a single "nft" run.
//
// [TableRef.Apply] can only be called after [Enable], and only if [Enable] returns
// true (meaning an "nft" executable was found). [Enabled] can be called to check
// whether nftables has been enabled.
//
// Be aware:
//   - The implementation is far from complete, only functionality needed so-far has
//     been included. Currently, there's only a limited set of chain/map/set types,
//     there's no way to delete sets/maps etc.
//   - There's no rollback so, once changes have been made to a TableRef, if the
//     Apply fails there is no way to undo changes. The TableRef will be out-of-sync
//     with the actual state of nftables.
//   - This is a thin layer between code and "nft", it doesn't do much error checking. So,
//     for example, if you get the syntax of a rule wrong the issue won't be reported
//     until Apply is called.
//   - Also in the category of no-error-checking, there's no reference checking. If you
//     delete a chain that's still referred to by a map, set or another chain, "nft" will
//     report an error when Apply is called.
//   - Error checking here is meant to help spot logical errors in the code, like adding
//     a rule twice, which would be fine by "nft" as it'd just create a duplicate rule.
//   - The existing state of a table in the ruleset is irrelevant, once a Table is created
//     by this package it will be flushed. Putting it another way, this package is
//     write-only, it does not load any state from the host.
//   - Errors from "nft" are logged along with the line-numbered command that failed,
//     that's the place to look when things go wrong.
package nftables

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/containerd/log"
	"go.opentelemetry.io/otel"
)

// Prefix for OTEL span names.
const spanPrefix = "libnetwork.internal.nftables"

var (
	// nftPath is the path of the "nft" tool, set by [Enable] and left empty if the tool
	// is not present - in which case, nftables is disabled.
	nftPath string
	// Error returned by Enable if nftables could not be initialised.
	nftEnableError error
	// incrementalUpdateTempl is a parsed text/template, used to apply incremental updates.
	incrementalUpdateTempl *template.Template
	// reloadTempl is a parsed text/template, used to apply a whole table.
	reloadTempl *template.Template
	// enableOnce is used by [Enable] to avoid checking the path for "nft" more than once.
	enableOnce sync.Once
)

var (
	// ErrRuleExist is returned when a rule is added, but it already exists in the same
	// rule group of a chain.
	ErrRuleExist = errors.New("rule exists")
	// ErrRuleNotExist is returned when a rule is removed, but does not exist in the
	// rule group of a chain.
	ErrRuleNotExist = errors.New("rule does not exist")
)

// BaseChainType enumerates the base chain types.
// See https://wiki.nftables.org/wiki-nftables/index.php/Configuring_chains#Base_chain_types
type BaseChainType string

const (
	BaseChainTypeFilter BaseChainType = "filter"
	BaseChainTypeRoute  BaseChainType = "route"
	BaseChainTypeNAT    BaseChainType = "nat"
)

// BaseChainHook enumerates the base chain hook types.
// See https://wiki.nftables.org/wiki-nftables/index.php/Configuring_chains#Base_chain_hooks
type BaseChainHook string

const (
	BaseChainHookIngress     BaseChainHook = "ingress"
	BaseChainHookPrerouting  BaseChainHook = "prerouting"
	BaseChainHookInput       BaseChainHook = "input"
	BaseChainHookForward     BaseChainHook = "forward"
	BaseChainHookOutput      BaseChainHook = "output"
	BaseChainHookPostrouting BaseChainHook = "postrouting"
)

// Standard priority values for base chains.
// (Not for the bridge family, those are different.)
const (
	BaseChainPriorityRaw      = -300
	BaseChainPriorityMangle   = -150
	BaseChainPriorityDstNAT   = -100
	BaseChainPriorityFilter   = 0
	BaseChainPrioritySecurity = 50
	BaseChainPrioritySrcNAT   = 100
)

// Family enumerates address families.
type Family string

const (
	IPv4 Family = "ip"
	IPv6 Family = "ip6"
)

// nftType enumerates nft types that can be used to define maps/sets etc.
type nftType string

const (
	nftTypeIPv4Addr    nftType = "ipv4_addr"
	nftTypeIPv6Addr    nftType = "ipv6_addr"
	nftTypeEtherAddr   nftType = "ether_addr"
	nftTypeInetProto   nftType = "inet_proto"
	nftTypeInetService nftType = "inet_service"
	nftTypeMark        nftType = "mark"
	nftTypeIfname      nftType = "ifname"
)

// Enable tries once to initialise nftables.
func Enable() error {
	enableOnce.Do(func() {
		path, err := exec.LookPath("nft")
		if err != nil {
			log.G(context.Background()).WithError(err).Warnf("Failed to find nft tool")
			nftEnableError = fmt.Errorf("failed to find nft tool: %w", err)
			return
		}
		if err := parseTemplate(); err != nil {
			log.G(context.Background()).WithError(err).Error("Internal error while initialising nftables")
			nftEnableError = fmt.Errorf("internal error while initialising nftables: %w", err)
			return
		}
		nftPath = path
	})
	return nftEnableError
}

// Enabled returns true if the "nft" tool is available and [Enable] has been called.
func Enabled() bool {
	return nftPath != ""
}

// Disable undoes Enable. Intended for unit testing.
func Disable() {
	nftPath = ""
	incrementalUpdateTempl = nil
	reloadTempl = nil
	enableOnce = sync.Once{}
}

//////////////////////////////
// Tables

// table is the internal representation of an nftables table.
// Its elements need to be exported for use by text/template, but they should only be
// manipulated via exported methods.
type table struct {
	Name   string
	Family Family

	VMaps  map[string]*vMap
	Sets   map[string]*set
	Chains map[string]*chain

	Dirty               bool // Set when the table is new, not when its elements change.
	DeleteChainCommands []string
}

// TableRef is a handle for an nftables table.
type TableRef struct {
	t *table
}

// NewTable creates a new nftables table and returns a [TableRef]
//
// See https://wiki.nftables.org/wiki-nftables/index.php/Configuring_tables
//
// The table will be created and flushed when [TableRef.Apply] is next called.
// It's flushed in case it already exists in the host's nftables - when that
// happens, rules in its chains will be deleted but not the chains themselves,
// maps, sets, or elements of maps or sets. But, those un-flushed items can't do
// anything disruptive unless referred to by rules, and they will be flushed if
// they get re-created via the [TableRef], when [TableRef.Apply] is next called
// (so, before they can be used by a new rule).
func NewTable(family Family, name string) (TableRef, error) {
	t := TableRef{
		t: &table{
			Name:   name,
			Family: family,
			VMaps:  map[string]*vMap{},
			Sets:   map[string]*set{},
			Chains: map[string]*chain{},
			Dirty:  true,
		},
	}
	return t, nil
}

// Family returns the address family of the nftables table described by [TableRef].
func (t TableRef) Family() Family {
	return t.t.Family
}

// incrementalUpdateTemplText is used with text/template to generate an nftables command file
// (which will be applied atomically). Updates using this template are always incremental.
// Steps are:
//   - declare the table and its sets/maps with empty versions of modified chains, so that
//     they can be flushed/deleted if they don't yet exist. (They need to be flushed in case
//     a version of them was left behind by an old incarnation of the daemon. But, it's an
//     error to flush or delete something that doesn't exist. So, avoid having to parse nft's
//     stderr to work out what happened by making sure they do exist before flushing.)
//   - if the table is newly declared, flush rules from its chains
//   - flush each newly declared map/set
//   - delete deleted map/set elements
//   - flush modified chains
//   - delete deleted chains
//   - re-populate modified chains
//   - add new map/set elements
const incrementalUpdateTemplText = `{{$family := .Family}}{{$tableName := .Name}}
table {{$family}} {{$tableName}} {
	{{range .VMaps}}map {{.Name}} {
		type {{.ElementType}} : verdict
		{{if len .Flags}}flags{{range .Flags}} {{.}}{{end}}{{end}}
	}
	{{end}}
	{{range .Sets}}set {{.Name}} {
		type {{.ElementType}}
		{{if len .Flags}}flags{{range .Flags}} {{.}}{{end}}{{end}}
	}
	{{end}}
	{{range .Chains}}{{if .Dirty}}chain {{.Name}} {
		{{if .ChainType}}type {{.ChainType}} hook {{.Hook}} priority {{.Priority}}; policy {{.Policy}}{{end}}
	} ; {{end}}{{end}}
}
{{if .Dirty}}flush table {{$family}} {{$tableName}}{{end}}
{{range .VMaps}}{{if .Dirty}}flush map {{$family}} {{$tableName}} {{.Name}}
{{end}}{{end}}
{{range .Sets}}{{if .Dirty}}flush set {{$family}} {{$tableName}} {{.Name}}
{{end}}{{end}}
{{range .Chains}}{{if .Dirty}}flush chain {{$family}} {{$tableName}} {{.Name}}
{{end}}{{end}}
{{range .VMaps}}{{if .DeletedElements}}delete element {{$family}} {{$tableName}} {{.Name}} { {{range $k,$v := .DeletedElements}}{{$k}}, {{end}} }
{{end}}{{end}}
{{range .Sets}}{{if .DeletedElements}}delete element {{$family}} {{$tableName}} {{.Name}} { {{range $k,$v := .DeletedElements}}{{$k}}, {{end}} }
{{end}}{{end}}
{{range .DeleteChainCommands}}{{.}}
{{end}}
table {{$family}} {{$tableName}} {
	{{range .Chains}}{{if .Dirty}}chain {{.Name}} {
		{{if .ChainType}}type {{.ChainType}} hook {{.Hook}} priority {{.Priority}}; policy {{.Policy}}{{end}}
		{{range .Rules}}{{.}}
		{{end}}
	}
	{{end}}{{end}}
}
{{range .VMaps}}{{if .AddedElements}}add element {{$family}} {{$tableName}} {{.Name}} { {{range $k,$v := .AddedElements}}{{$k}} : {{$v}}, {{end}} }
{{end}}{{end}}
{{range .Sets}}{{if .AddedElements}}add element {{$family}} {{$tableName}} {{.Name}} { {{range $k,$v := .AddedElements}}{{$k}}, {{end}} }
{{end}}{{end}}
`

// reloadTemplText is used with text/template to generate an nftables command file
// (which will be applied atomically), to fully re-create a table.
//
// It first declares the table so if it doesn't already exist, it can be deleted.
// Then it deletes the table and re-creates it.
const reloadTemplText = `{{$family := .Family}}{{$tableName := .Name}}
table {{$family}} {{$tableName}} {}
delete table {{$family}} {{$tableName}}
table {{$family}} {{$tableName}} {
	{{range .VMaps}}map {{.Name}} {
		type {{.ElementType}} : verdict
		{{if len .Flags}}flags{{range .Flags}} {{.}}{{end}}{{end}}
        {{if .Elements}}elements = {
			{{range $k,$v := .Elements}}{{$k}} : {{$v}},
            {{end -}}
		}{{end}}
	}
	{{end}}
	{{range .Sets}}set {{.Name}} {
		type {{.ElementType}}
		{{if len .Flags}}flags{{range .Flags}} {{.}}{{end}}{{end}}
        {{if .Elements}}elements = {
			{{range $k,$v := .Elements}}{{$k}},
            {{end -}}
		}{{end}}
	}
	{{end}}
	{{range .Chains}}chain {{.Name}} {
		{{if .ChainType}}type {{.ChainType}} hook {{.Hook}} priority {{.Priority}}; policy {{.Policy}}{{end}}
		{{range .Rules}}{{.}}
		{{end}}
	}
	{{end}}
}
`

// Apply makes incremental updates to nftables, corresponding to changes to the [TableRef]
// since Apply was last called.
func (t TableRef) Apply(ctx context.Context) error {
	if !Enabled() {
		return errors.New("nftables is not enabled")
	}

	// Update nftables.
	var buf bytes.Buffer
	if err := incrementalUpdateTempl.Execute(&buf, t.t); err != nil {
		return fmt.Errorf("failed to execute template nft ruleset: %w", err)
	}

	if err := nftApply(ctx, buf.Bytes()); err != nil {
		// On error, log a line-numbered version of the generated "nft" input (because
		// nft error messages refer to line numbers).
		var sb strings.Builder
		for i, line := range bytes.SplitAfter(buf.Bytes(), []byte("\n")) {
			sb.WriteString(strconv.Itoa(i + 1))
			sb.WriteString(":\t")
			sb.Write(line)
		}
		log.G(ctx).Error("nftables: failed to update nftables:\n", sb.String(), "\n", err)

		// It's possible something destructive has happened to nftables. For example, in
		// integration-cli tests, tests start daemons in the same netns as the integration
		// test's own daemon. They don't always use their own daemon, but they tend to leave
		// behind networks for the test infrastructure to clean up between tests. Starting
		// a daemon flushes the "docker-bridges" table, so the cleanup fails to delete a
		// rule that's been flushed. So, try reloading the whole table to get back in-sync.
		return t.Reload(ctx)
	}

	// Note that updates have been applied.
	t.t.updatesApplied()
	return nil
}

// Reload deletes the table, then re-creates it, atomically.
func (t TableRef) Reload(ctx context.Context) error {
	if !Enabled() {
		return errors.New("nftables is not enabled")
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{"table": t.t.Name, "family": t.t.Family}))
	log.G(ctx).Warn("nftables: reloading table")

	// Build the update.
	var buf bytes.Buffer
	if err := reloadTempl.Execute(&buf, t.t); err != nil {
		return fmt.Errorf("failed to execute reload template: %w", err)
	}

	if err := nftApply(ctx, buf.Bytes()); err != nil {
		// On error, log a line-numbered version of the generated "nft" input (because
		// nft error messages refer to line numbers).
		var sb strings.Builder
		for i, line := range bytes.SplitAfter(buf.Bytes(), []byte("\n")) {
			sb.WriteString(strconv.Itoa(i + 1))
			sb.WriteString(":\t")
			sb.Write(line)
		}
		log.G(ctx).Error("nftables: failed to reload nftable:\n", sb.String(), "\n", err)
		return err
	}

	// Note that updates have been applied.
	t.t.updatesApplied()
	return nil
}

// ////////////////////////////
// Chains

// RuleGroup is used to allocate rules within a chain to a group. These groups are
// purely an internal construct, nftables knows nothing about them. Within groups
// rules retain the order in which they were added, and groups are ordered from
// lowest to highest numbered group.
type RuleGroup int

// chain is the internal representation of an nftables chain.
// Its elements need to be exported for use by text/template, but they should only be
// manipulated via exported methods.
type chain struct {
	table      *table
	Name       string
	ChainType  BaseChainType
	Hook       BaseChainHook
	Priority   int
	Policy     string
	Dirty      bool
	ruleGroups map[RuleGroup][]string
}

// ChainRef is a handle for an nftables chain.
type ChainRef struct {
	c *chain
}

// BaseChain constructs a new nftables base chain and returns a [ChainRef].
//
// See https://wiki.nftables.org/wiki-nftables/index.php/Configuring_chains#Adding_base_chains
//
// It is an error to create a base chain that already exists.
// If the underlying chain already exists, it will be flushed by the
// next [TableRef.Apply] before new rules are added.
func (t TableRef) BaseChain(ctx context.Context, name string, chainType BaseChainType, hook BaseChainHook, priority int) (ChainRef, error) {
	if _, ok := t.t.Chains[name]; ok {
		return ChainRef{}, fmt.Errorf("chain %q already exists", name)
	}
	c := &chain{
		table:      t.t,
		Name:       name,
		ChainType:  chainType,
		Hook:       hook,
		Priority:   priority,
		Policy:     "accept",
		Dirty:      true,
		ruleGroups: map[RuleGroup][]string{},
	}
	t.t.Chains[name] = c
	log.G(ctx).WithFields(log.Fields{
		"family": t.t.Family,
		"table":  t.t.Name,
		"chain":  name,
		"type":   chainType,
		"hook":   hook,
		"prio":   priority,
	}).Debug("nftables: created base chain")
	return ChainRef{c: c}, nil
}

// Chain returns a [ChainRef] for an existing chain (which may be a base chain).
// If there is no existing chain, a regular chain is added and its [ChainRef] is
// returned.
//
// See https://wiki.nftables.org/wiki-nftables/index.php/Configuring_chains#Adding_regular_chains
//
// If a new [ChainRef] is created and the underlying chain already exists, it
// will be flushed by the next [TableRef.Apply] before new rules are added.
func (t TableRef) Chain(ctx context.Context, name string) ChainRef {
	c, ok := t.t.Chains[name]
	if !ok {
		c = &chain{
			table:      t.t,
			Name:       name,
			Dirty:      true,
			ruleGroups: map[RuleGroup][]string{},
		}
		t.t.Chains[name] = c
	}
	log.G(ctx).WithFields(log.Fields{
		"family": t.t.Family,
		"table":  t.t.Name,
		"chain":  name,
	}).Debug("nftables: created chain")
	return ChainRef{c: c}
}

// ChainUpdateFunc is a function that can add rules to a chain, or remove rules from it.
type ChainUpdateFunc func(context.Context, RuleGroup, string, ...any) error

// ChainUpdateFunc returns a [ChainUpdateFunc] to add rules to the named chain if
// enable is true, or to remove rules from the chain if enable is false.
// (Written as a convenience function to ease migration of iptables functions
// originally written with an enable flag.)
func (t TableRef) ChainUpdateFunc(ctx context.Context, name string, enable bool) ChainUpdateFunc {
	c := t.Chain(ctx, name)
	if enable {
		return c.AppendRule
	}
	return c.DeleteRule
}

// DeleteChain deletes a chain. It is an error to delete a chain that does not exist.
func (t TableRef) DeleteChain(ctx context.Context, name string) error {
	if _, ok := t.t.Chains[name]; !ok {
		return fmt.Errorf("chain %q does not exist", name)
	}
	delete(t.t.Chains, name)
	t.t.DeleteChainCommands = append(t.t.DeleteChainCommands,
		fmt.Sprintf("delete chain %s %s %s", t.t.Family, t.t.Name, name))
	log.G(ctx).WithFields(log.Fields{
		"family": t.t.Family,
		"table":  t.t.Name,
		"chain":  name,
	}).Debug("nftables: deleted chain")
	return nil
}

// SetPolicy sets the default policy for a base chain. It is an error to call this
// for a non-base [ChainRef].
func (c ChainRef) SetPolicy(policy string) error {
	if c.c.ChainType == "" {
		return errors.New("not a base chain")
	}
	c.c.Policy = policy
	c.c.Dirty = true
	return nil
}

// AppendRule appends a rule to a [RuleGroup] in a [ChainRef].
func (c ChainRef) AppendRule(ctx context.Context, group RuleGroup, rule string, args ...any) error {
	if len(args) > 0 {
		rule = fmt.Sprintf(rule, args...)
	}
	if rg, ok := c.c.ruleGroups[group]; ok && slices.Contains(rg, rule) {
		return ErrRuleExist
	}
	c.c.ruleGroups[group] = append(c.c.ruleGroups[group], rule)
	c.c.Dirty = true
	log.G(ctx).WithFields(log.Fields{
		"family": c.c.table.Family,
		"table":  c.c.table.Name,
		"chain":  c.c.Name,
		"group":  group,
		"rule":   rule,
	}).Debug("nftables: appended rule")
	return nil
}

// AppendRuleCf calls AppendRule and returns a cleanup function or an error.
func (c ChainRef) AppendRuleCf(ctx context.Context, group RuleGroup, rule string, args ...any) (func(context.Context) error, error) {
	if err := c.AppendRule(ctx, group, rule, args...); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error { return c.DeleteRule(ctx, group, rule, args...) }, nil
}

// DeleteRule deletes a rule from a [RuleGroup] in a [ChainRef]. It is an error
// to delete from a group that does not exist, or to delete a rule that does not
// exist.
func (c ChainRef) DeleteRule(ctx context.Context, group RuleGroup, rule string, args ...any) error {
	if len(args) > 0 {
		rule = fmt.Sprintf(rule, args...)
	}
	rg, ok := c.c.ruleGroups[group]
	if !ok {
		return fmt.Errorf("rule group %d does not exist", group)
	}
	origLen := len(rg)
	c.c.ruleGroups[group] = slices.DeleteFunc(rg, func(r string) bool { return r == rule })
	if len(c.c.ruleGroups[group]) == origLen {
		return ErrRuleNotExist
	}
	c.c.Dirty = true
	log.G(ctx).WithFields(log.Fields{
		"family": c.c.table.Family,
		"table":  c.c.table.Name,
		"chain":  c.c.Name,
		"rule":   rule,
	}).Debug("nftables: deleted rule")
	return nil
}

// ////////////////////////////
// VMaps

// vMap is the internal representation of an nftables verdict map.
// Its elements need to be exported for use by text/template, but they should only be
// manipulated via exported methods.
type vMap struct {
	table           *table
	Name            string
	ElementType     nftType
	Flags           []string
	Elements        map[string]string
	Dirty           bool // New vMap, needs to be flushed (not set when elements are added/deleted).
	AddedElements   map[string]string
	DeletedElements map[string]struct{}
}

// VMapRef is a handle for an nftables verdict map.
type VMapRef struct {
	v *vMap
}

// InterfaceVMap creates a map from interface name to a verdict and returns a [VMapRef],
// or returns an existing [VMapRef] if it has already been created.
//
// See https://wiki.nftables.org/wiki-nftables/index.php/Verdict_Maps_(vmaps)
//
// If a [VMapRef] is created and the underlying map already exists, it will be flushed
// by the next [TableRef.Apply] before new elements are added.
func (t TableRef) InterfaceVMap(ctx context.Context, name string) VMapRef {
	if vmap, ok := t.t.VMaps[name]; ok {
		return VMapRef{vmap}
	}
	vmap := &vMap{
		table:           t.t,
		Name:            name,
		ElementType:     nftTypeIfname,
		Elements:        map[string]string{},
		AddedElements:   map[string]string{},
		DeletedElements: map[string]struct{}{},
		Dirty:           true,
	}
	t.t.VMaps[name] = vmap
	log.G(ctx).WithFields(log.Fields{
		"family": t.t.Family,
		"table":  t.t.Name,
		"vmap":   name,
	}).Debug("nftables: created interface vmap")
	return VMapRef{vmap}
}

// AddElement adds an element to a verdict map. The caller must ensure the key has
// the correct type. It is an error to add a key that already exists.
func (v VMapRef) AddElement(ctx context.Context, key string, verdict string) error {
	if _, ok := v.v.Elements[key]; ok {
		return fmt.Errorf("verdict map already contains element %q", key)
	}
	v.v.Elements[key] = verdict
	v.v.AddedElements[key] = verdict
	log.G(ctx).WithFields(log.Fields{
		"family":  v.v.table.Family,
		"table":   v.v.table.Name,
		"vmap":    v.v.Name,
		"key":     key,
		"verdict": verdict,
	}).Debug("nftables: added vmap element")
	return nil
}

// AddElementCf calls AddElement and returns a cleanup function or an error.
func (v VMapRef) AddElementCf(ctx context.Context, key string, verdict string) (func(context.Context) error, error) {
	if err := v.AddElement(ctx, key, verdict); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error { return v.DeleteElement(ctx, key) }, nil
}

// DeleteElement deletes an element from a verdict map. It is an error to delete
// an element that does not exist.
func (v VMapRef) DeleteElement(ctx context.Context, key string) error {
	if _, ok := v.v.Elements[key]; !ok {
		return fmt.Errorf("verdict map does not contain element %q", key)
	}
	delete(v.v.Elements, key)
	v.v.DeletedElements[key] = struct{}{}
	log.G(ctx).WithFields(log.Fields{
		"family": v.v.table.Family,
		"table":  v.v.table.Name,
		"vmap":   v.v.Name,
		"key":    key,
	}).Debug("nftables: deleted vmap element")
	return nil
}

// ////////////////////////////
// Sets

// set is the internal representation of an nftables set.
// Its elements need to be exported for use by text/template, but they should only be
// manipulated via exported methods.
type set struct {
	table           *table
	Name            string
	ElementType     nftType
	Flags           []string
	Elements        map[string]struct{}
	Dirty           bool // New set, needs to be flushed (not set when elements are added/deleted).
	AddedElements   map[string]struct{}
	DeletedElements map[string]struct{}
}

// SetRef is a handle for an nftables named set.
type SetRef struct {
	s *set
}

// PrefixSet creates a new named nftables set for IPv4 or IPv6 addresses (depending
// on the address family of the [TableRef]), and returns its [SetRef]. Or, if the
// set has already been created, its [SetRef] is returned.
//
// ([TableRef] does not support "inet", only "ip" or "ip6". So the element type can
// always be determined. But, there's no "inet" element type, so this will need to
// change if we need an "inet" table.)
//
// See https://wiki.nftables.org/wiki-nftables/index.php/Sets#Named_sets
func (t TableRef) PrefixSet(ctx context.Context, name string) SetRef {
	if s, ok := t.t.Sets[name]; ok {
		return SetRef{s}
	}
	s := &set{
		table:           t.t,
		Name:            name,
		Elements:        map[string]struct{}{},
		ElementType:     nftTypeIPv4Addr,
		Flags:           []string{"interval"},
		Dirty:           true,
		AddedElements:   map[string]struct{}{},
		DeletedElements: map[string]struct{}{},
	}
	if t.t.Family == IPv6 {
		s.ElementType = nftTypeIPv6Addr
	}
	t.t.Sets[name] = s
	log.G(ctx).WithFields(log.Fields{
		"family": t.t.Family,
		"table":  t.t.Name,
		"set":    name,
	}).Debug("nftables: created set")
	return SetRef{s}
}

// AddElement adds an element to a set. It is the caller's responsibility to make sure
// the element has the correct type. It is an error to add an element that is already
// in the set.
func (s SetRef) AddElement(ctx context.Context, element string) error {
	if _, ok := s.s.Elements[element]; ok {
		return fmt.Errorf("set already contains element %q", element)
	}
	s.s.Elements[element] = struct{}{}
	s.s.AddedElements[element] = struct{}{}
	log.G(ctx).WithFields(log.Fields{
		"family":  s.s.table.Family,
		"table":   s.s.table.Name,
		"set":     s.s.Name,
		"element": element,
	}).Debug("nftables: added set element")
	return nil
}

// DeleteElement deletes an element from the set. It is an error to delete an
// element that is not in the set.
func (s SetRef) DeleteElement(ctx context.Context, element string) error {
	if _, ok := s.s.Elements[element]; !ok {
		return fmt.Errorf("set does not contain element %q", element)
	}
	delete(s.s.Elements, element)
	s.s.DeletedElements[element] = struct{}{}
	log.G(ctx).WithFields(log.Fields{
		"family":  s.s.table.Family,
		"table":   s.s.table.Name,
		"set":     s.s.Name,
		"element": element,
	}).Debug("nftables: deleted set element")
	return nil
}

// ////////////////////////////
// Internal

func (t *table) updatesApplied() {
	t.DeleteChainCommands = t.DeleteChainCommands[:0]
	for _, c := range t.Chains {
		c.Dirty = false
	}
	for _, m := range t.VMaps {
		m.Dirty = false
		m.AddedElements = map[string]string{}
		m.DeletedElements = map[string]struct{}{}
	}
	for _, s := range t.Sets {
		s.Dirty = false
		s.AddedElements = map[string]struct{}{}
		s.DeletedElements = map[string]struct{}{}
	}
	t.Dirty = false
}

/* Can't make text/template range over this, not sure why ...
func (c *chain) Rules() iter.Seq[string] {
	groups := make([]int, 0, len(c.ruleGroups))
	for group := range c.ruleGroups {
		groups = append(groups, group)
	}
	slices.Sort(groups)
	return func(yield func(string) bool) {
		for _, group := range groups {
			for _, rule := range c.ruleGroups[group] {
				if !yield(rule) {
					return
				}
			}
		}
	}
}
*/

// Rules returns the chain's rules, in order.
func (c *chain) Rules() []string {
	groups := make([]RuleGroup, 0, len(c.ruleGroups))
	nRules := 0
	for group := range c.ruleGroups {
		groups = append(groups, group)
		nRules += len(c.ruleGroups[group])
	}
	slices.Sort(groups)
	rules := make([]string, 0, nRules)
	for _, group := range groups {
		rules = append(rules, c.ruleGroups[group]...)
	}
	return rules
}

func parseTemplate() error {
	var err error
	incrementalUpdateTempl, err = template.New("ruleset").Parse(incrementalUpdateTemplText)
	if err != nil {
		return fmt.Errorf("parsing 'incrementalUpdateTemplText': %w", err)
	}
	reloadTempl, err = template.New("ruleset").Parse(reloadTemplText)
	if err != nil {
		return fmt.Errorf("parsing 'reloadTemplText': %w", err)
	}
	return nil
}

// nftApply runs the "nft" command.
func nftApply(ctx context.Context, nftCmd []byte) error {
	ctx, span := otel.Tracer("").Start(ctx, spanPrefix+".nftApply")
	defer span.End()

	if !Enabled() {
		return errors.New("nftables is not enabled")
	}
	cmd := exec.Command(nftPath, "-f", "-")
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("getting stdin pipe for nft: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("getting stdout pipe for nft: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("getting stderr pipe for nft: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting nft: %w", err)
	}
	if _, err := stdinPipe.Write(nftCmd); err != nil {
		return fmt.Errorf("sending nft commands: %w", err)
	}
	if err := stdinPipe.Close(); err != nil {
		return fmt.Errorf("closing nft input pipe: %w", err)
	}

	stdoutBuf := strings.Builder{}
	if _, err := io.Copy(&stdoutBuf, stdoutPipe); err != nil {
		return fmt.Errorf("reading stdout of nft: %w", err)
	}
	stdout := stdoutBuf.String()
	stderrBuf := strings.Builder{}
	if _, err := io.Copy(&stderrBuf, stderrPipe); err != nil {
		return fmt.Errorf("reading stderr of nft: %w", err)
	}
	stderr := stderrBuf.String()

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("running nft: %s %w", stderr, err)
	}
	log.G(ctx).WithFields(log.Fields{"stdout": stdout, "stderr": stderr}).Debug("nftables: updated")
	return nil
}
