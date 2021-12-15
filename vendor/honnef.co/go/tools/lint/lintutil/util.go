// Copyright (c) 2013 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

// Package lintutil provides helpers for writing linter command lines.
package lintutil // import "honnef.co/go/tools/lint/lintutil"

import (
	"errors"
	"flag"
	"fmt"
	"go/build"
	"go/token"
	"log"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"honnef.co/go/tools/config"
	"honnef.co/go/tools/lint"
	"honnef.co/go/tools/lint/lintutil/format"
	"honnef.co/go/tools/version"

	"golang.org/x/tools/go/packages"
)

func usage(name string, flags *flag.FlagSet) func() {
	return func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", name)
		fmt.Fprintf(os.Stderr, "\t%s [flags] # runs on package in current directory\n", name)
		fmt.Fprintf(os.Stderr, "\t%s [flags] packages\n", name)
		fmt.Fprintf(os.Stderr, "\t%s [flags] directory\n", name)
		fmt.Fprintf(os.Stderr, "\t%s [flags] files... # must be a single package\n", name)
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flags.PrintDefaults()
	}
}

func parseIgnore(s string) ([]lint.Ignore, error) {
	var out []lint.Ignore
	if len(s) == 0 {
		return nil, nil
	}
	for _, part := range strings.Fields(s) {
		p := strings.Split(part, ":")
		if len(p) != 2 {
			return nil, errors.New("malformed ignore string")
		}
		path := p[0]
		checks := strings.Split(p[1], ",")
		out = append(out, &lint.GlobIgnore{Pattern: path, Checks: checks})
	}
	return out, nil
}

type versionFlag int

func (v *versionFlag) String() string {
	return fmt.Sprintf("1.%d", *v)
}

func (v *versionFlag) Set(s string) error {
	if len(s) < 3 {
		return errors.New("invalid Go version")
	}
	if s[0] != '1' {
		return errors.New("invalid Go version")
	}
	if s[1] != '.' {
		return errors.New("invalid Go version")
	}
	i, err := strconv.Atoi(s[2:])
	*v = versionFlag(i)
	return err
}

func (v *versionFlag) Get() interface{} {
	return int(*v)
}

type list []string

func (list *list) String() string {
	return `"` + strings.Join(*list, ",") + `"`
}

func (list *list) Set(s string) error {
	if s == "" {
		*list = nil
		return nil
	}

	*list = strings.Split(s, ",")
	return nil
}

func FlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Usage = usage(name, flags)
	flags.String("tags", "", "List of `build tags`")
	flags.String("ignore", "", "Deprecated: use linter directives instead")
	flags.Bool("tests", true, "Include tests")
	flags.Bool("version", false, "Print version and exit")
	flags.Bool("show-ignored", false, "Don't filter ignored problems")
	flags.String("f", "text", "Output `format` (valid choices are 'stylish', 'text' and 'json')")
	flags.String("explain", "", "Print description of `check`")

	flags.Int("debug.max-concurrent-jobs", 0, "Number of jobs to run concurrently")
	flags.Bool("debug.print-stats", false, "Print debug statistics")
	flags.String("debug.cpuprofile", "", "Write CPU profile to `file`")
	flags.String("debug.memprofile", "", "Write memory profile to `file`")

	checks := list{"inherit"}
	fail := list{"all"}
	flags.Var(&checks, "checks", "Comma-separated list of `checks` to enable.")
	flags.Var(&fail, "fail", "Comma-separated list of `checks` that can cause a non-zero exit status.")

	tags := build.Default.ReleaseTags
	v := tags[len(tags)-1][2:]
	version := new(versionFlag)
	if err := version.Set(v); err != nil {
		panic(fmt.Sprintf("internal error: %s", err))
	}

	flags.Var(version, "go", "Target Go `version` in the format '1.x'")
	return flags
}

func findCheck(cs []lint.Checker, check string) (lint.Check, bool) {
	for _, c := range cs {
		for _, cc := range c.Checks() {
			if cc.ID == check {
				return cc, true
			}
		}
	}
	return lint.Check{}, false
}

func ProcessFlagSet(cs []lint.Checker, fs *flag.FlagSet) {
	if _, ok := os.LookupEnv("GOGC"); !ok {
		debug.SetGCPercent(50)
	}

	tags := fs.Lookup("tags").Value.(flag.Getter).Get().(string)
	ignore := fs.Lookup("ignore").Value.(flag.Getter).Get().(string)
	tests := fs.Lookup("tests").Value.(flag.Getter).Get().(bool)
	goVersion := fs.Lookup("go").Value.(flag.Getter).Get().(int)
	formatter := fs.Lookup("f").Value.(flag.Getter).Get().(string)
	printVersion := fs.Lookup("version").Value.(flag.Getter).Get().(bool)
	showIgnored := fs.Lookup("show-ignored").Value.(flag.Getter).Get().(bool)
	explain := fs.Lookup("explain").Value.(flag.Getter).Get().(string)

	maxConcurrentJobs := fs.Lookup("debug.max-concurrent-jobs").Value.(flag.Getter).Get().(int)
	printStats := fs.Lookup("debug.print-stats").Value.(flag.Getter).Get().(bool)
	cpuProfile := fs.Lookup("debug.cpuprofile").Value.(flag.Getter).Get().(string)
	memProfile := fs.Lookup("debug.memprofile").Value.(flag.Getter).Get().(string)

	cfg := config.Config{}
	cfg.Checks = *fs.Lookup("checks").Value.(*list)

	exit := func(code int) {
		if cpuProfile != "" {
			pprof.StopCPUProfile()
		}
		if memProfile != "" {
			f, err := os.Create(memProfile)
			if err != nil {
				panic(err)
			}
			runtime.GC()
			pprof.WriteHeapProfile(f)
		}
		os.Exit(code)
	}
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
	}

	if printVersion {
		version.Print()
		exit(0)
	}

	if explain != "" {
		check, ok := findCheck(cs, explain)
		if !ok {
			fmt.Fprintln(os.Stderr, "Couldn't find check", explain)
			exit(1)
		}
		if check.Doc == "" {
			fmt.Fprintln(os.Stderr, explain, "has no documentation")
			exit(1)
		}
		fmt.Println(check.Doc)
		exit(0)
	}

	ps, err := Lint(cs, fs.Args(), &Options{
		Tags:          strings.Fields(tags),
		LintTests:     tests,
		Ignores:       ignore,
		GoVersion:     goVersion,
		ReturnIgnored: showIgnored,
		Config:        cfg,

		MaxConcurrentJobs: maxConcurrentJobs,
		PrintStats:        printStats,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		exit(1)
	}

	var f format.Formatter
	switch formatter {
	case "text":
		f = format.Text{W: os.Stdout}
	case "stylish":
		f = &format.Stylish{W: os.Stdout}
	case "json":
		f = format.JSON{W: os.Stdout}
	default:
		fmt.Fprintf(os.Stderr, "unsupported output format %q\n", formatter)
		exit(2)
	}

	var (
		total    int
		errors   int
		warnings int
	)

	fail := *fs.Lookup("fail").Value.(*list)
	var allChecks []string
	for _, p := range ps {
		allChecks = append(allChecks, p.Check)
	}

	shouldExit := lint.FilterChecks(allChecks, fail)

	total = len(ps)
	for _, p := range ps {
		if shouldExit[p.Check] {
			errors++
		} else {
			p.Severity = lint.Warning
			warnings++
		}
		f.Format(p)
	}
	if f, ok := f.(format.Statter); ok {
		f.Stats(total, errors, warnings)
	}
	if errors > 0 {
		exit(1)
	}
}

type Options struct {
	Config config.Config

	Tags          []string
	LintTests     bool
	Ignores       string
	GoVersion     int
	ReturnIgnored bool

	MaxConcurrentJobs int
	PrintStats        bool
}

func Lint(cs []lint.Checker, paths []string, opt *Options) ([]lint.Problem, error) {
	stats := lint.PerfStats{
		CheckerInits: map[string]time.Duration{},
	}

	if opt == nil {
		opt = &Options{}
	}
	ignores, err := parseIgnore(opt.Ignores)
	if err != nil {
		return nil, err
	}

	conf := &packages.Config{
		Mode:  packages.LoadAllSyntax,
		Tests: opt.LintTests,
		BuildFlags: []string{
			"-tags=" + strings.Join(opt.Tags, " "),
		},
	}

	t := time.Now()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	pkgs, err := packages.Load(conf, paths...)
	if err != nil {
		return nil, err
	}
	stats.PackageLoading = time.Since(t)
	runtime.GC()

	var problems []lint.Problem
	workingPkgs := make([]*packages.Package, 0, len(pkgs))
	for _, pkg := range pkgs {
		if pkg.IllTyped {
			problems = append(problems, compileErrors(pkg)...)
		} else {
			workingPkgs = append(workingPkgs, pkg)
		}
	}

	if len(workingPkgs) == 0 {
		return problems, nil
	}

	l := &lint.Linter{
		Checkers:      cs,
		Ignores:       ignores,
		GoVersion:     opt.GoVersion,
		ReturnIgnored: opt.ReturnIgnored,
		Config:        opt.Config,

		MaxConcurrentJobs: opt.MaxConcurrentJobs,
		PrintStats:        opt.PrintStats,
	}
	problems = append(problems, l.Lint(workingPkgs, &stats)...)

	return problems, nil
}

var posRe = regexp.MustCompile(`^(.+?):(\d+)(?::(\d+)?)?$`)

func parsePos(pos string) token.Position {
	if pos == "-" || pos == "" {
		return token.Position{}
	}
	parts := posRe.FindStringSubmatch(pos)
	if parts == nil {
		panic(fmt.Sprintf("internal error: malformed position %q", pos))
	}
	file := parts[1]
	line, _ := strconv.Atoi(parts[2])
	col, _ := strconv.Atoi(parts[3])
	return token.Position{
		Filename: file,
		Line:     line,
		Column:   col,
	}
}

func compileErrors(pkg *packages.Package) []lint.Problem {
	if !pkg.IllTyped {
		return nil
	}
	if len(pkg.Errors) == 0 {
		// transitively ill-typed
		var ps []lint.Problem
		for _, imp := range pkg.Imports {
			ps = append(ps, compileErrors(imp)...)
		}
		return ps
	}
	var ps []lint.Problem
	for _, err := range pkg.Errors {
		p := lint.Problem{
			Position: parsePos(err.Pos),
			Text:     err.Msg,
			Check:    "compile",
		}
		ps = append(ps, p)
	}
	return ps
}

func ProcessArgs(name string, cs []lint.Checker, args []string) {
	flags := FlagSet(name)
	flags.Parse(args)

	ProcessFlagSet(cs, flags)
}
