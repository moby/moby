// Package nftables provides methods to create an nftables table and manage its maps, sets,
// chains, and rules.
//
// To use it, the first step is to create a [Table] using [NewTable]. Then, retrieve
// a [Modifier], add commands to it, and apply the updates.
//
// For example:
//
//	t, _ := NewTable(...)
//	tm := t.Modifier()
//	// Then a sequence of ...
//	tm.Create(<object>)
//	tm.Delete(<object>)
//	// Apply the updates with ...
//	err := tm.Apply(ctx)
//
// The objects are any of: [BaseChain], [Chain], [Rule], [Map], [MapElement],
// [Set], [SetElement]
//
// The modifier can be reused to apply the same set of commands again or, more
// usefully, reversed in order to revert its changes. See [Modifier.Reverse].
//
// [Modifier.Apply] can only be called after [Enable], and only if [Enable] returns
// true (meaning an "nft" executable was found). [Enabled] can be called to check
// whether nftables has been enabled.
//
// Be aware:
//   - The implementation is far from complete, only functionality needed so-far has
//     been included. Currently, there's only a limited set of chain/map/set types,
//     there's no way to delete sets/maps etc.
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
	_ "embed"
	"errors"
	"fmt"
	"iter"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/containerd/log"
)

// Prefix for OTEL span names.
const spanPrefix = "libnetwork.internal.nftables"

var (
	// enabled is set by [Enable].
	enabled bool
	// Error returned by Enable if nftables could not be initialised.
	nftEnableError error
	// incrementalUpdateTempl is a parsed text/template, used to apply incremental updates.
	incrementalUpdateTempl *template.Template
	// reloadTempl is a parsed text/template, used to apply a whole table.
	reloadTempl *template.Template
	// enableOnce is used by [Enable] to avoid parsing the templates more than once.
	enableOnce sync.Once
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
	BaseChainHookEgress      BaseChainHook = "egress"
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

// BaseChainPolicy enumerates base chain policies.
// See https://wiki.nftables.org/wiki-nftables/index.php/Configuring_chains#Base_chain_policy
type BaseChainPolicy string

const (
	BaseChainPolicyAccept BaseChainPolicy = "accept"
	BaseChainPolicyDrop   BaseChainPolicy = "drop"
)

// Family enumerates address families.
type Family string

const (
	IPv4   Family = "ip"
	IPv6   Family = "ip6"
	Netdev Family = "netdev"
)

type SetTyper interface {
	setType() string
}

type MapTyper interface {
	mapType() string
}

// SetType represents named nft types that can be used to define sets or
// construct map types.
type SetType string

const (
	IPv4Addr    SetType = "ipv4_addr"
	IPv6Addr    SetType = "ipv6_addr"
	EtherAddr   SetType = "ether_addr"
	InetProto   SetType = "inet_proto"
	InetService SetType = "inet_service"
	Mark        SetType = "mark"
	Ifname      SetType = "ifname"
)

func (t SetType) setType() string {
	return "type " + string(t)
}

// Concat returns the tuple type formed by concatenating t and u.
func (t SetType) Concat(u SetType) SetType {
	return SetType(string(t) + " . " + string(u))
}

// MapType represents the named type of a map element, which is a compound type
// with a key and a value.
type MapType string

func (t MapType) mapType() string {
	return "type " + string(t)
}

// MapTo returns the map type where t is the key type and v is the value type.
func (t SetType) MapTo(v SetType) MapType {
	return MapType(string(t) + " : " + string(v))
}

// VMap returns the map type whose elements have t as the key type and contain
// a verdict as the value.
func (t SetType) VMap() MapType {
	return MapType(string(t) + " : verdict")
}

// Typeof represents an nft "typeof" expression.
type Typeof string

func (t Typeof) setType() string {
	return "typeof " + string(t)
}

func (t Typeof) Concat(u Typeof) Typeof {
	return Typeof(string(t) + " . " + string(u))
}

// MapTypeof represents the type of a map element defined by a "typeof"
// expression.
type MapTypeof string

func (t MapTypeof) mapType() string {
	return "typeof " + string(t)
}

// MapTo returns the map type where t is the key type and v is the value type.
func (t Typeof) MapTo(v Typeof) MapTypeof {
	return MapTypeof(string(t) + " : " + string(v))
}

// VMap returns the map type whose elements have t as the key type and contain
// a verdict as the value.
func (t Typeof) VMap() MapTypeof {
	return MapTypeof(string(t) + " : verdict")
}

// Enable tries once to initialise nftables.
func Enable() error {
	enableOnce.Do(func() {
		err := preflight()
		if err != nil {
			log.G(context.Background()).WithError(err).Warnf("Failed to initialize nftables")
			nftEnableError = err
			return
		}
		if err := parseTemplate(); err != nil {
			log.G(context.Background()).WithError(err).Error("Internal error while initialising nftables")
			nftEnableError = fmt.Errorf("internal error while initialising nftables: %w", err)
			return
		}
		enabled = true
	})
	return nftEnableError
}

// Enabled returns true if [Enable] has been called and nftables was initialized successfully.
func Enabled() bool {
	return enabled
}

// Disable undoes Enable. Intended for unit testing.
func Disable() {
	enabled = false
	incrementalUpdateTempl = nil
	reloadTempl = nil
	enableOnce = sync.Once{}
}

// RunCmd runs arbitrary nftables ruleset commands, like `nft -f`, irrespective
// of whether nftables is [Enabled].
//
// Most users of this package should use [Table] and [Modifier] to manage their
// nftables ruleset.
func RunCmd(ctx context.Context, nftCmd []byte) error {
	h, err := newNftCtx()
	if err != nil {
		return err
	}
	defer h.Close()
	return h.Apply(ctx, nftCmd)
}

//////////////////////////////
// Tables

// table is the internal representation of an nftables table.
// Its elements need to be exported for use by text/template, but they should only be
// manipulated via exported methods.
type table struct {
	Name   string
	Family Family

	Maps   map[string]*nftMap
	Sets   map[string]*set
	Chains map[string]*chain

	DeleteCommands []string
	MustFlush      bool

	applyLock sync.Mutex
	nftHandle *nftCtx // applyLock must be held to access
}

// nftApply executes the nftables commands in nftCmd.
// Acquire t.applyLock before calling this function.
func (t *table) nftApply(ctx context.Context, nftCmd []byte) error {
	if t.nftHandle == nil {
		h, err := newNftCtx()
		if err != nil {
			return err
		}
		t.nftHandle = h
	}
	return t.nftHandle.Apply(ctx, nftCmd)
}

// Table is a handle for an nftables table.
type Table struct {
	t *table
}

// IsValid returns true if t is a valid reference to a table.
func (t Table) IsValid() bool {
	return t.t != nil
}

// NewTable creates a new nftables table and returns a [Table]
//
// See https://wiki.nftables.org/wiki-nftables/index.php/Configuring_tables
//
// To modify the table, instantiate a [Modifier], add commands to it, and call
// [Table.Apply].
//
// It's flushed in case it already exists in the host's nftables - when that
// happens, rules in its chains will be deleted but not the chains themselves,
// maps, sets, or elements of maps or sets. But, those un-flushed items can't do
// anything disruptive unless referred to by rules, and they will be flushed if
// they get re-created via the [Table], when [Table.Apply] is next called
// (so, before they can be used by a new rule).
//
// To fully delete an underlying nftables table, if one already exists,
// use [Table.Reload] after creating the table.
func NewTable(family Family, name string) (Table, error) {
	t := Table{
		t: &table{
			Name:      name,
			Family:    family,
			Maps:      map[string]*nftMap{},
			Sets:      map[string]*set{},
			Chains:    map[string]*chain{},
			MustFlush: true,
		},
	}
	return t, nil
}

// Close releases resources associated with the table. It does not modify or delete
// the underlying nftables table.
func (t Table) Close() error {
	if t.IsValid() {
		t.t.applyLock.Lock()
		defer t.t.applyLock.Unlock()
		if t.t.nftHandle != nil {
			t.t.nftHandle.Close()
			t.t.nftHandle = nil
		}
		t.t = nil
	}
	return nil
}

// Name returns the name of the table, or an empty string if t is not valid.
func (t Table) Name() string {
	if !t.IsValid() {
		return ""
	}
	return t.t.Name
}

// Family returns the address family of the nftables table described by [TableRef].
func (t Table) Family() Family {
	if !t.IsValid() {
		return ""
	}
	return t.t.Family
}

// SetBaseChainPolicy sets the default policy for a base chain. The update
// is applied immediately, unlike creation/deletion of objects via a [Modifier]
// which are not applied until [Modifier.Apply] is called.
func (t Table) SetBaseChainPolicy(ctx context.Context, chainName string, policy BaseChainPolicy) error {
	if !t.IsValid() {
		return errors.New("invalid table")
	}
	c := t.t.Chains[chainName]
	if c == nil {
		return fmt.Errorf("cannot set base chain policy for '%s', it does not exist", chainName)
	}
	if c.ChainType == "" {
		return fmt.Errorf("cannot set base chain policy for '%s', it is not a base chain", chainName)
	}
	oldPolicy := c.Policy
	c.Policy = policy
	c.MustFlush = true

	if err := t.Apply(ctx, Modifier{}); err != nil {
		c.Policy = oldPolicy
		return err
	}
	return nil
}

// Obj is an object that can be given to a [Modifier], representing an
// nftables object for it to create or delete.
type Obj interface {
	create(context.Context, *table) (bool, error)
	delete(context.Context, *table) (bool, error)
}

// Modifier is used to apply changes to a Table.
type Modifier struct {
	cmds []command
}

// Create enqueues creation of object o, to be applied by tm.Apply.
func (tm *Modifier) Create(o Obj) {
	tm.create(o, 1)
}

func (tm *Modifier) create(o Obj, skipFrames int) {
	_, f, l, _ := runtime.Caller(skipFrames + 1)
	tm.cmds = append(tm.cmds, command{
		obj:        o,
		callerFile: f,
		callerLine: l,
	})
}

// Delete enqueues deletion of object o, to be applied by tm.Apply.
func (tm *Modifier) Delete(o Obj) {
	_, f, l, _ := runtime.Caller(1)
	tm.cmds = append(tm.cmds, command{
		obj:        o,
		delete:     true,
		callerFile: f,
		callerLine: l,
	})
}

// Reverse returns a Modifier that will undo the actions of tm.
// Its operations are performed in reverse order, creates become
// deletes, and deletes become creates.
//
// Most operations are fully reversible (chains/maps/sets must be
// empty before they're deleted, so no information is lost). But,
// there are exceptions, noted in comments in the object definitions.
//
// Applying the updates in a reversed modifier may not work if
// any of the objects have been removed or modified since they
// were added. For example, if a Modifier creates a chain then another
// Modifier adds rules, the reversed Modifier will not be able to
// delete the chain as it is not empty.
func (tm *Modifier) Reverse() Modifier {
	rtm := Modifier{
		cmds: make([]command, len(tm.cmds)),
	}
	for i, cmd := range tm.cmds {
		cmd.delete = !cmd.delete
		rtm.cmds[len(tm.cmds)-i-1] = cmd
	}
	return rtm
}

var incrementalUpdateFailedHook func(applied string, err error)

// Apply makes incremental updates to nftables. If there's a validation
// error in any of the enqueued objects, or an error applying the updates
// to the underlying nftables, the [Table] will be unmodified.
func (t *Table) Apply(ctx context.Context, tm ...Modifier) (retErr error) {
	if !Enabled() {
		return errors.New("nftables is not enabled")
	}
	t.t.applyLock.Lock()
	defer t.t.applyLock.Unlock()

	var rollback []command
	defer func() {
		if retErr == nil {
			return
		}
		for _, c := range slices.Backward(rollback) {
			if _, err := c.rollback(ctx, t.t); err != nil {
				log.G(ctx).WithError(err).Error("Failed to roll back nftables updates")
			}
		}
		t.t.updatesApplied()
	}()

	// Apply tm's updates to the Table.
	for _, tmm := range tm {
		for _, cmd := range tmm.cmds {
			applied, err := cmd.apply(ctx, t.t)
			if err != nil {
				return fmt.Errorf("rule from %s:%d: %w", cmd.callerFile, cmd.callerLine, err)
			}
			if applied {
				rollback = append(rollback, cmd)
			}
		}
	}

	// Update nftables.
	var buf bytes.Buffer
	if err := incrementalUpdateTempl.Execute(&buf, t.t); err != nil {
		return fmt.Errorf("failed to execute template nft ruleset: %w", err)
	}

	if err := t.t.nftApply(ctx, buf.Bytes()); err != nil {
		// On error, log a line-numbered version of the generated "nft" input (because
		// nft error messages refer to line numbers).
		var sb strings.Builder
		for i, line := range bytes.SplitAfter(buf.Bytes(), []byte("\n")) {
			sb.WriteString(strconv.Itoa(i + 1))
			sb.WriteString(":\t")
			sb.Write(line)
		}
		log.G(ctx).Error("nftables: failed to update nftables:\n", sb.String(), "\n", err)
		if incrementalUpdateFailedHook != nil {
			incrementalUpdateFailedHook(sb.String(), err)
		}

		// It's possible something destructive has happened to nftables. For example, in
		// integration-cli tests, tests start daemons in the same netns as the integration
		// test's own daemon. They don't always use their own daemon, but they tend to leave
		// behind networks for the test infrastructure to clean up between tests. Starting
		// a daemon flushes the "docker-bridges" table, so the cleanup fails to delete a
		// rule that's been flushed. So, try reloading the whole table to get back in-sync.
		return t.t.reload(ctx)
	}

	// Note that updates have been applied.
	t.t.updatesApplied()
	return nil
}

// Reload deletes the table, then re-creates it, atomically.
func (t Table) Reload(ctx context.Context) error {
	if !Enabled() {
		return errors.New("nftables is not enabled")
	}
	if !t.IsValid() {
		return errors.New("invalid table")
	}
	t.t.applyLock.Lock()
	defer t.t.applyLock.Unlock()
	return t.t.reload(ctx)
}

func (t *table) reload(ctx context.Context) error {
	if !Enabled() {
		return errors.New("nftables is not enabled")
	}
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{"table": t.Name, "family": t.Family}))
	log.G(ctx).Warn("nftables: reloading table")

	// Build the update.
	var buf bytes.Buffer
	if err := reloadTempl.Execute(&buf, t); err != nil {
		return fmt.Errorf("failed to execute reload template: %w", err)
	}

	if err := t.nftApply(ctx, buf.Bytes()); err != nil {
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
	t.updatesApplied()
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
	Device     string
	Priority   int
	Policy     BaseChainPolicy
	MustFlush  bool
	ruleGroups map[RuleGroup][]string
}

// BaseChain constructs a new nftables base chain and returns a [ChainRef].
//
// See https://wiki.nftables.org/wiki-nftables/index.php/Configuring_chains#Adding_base_chains
//
// It is an error to create a base chain that already exists.
// If the underlying chain already exists, it will be flushed by the
// next [Table.Apply] before new rules are added.
type BaseChain struct {
	Name      string
	ChainType BaseChainType
	Hook      BaseChainHook
	Device    string
	Priority  int
	Policy    BaseChainPolicy // Defaults to BaseChainPolicyAccept
}

func (cd BaseChain) create(ctx context.Context, t *table) (bool, error) {
	if _, ok := t.Chains[cd.Name]; ok {
		return false, fmt.Errorf("base chain '%s' already exists", cd.Name)
	}
	if cd.Name == "" {
		return false, errors.New("base chain must have a name")
	}
	if cd.ChainType == "" || cd.Hook == "" {
		return false, fmt.Errorf("chain '%s': fields ChainType and Hook are required", cd.Name)
	}
	if cd.Policy == "" {
		// nftables will default to "accept" if unspecified, but the text/template
		// requires a policy string.
		cd.Policy = BaseChainPolicyAccept
	}
	c := &chain{
		table:      t,
		Name:       cd.Name,
		ChainType:  cd.ChainType,
		Hook:       cd.Hook,
		Device:     cd.Device,
		Priority:   cd.Priority,
		Policy:     cd.Policy,
		MustFlush:  true,
		ruleGroups: map[RuleGroup][]string{},
	}
	t.Chains[c.Name] = c
	log.G(ctx).WithFields(log.Fields{
		"family": t.Family,
		"table":  t.Name,
		"chain":  c.Name,
		"type":   c.ChainType,
		"hook":   c.Hook,
		"device": c.Device,
		"prio":   c.Priority,
	}).Debug("nftables: created base chain")
	return true, nil
}

func (cd BaseChain) delete(ctx context.Context, t *table) (bool, error) {
	return t.deleteChain(ctx, cd.Name)
}

// Chain implements the [Obj] interface, it can be passed to a
// [Modifier] to create or delete a chain.
type Chain struct {
	Name string
}

func (cd Chain) create(ctx context.Context, t *table) (bool, error) {
	if _, ok := t.Chains[cd.Name]; ok {
		return false, fmt.Errorf("chain '%s' already exists", cd.Name)
	}
	if cd.Name == "" {
		return false, errors.New("chain must have a name")
	}
	c := &chain{
		table:      t,
		Name:       cd.Name,
		MustFlush:  true,
		ruleGroups: map[RuleGroup][]string{},
	}
	t.Chains[c.Name] = c
	log.G(ctx).WithFields(log.Fields{
		"family": t.Family,
		"table":  t.Name,
		"chain":  cd.Name,
	}).Debug("nftables: created chain")
	return true, nil
}

func (cd Chain) delete(ctx context.Context, t *table) (bool, error) {
	return t.deleteChain(ctx, cd.Name)
}

// Rule implements the [Obj] interface, it can be passed to a
// [Modifier] to create or delete a rule in a chain.
type Rule struct {
	Chain string
	Group RuleGroup
	Rule  []string
	// IgnoreExist suppresses errors about deleting a rule that does not exist
	// or creating a rule that does already exist.
	//
	// Note that, when set, reversing the [Modifier] may not do what you want! For
	// example, if the original modifier deleted a rule that did not exist, the
	// reversed modifier will create that rule.
	IgnoreExist bool
}

func (ru Rule) create(ctx context.Context, t *table) (bool, error) {
	c := t.Chains[ru.Chain]
	if c == nil {
		return false, fmt.Errorf("chain '%s' does not exist", ru.Chain)
	}
	if len(ru.Rule) == 0 {
		return false, fmt.Errorf("chain '%s', cannot add empty rule", ru.Chain)
	}
	rule := strings.Join(ru.Rule, " ")
	if rg, ok := c.ruleGroups[ru.Group]; ok && slices.Contains(rg, rule) {
		if !ru.IgnoreExist {
			return false, fmt.Errorf("adding rule:'%s' chain:'%s' group:%d: rule exists", rule, ru.Chain, ru.Group)
		}
		return false, nil
	}
	c.ruleGroups[ru.Group] = append(c.ruleGroups[ru.Group], rule)
	c.MustFlush = true
	log.G(ctx).WithFields(log.Fields{
		"family": t.Family,
		"table":  t.Name,
		"chain":  c.Name,
		"group":  ru.Group,
		"rule":   rule,
	}).Debug("nftables: appended rule")
	return true, nil
}

func (ru Rule) delete(ctx context.Context, t *table) (bool, error) {
	rule := strings.Join(ru.Rule, " ")
	c := t.Chains[ru.Chain]
	if c == nil {
		return false, fmt.Errorf("deleting rule:'%s' - chain '%s' does not exist", rule, ru.Chain)
	}
	if rule == "" {
		return false, fmt.Errorf("chain '%s', cannot delete empty rule", ru.Chain)
	}
	rg, ok := c.ruleGroups[ru.Group]
	if !ok {
		if !ru.IgnoreExist {
			return false, fmt.Errorf("deleting rule:'%s' chain:'%s' rule group:%d does not exist", rule, ru.Chain, ru.Group)
		}
		return false, nil
	}
	origLen := len(rg)
	c.ruleGroups[ru.Group] = slices.DeleteFunc(rg, func(r string) bool { return r == rule })
	if len(c.ruleGroups[ru.Group]) == origLen {
		if !ru.IgnoreExist {
			return false, fmt.Errorf("deleting rule:'%s' chain:'%s' group:%d: rule does not exist", rule, ru.Chain, ru.Group)
		}
		return false, nil
	}
	if len(c.ruleGroups[ru.Group]) == 0 {
		delete(c.ruleGroups, ru.Group)
	}
	c.MustFlush = true
	log.G(ctx).WithFields(log.Fields{
		"family": t.Family,
		"table":  t.Name,
		"chain":  c.Name,
		"rule":   rule,
	}).Debug("nftables: deleted rule")
	return true, nil
}

// ////////////////////////////
// Maps

type mapValue struct {
	Value   string
	Comment string
}

// nftMap is the internal representation of an nftables map (including verdict maps).
// Its elements need to be exported for use by text/template, but they should only be
// manipulated via exported methods.
type nftMap struct {
	table           *table
	Name            string
	ElementTypeExpr string
	Flags           []string
	Size            int
	Counter         bool
	Timeout         time.Duration
	Elements        map[string]mapValue
	AddedElements   map[string]mapValue
	DeletedElements map[string]string
	MustFlush       bool
}

// Map implements the [Obj] interface, it can be passed to a
// [Modifier] to create or delete a map.
type Map struct {
	Name        string
	ElementType MapTyper
	Flags       []string
	Size        int
	Counter     bool
	Timeout     time.Duration
}

func (m Map) create(ctx context.Context, t *table) (bool, error) {
	if m.Name == "" {
		return false, errors.New("map must have a name")
	}
	if _, ok := t.Maps[m.Name]; ok {
		return false, fmt.Errorf("map '%s' already exists", m.Name)
	}
	if m.ElementType == nil {
		return false, fmt.Errorf("map '%s' has no element type", m.Name)
	}
	nm := &nftMap{
		table:           t,
		Name:            m.Name,
		ElementTypeExpr: m.ElementType.mapType(),
		Flags:           slices.Clone(m.Flags),
		Size:            m.Size,
		Counter:         m.Counter,
		Timeout:         m.Timeout,
		Elements:        map[string]mapValue{},
		AddedElements:   map[string]mapValue{},
		DeletedElements: map[string]string{},
		MustFlush:       true,
	}
	t.Maps[nm.Name] = nm
	log.G(ctx).WithFields(log.Fields{
		"family": t.Family,
		"table":  t.Name,
		"map":    nm.Name,
	}).Debug("nftables: created map")
	return true, nil
}

func (m Map) delete(ctx context.Context, t *table) (bool, error) {
	nm := t.Maps[m.Name]
	if nm == nil {
		return false, fmt.Errorf("cannot delete map '%s', it does not exist", m.Name)
	}
	if len(nm.Elements) != 0 {
		return false, fmt.Errorf("cannot delete map '%s', it contains %d elements", nm.Name, len(nm.Elements))
	}
	delete(t.Maps, nm.Name)
	t.DeleteCommands = append(t.DeleteCommands,
		fmt.Sprintf("delete map %s %s %s", t.Family, t.Name, nm.Name))
	log.G(ctx).WithFields(log.Fields{
		"family": t.Family,
		"table":  t.Name,
		"map":    nm.Name,
	}).Debug("nftables: deleted map")
	return true, nil
}

// MapElement implements the [Obj] interface, it can be passed to a
// [Modifier] to create or delete an entry in a map.
type MapElement struct {
	MapName string
	Key     string
	Value   string
	Comment string
}

func (me MapElement) create(ctx context.Context, t *table) (bool, error) {
	if me.MapName == "" {
		return false, errors.New("cannot add element to unnamed map")
	}
	if bad := validateComment(me.Comment); bad != nil {
		bad.kind = fmt.Sprintf("map '%s' element", me.MapName)
		bad.name = me.Key
		return false, bad
	}
	nm := t.Maps[me.MapName]
	if nm == nil {
		return false, fmt.Errorf("cannot add to map '%s', it does not exist", me.MapName)
	}
	if me.Key == "" || me.Value == "" {
		return false, fmt.Errorf("cannot add to map '%s', element must have key and value", me.MapName)
	}
	if _, ok := nm.Elements[me.Key]; ok {
		return false, fmt.Errorf("map '%s' already contains element '%s'", me.MapName, me.Key)
	}
	nm.Elements[me.Key] = mapValue{
		Value:   me.Value,
		Comment: me.Comment,
	}
	nm.AddedElements[me.Key] = nm.Elements[me.Key]
	log.G(ctx).WithFields(log.Fields{
		"family":  t.Family,
		"table":   t.Name,
		"map":     me.MapName,
		"key":     me.Key,
		"value":   me.Value,
		"comment": me.Comment,
	}).Debug("nftables: added map element")
	return true, nil
}

func (me MapElement) delete(ctx context.Context, t *table) (bool, error) {
	nm := t.Maps[me.MapName]
	if nm == nil {
		return false, fmt.Errorf("cannot delete from map '%s', it does not exist", me.MapName)
	}
	oldValue, ok := nm.Elements[me.Key]
	if !ok {
		return false, fmt.Errorf("map '%s' does not contain element '%s'", me.MapName, me.Key)
	}
	if oldValue.Value != me.Value {
		return false, fmt.Errorf("cannot delete map '%s' element '%s', value was '%s', not '%s'",
			me.MapName, me.Key, oldValue.Value, me.Value)
	}
	delete(nm.Elements, me.Key)
	nm.DeletedElements[me.Key] = me.Value
	log.G(ctx).WithFields(log.Fields{
		"family":  t.Family,
		"table":   t.Name,
		"map":     me.MapName,
		"key":     me.Key,
		"value":   me.Value,
		"comment": oldValue.Comment,
	}).Debug("nftables: deleted map element")
	return true, nil
}

// ////////////////////////////
// Sets

type setElementOptions struct {
	Comment string
}

// set is the internal representation of an nftables set.
// Its elements need to be exported for use by text/template, but they should only be
// manipulated via exported methods.
type set struct {
	table           *table
	Name            string
	ElementTypeExpr string
	Flags           []string
	Size            int
	Counter         bool
	Timeout         time.Duration
	Elements        map[string]setElementOptions
	AddedElements   map[string]setElementOptions
	DeletedElements map[string]struct{}
	MustFlush       bool
}

// Set implements the [Obj] interface, it can be passed to a
// [Modifier] to create or delete a set.
type Set struct {
	Name        string
	ElementType SetTyper
	Flags       []string
	Size        int
	Counter     bool
	Timeout     time.Duration
}

// See https://wiki.nftables.org/wiki-nftables/index.php/Sets#Named_sets
func (sd Set) create(ctx context.Context, t *table) (bool, error) {
	if sd.Name == "" {
		return false, errors.New("set must have a name")
	}
	if _, ok := t.Sets[sd.Name]; ok {
		return false, fmt.Errorf("set '%s' already exists", sd.Name)
	}
	if sd.ElementType == nil {
		return false, fmt.Errorf("set '%s' must have a type", sd.Name)
	}
	s := &set{
		table:           t,
		Name:            sd.Name,
		Elements:        map[string]setElementOptions{},
		ElementTypeExpr: sd.ElementType.setType(),
		Flags:           slices.Clone(sd.Flags),
		Size:            sd.Size,
		Counter:         sd.Counter,
		Timeout:         sd.Timeout,
		AddedElements:   map[string]setElementOptions{},
		DeletedElements: map[string]struct{}{},
		MustFlush:       true,
	}
	t.Sets[sd.Name] = s
	log.G(ctx).WithFields(log.Fields{
		"family": t.Family,
		"table":  t.Name,
		"set":    s.Name,
	}).Debug("nftables: created set")
	return true, nil
}

func (sd Set) delete(ctx context.Context, t *table) (bool, error) {
	s := t.Sets[sd.Name]
	if s == nil {
		return false, fmt.Errorf("cannot delete set '%s', it does not exist", sd.Name)
	}
	if len(s.Elements) != 0 {
		return false, fmt.Errorf("cannot delete set '%s', it contains %d elements", s.Name, len(s.Elements))
	}
	delete(t.Sets, sd.Name)
	t.DeleteCommands = append(t.DeleteCommands,
		fmt.Sprintf("delete set %s %s %s", t.Family, t.Name, s.Name))
	log.G(ctx).WithFields(log.Fields{
		"family": t.Family,
		"table":  t.Name,
		"set":    sd.Name,
	}).Debug("nftables: deleted set")
	return true, nil
}

// SetElement implements the [Obj] interface, it can be passed to a
// [Modifier] to create or delete an entry in a set.
type SetElement struct {
	SetName string
	Element string
	Comment string
	// If true, deleting an element that does not exist or creating an
	// element that already exists will succeed.
	Idempotent bool
}

func (se SetElement) create(ctx context.Context, t *table) (bool, error) {
	s := t.Sets[se.SetName]
	if s == nil {
		return false, fmt.Errorf("cannot add to set '%s', it does not exist", se.SetName)
	}
	if se.Element == "" {
		return false, fmt.Errorf("cannot add to set '%s', element not specified", se.SetName)
	}
	if bad := validateComment(se.Comment); bad != nil {
		bad.kind = fmt.Sprintf("set '%s' element", se.SetName)
		bad.name = se.Element
		return false, bad
	}
	if _, ok := s.Elements[se.Element]; ok {
		if se.Idempotent {
			return false, nil
		}
		return false, fmt.Errorf("set '%s' already contains element '%s'", s.Name, se.Element)
	}
	s.Elements[se.Element] = setElementOptions{
		Comment: se.Comment,
	}
	s.AddedElements[se.Element] = s.Elements[se.Element]
	delete(s.DeletedElements, se.Element)
	log.G(ctx).WithFields(log.Fields{
		"family":  t.Family,
		"table":   t.Name,
		"set":     s.Name,
		"element": se.Element,
		"comment": se.Comment,
	}).Debug("nftables: added set element")
	return true, nil
}

func (se SetElement) delete(ctx context.Context, t *table) (bool, error) {
	s := t.Sets[se.SetName]
	if s == nil {
		return false, fmt.Errorf("cannot delete from set '%s', it does not exist", se.SetName)
	}
	oldValue, ok := s.Elements[se.Element]
	if !ok {
		if se.Idempotent {
			return false, nil
		}
		return false, fmt.Errorf("cannot delete '%s' from set '%s', it does not exist", se.Element, s.Name)
	}
	delete(s.Elements, se.Element)
	delete(s.AddedElements, se.Element)
	s.DeletedElements[se.Element] = struct{}{}
	log.G(ctx).WithFields(log.Fields{
		"family":  t.Family,
		"table":   t.Name,
		"set":     s.Name,
		"element": se.Element,
		"comment": oldValue.Comment,
	}).Debug("nftables: deleted set element")
	return true, nil
}

// ////////////////////////////
// Internal

func (t *table) deleteChain(ctx context.Context, name string) (bool, error) {
	c := t.Chains[name]
	if c == nil {
		return false, fmt.Errorf("cannot delete chain '%s', it does not exist", name)
	}
	if len(c.ruleGroups) != 0 {
		return false, fmt.Errorf("cannot delete chain '%s', it is not empty", name)
	}
	delete(t.Chains, name)
	t.DeleteCommands = append(t.DeleteCommands,
		fmt.Sprintf("delete chain %s %s %s", t.Family, t.Name, name))
	log.G(ctx).WithFields(log.Fields{
		"family": t.Family,
		"table":  t.Name,
		"chain":  name,
	}).Debug("nftables: deleted chain")
	return true, nil
}

type command struct {
	obj        Obj
	callerFile string
	callerLine int
	delete     bool
}

func (c command) apply(ctx context.Context, t *table) (bool, error) {
	if c.delete {
		return c.obj.delete(ctx, t)
	}
	return c.obj.create(ctx, t)
}

func (c command) rollback(ctx context.Context, t *table) (bool, error) {
	if c.delete {
		return c.obj.create(ctx, t)
	}
	return c.obj.delete(ctx, t)
}

func (t *table) updatesApplied() {
	t.DeleteCommands = t.DeleteCommands[:0]
	for _, c := range t.Chains {
		c.MustFlush = false
	}
	for _, m := range t.Maps {
		m.AddedElements = map[string]mapValue{}
		m.DeletedElements = map[string]string{}
		m.MustFlush = false
	}
	for _, s := range t.Sets {
		s.AddedElements = map[string]setElementOptions{}
		s.DeletedElements = map[string]struct{}{}
		s.MustFlush = false
	}
	t.MustFlush = false
}

// Rules returns an iterator that yields the chain's rules in order.
func (c *chain) Rules() iter.Seq[string] {
	groups := make([]RuleGroup, 0, len(c.ruleGroups))
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

type badCommentError struct {
	kind, name, disallowed string
}

func (e badCommentError) Error() string {
	return fmt.Sprintf("%s '%s' comment contains %q which is not permitted in nftables comments",
		e.kind, e.name, e.disallowed)
}

// validateComment checks whether s would break the rendered nftables templates
// if interpolated as a comment.
//
// The nftables ruleset syntax does not support
// escaping of characters in quoted strings; comments simply cannot contain
// double-quote or newline characters.
func validateComment(s string) *badCommentError {
	i := strings.IndexAny(s, "\r\n\"")
	if i == -1 {
		return nil
	}
	return &badCommentError{
		disallowed: strings.Clone(s[i : i+1]),
	}
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
//
//go:embed incremental_update.nft.gotmpl
var incrementalUpdateTemplText string

// reloadTemplText is used with text/template to generate an nftables command file
// (which will be applied atomically), to fully re-create a table.
//
// It first declares the table so if it doesn't already exist, it can be deleted.
// Then it deletes the table and re-creates it.
//
//go:embed reload.nft.gotmpl
var reloadTemplText string

func parseTemplate() error {
	var errs [2]error
	incrementalUpdateTempl, errs[0] = template.New("incremental_update.nft.gotmpl").Funcs(templateFuncs).Parse(incrementalUpdateTemplText)
	reloadTempl, errs[1] = template.New("reload.nft.gotmpl").Funcs(templateFuncs).Parse(reloadTemplText)
	return errors.Join(errs[:]...)
}

var templateFuncs = template.FuncMap{
	"join": strings.Join,
}
