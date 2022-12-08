package netlink

import (
	"fmt"
	"net"
)

// Rule represents a netlink rule.
type Rule struct {
	Priority          int
	Family            int
	Table             int
	Mark              int
	Mask              int
	Tos               uint
	TunID             uint
	Goto              int
	Src               *net.IPNet
	Dst               *net.IPNet
	Flow              int
	IifName           string
	OifName           string
	SuppressIfgroup   int
	SuppressPrefixlen int
	Invert            bool
	Dport             *RulePortRange
	Sport             *RulePortRange
}

func (r Rule) String() string {
	return fmt.Sprintf("ip rule %d: from %s table %d", r.Priority, r.Src, r.Table)
}

// NewRule return empty rules.
func NewRule() *Rule {
	return &Rule{
		SuppressIfgroup:   -1,
		SuppressPrefixlen: -1,
		Priority:          -1,
		Mark:              -1,
		Mask:              -1,
		Goto:              -1,
		Flow:              -1,
	}
}

// NewRulePortRange creates rule sport/dport range.
func NewRulePortRange(start, end uint16) *RulePortRange {
	return &RulePortRange{Start: start, End: end}
}

// RulePortRange represents rule sport/dport range.
type RulePortRange struct {
	Start uint16
	End   uint16
}
