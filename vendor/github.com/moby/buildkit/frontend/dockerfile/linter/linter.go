package linter

import (
	"fmt"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/pkg/errors"
)

type Config struct {
	Warn          LintWarnFunc
	SkipRules     []string
	SkipAll       bool
	ReturnAsError bool
}

type Linter struct {
	SkippedRules  map[string]struct{}
	CalledRules   []string
	SkipAll       bool
	ReturnAsError bool
	Warn          LintWarnFunc
}

func New(config *Config) *Linter {
	toret := &Linter{
		SkippedRules: map[string]struct{}{},
		CalledRules:  []string{},
		Warn:         config.Warn,
	}
	toret.SkipAll = config.SkipAll
	toret.ReturnAsError = config.ReturnAsError
	for _, rule := range config.SkipRules {
		toret.SkippedRules[rule] = struct{}{}
	}
	return toret
}

func (lc *Linter) Run(rule LinterRuleI, location []parser.Range, txt ...string) {
	if lc == nil || lc.Warn == nil || lc.SkipAll {
		return
	}
	rulename := rule.RuleName()
	if _, ok := lc.SkippedRules[rulename]; ok {
		return
	}
	lc.CalledRules = append(lc.CalledRules, rulename)
	rule.Run(lc.Warn, location, txt...)
}

func (lc *Linter) Error() error {
	if lc == nil || !lc.ReturnAsError {
		return nil
	}
	if len(lc.CalledRules) == 0 {
		return nil
	}
	var rules []string
	uniqueRules := map[string]struct{}{}
	for _, r := range lc.CalledRules {
		uniqueRules[r] = struct{}{}
	}
	for r := range uniqueRules {
		rules = append(rules, r)
	}
	return errors.Errorf("lint violation found for rules: %s", strings.Join(rules, ", "))
}

type LinterRuleI interface {
	RuleName() string
	Run(warn LintWarnFunc, location []parser.Range, txt ...string)
}

type LinterRule[F any] struct {
	Name        string
	Description string
	URL         string
	Format      F
}

func (rule *LinterRule[F]) RuleName() string {
	return rule.Name
}

func (rule *LinterRule[F]) Run(warn LintWarnFunc, location []parser.Range, txt ...string) {
	if len(txt) == 0 {
		txt = []string{rule.Description}
	}
	short := strings.Join(txt, " ")
	warn(rule.Name, rule.Description, rule.URL, short, location)
}

func LintFormatShort(rulename, msg string, line int) string {
	msg = fmt.Sprintf("%s: %s", rulename, msg)
	if line > 0 {
		msg = fmt.Sprintf("%s (line %d)", msg, line)
	}
	return msg
}

type LintWarnFunc func(rulename, description, url, fmtmsg string, location []parser.Range)
