package netlink

import (
	"net"
	"syscall"
	"testing"
)

func TestRuleAddDel(t *testing.T) {
	srcNet := &net.IPNet{IP: net.IPv4(172, 16, 0, 1), Mask: net.CIDRMask(16, 32)}
	dstNet := &net.IPNet{IP: net.IPv4(172, 16, 1, 1), Mask: net.CIDRMask(24, 32)}

	rules_begin, err := RuleList(syscall.AF_INET)
	if err != nil {
		t.Fatal(err)
	}

	rule := NewRule()
	rule.Table = syscall.RT_TABLE_MAIN
	rule.Src = srcNet
	rule.Dst = dstNet
	rule.Priority = 5
	rule.OifName = "lo"
	rule.IifName = "lo"
	if err := RuleAdd(rule); err != nil {
		t.Fatal(err)
	}

	rules, err := RuleList(syscall.AF_INET)
	if err != nil {
		t.Fatal(err)
	}

	if len(rules) != len(rules_begin)+1 {
		t.Fatal("Rule not added properly")
	}

	// find this rule
	var found bool
	for i := range rules {
		if rules[i].Table == rule.Table &&
			rules[i].Src != nil && rules[i].Src.String() == srcNet.String() &&
			rules[i].Dst != nil && rules[i].Dst.String() == dstNet.String() &&
			rules[i].OifName == rule.OifName &&
			rules[i].Priority == rule.Priority &&
			rules[i].IifName == rule.IifName {
			found = true
		}
	}
	if !found {
		t.Fatal("Rule has diffrent options than one added")
	}

	if err := RuleDel(rule); err != nil {
		t.Fatal(err)
	}

	rules_end, err := RuleList(syscall.AF_INET)
	if err != nil {
		t.Fatal(err)
	}

	if len(rules_end) != len(rules_begin) {
		t.Fatal("Rule not removed properly")
	}
}
