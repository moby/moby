// Package lint provides the foundation for tools like staticcheck
package lint // import "honnef.co/go/tools/lint"

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/packages"
	"honnef.co/go/tools/config"
	"honnef.co/go/tools/ssa"
	"honnef.co/go/tools/ssa/ssautil"
)

type Job struct {
	Pkg       *Pkg
	GoVersion int

	check    Check
	problems []Problem

	duration time.Duration
}

type Ignore interface {
	Match(p Problem) bool
}

type LineIgnore struct {
	File    string
	Line    int
	Checks  []string
	matched bool
	pos     token.Pos
}

func (li *LineIgnore) Match(p Problem) bool {
	if p.Position.Filename != li.File || p.Position.Line != li.Line {
		return false
	}
	for _, c := range li.Checks {
		if m, _ := filepath.Match(c, p.Check); m {
			li.matched = true
			return true
		}
	}
	return false
}

func (li *LineIgnore) String() string {
	matched := "not matched"
	if li.matched {
		matched = "matched"
	}
	return fmt.Sprintf("%s:%d %s (%s)", li.File, li.Line, strings.Join(li.Checks, ", "), matched)
}

type FileIgnore struct {
	File   string
	Checks []string
}

func (fi *FileIgnore) Match(p Problem) bool {
	if p.Position.Filename != fi.File {
		return false
	}
	for _, c := range fi.Checks {
		if m, _ := filepath.Match(c, p.Check); m {
			return true
		}
	}
	return false
}

type GlobIgnore struct {
	Pattern string
	Checks  []string
}

func (gi *GlobIgnore) Match(p Problem) bool {
	if gi.Pattern != "*" {
		pkgpath := p.Package.Types.Path()
		if strings.HasSuffix(pkgpath, "_test") {
			pkgpath = pkgpath[:len(pkgpath)-len("_test")]
		}
		name := filepath.Join(pkgpath, filepath.Base(p.Position.Filename))
		if m, _ := filepath.Match(gi.Pattern, name); !m {
			return false
		}
	}
	for _, c := range gi.Checks {
		if m, _ := filepath.Match(c, p.Check); m {
			return true
		}
	}
	return false
}

type Program struct {
	SSA             *ssa.Program
	InitialPackages []*Pkg
	AllPackages     []*packages.Package
	AllFunctions    []*ssa.Function
}

func (prog *Program) Fset() *token.FileSet {
	return prog.InitialPackages[0].Fset
}

type Func func(*Job)

type Severity uint8

const (
	Error Severity = iota
	Warning
	Ignored
)

// Problem represents a problem in some source code.
type Problem struct {
	Position token.Position // position in source file
	Text     string         // the prose that describes the problem
	Check    string
	Package  *Pkg
	Severity Severity
}

func (p *Problem) String() string {
	if p.Check == "" {
		return p.Text
	}
	return fmt.Sprintf("%s (%s)", p.Text, p.Check)
}

type Checker interface {
	Name() string
	Prefix() string
	Init(*Program)
	Checks() []Check
}

type Check struct {
	Fn              Func
	ID              string
	FilterGenerated bool
	Doc             string
}

// A Linter lints Go source code.
type Linter struct {
	Checkers      []Checker
	Ignores       []Ignore
	GoVersion     int
	ReturnIgnored bool
	Config        config.Config

	MaxConcurrentJobs int
	PrintStats        bool

	automaticIgnores []Ignore
}

func (l *Linter) ignore(p Problem) bool {
	ignored := false
	for _, ig := range l.automaticIgnores {
		// We cannot short-circuit these, as we want to record, for
		// each ignore, whether it matched or not.
		if ig.Match(p) {
			ignored = true
		}
	}
	if ignored {
		// no need to execute other ignores if we've already had a
		// match.
		return true
	}
	for _, ig := range l.Ignores {
		// We can short-circuit here, as we aren't tracking any
		// information.
		if ig.Match(p) {
			return true
		}
	}

	return false
}

func (j *Job) File(node Positioner) *ast.File {
	return j.Pkg.tokenFileMap[j.Pkg.Fset.File(node.Pos())]
}

func parseDirective(s string) (cmd string, args []string) {
	if !strings.HasPrefix(s, "//lint:") {
		return "", nil
	}
	s = strings.TrimPrefix(s, "//lint:")
	fields := strings.Split(s, " ")
	return fields[0], fields[1:]
}

type PerfStats struct {
	PackageLoading time.Duration
	SSABuild       time.Duration
	OtherInitWork  time.Duration
	CheckerInits   map[string]time.Duration
	Jobs           []JobStat
}

type JobStat struct {
	Job      string
	Duration time.Duration
}

func (stats *PerfStats) Print(w io.Writer) {
	fmt.Fprintln(w, "Package loading:", stats.PackageLoading)
	fmt.Fprintln(w, "SSA build:", stats.SSABuild)
	fmt.Fprintln(w, "Other init work:", stats.OtherInitWork)

	fmt.Fprintln(w, "Checker inits:")
	for checker, d := range stats.CheckerInits {
		fmt.Fprintf(w, "\t%s: %s\n", checker, d)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Jobs:")
	sort.Slice(stats.Jobs, func(i, j int) bool {
		return stats.Jobs[i].Duration < stats.Jobs[j].Duration
	})
	var total time.Duration
	for _, job := range stats.Jobs {
		fmt.Fprintf(w, "\t%s: %s\n", job.Job, job.Duration)
		total += job.Duration
	}
	fmt.Fprintf(w, "\tTotal: %s\n", total)
}

func (l *Linter) Lint(initial []*packages.Package, stats *PerfStats) []Problem {
	allPkgs := allPackages(initial)
	t := time.Now()
	ssaprog, _ := ssautil.Packages(allPkgs, ssa.GlobalDebug)
	ssaprog.Build()
	if stats != nil {
		stats.SSABuild = time.Since(t)
	}
	runtime.GC()

	t = time.Now()
	pkgMap := map[*ssa.Package]*Pkg{}
	var pkgs []*Pkg
	for _, pkg := range initial {
		ssapkg := ssaprog.Package(pkg.Types)
		var cfg config.Config
		if len(pkg.GoFiles) != 0 {
			path := pkg.GoFiles[0]
			dir := filepath.Dir(path)
			var err error
			// OPT(dh): we're rebuilding the entire config tree for
			// each package. for example, if we check a/b/c and
			// a/b/c/d, we'll process a, a/b, a/b/c, a, a/b, a/b/c,
			// a/b/c/d – we should cache configs per package and only
			// load the new levels.
			cfg, err = config.Load(dir)
			if err != nil {
				// FIXME(dh): we couldn't load the config, what are we
				// supposed to do? probably tell the user somehow
			}
			cfg = cfg.Merge(l.Config)
		}

		pkg := &Pkg{
			SSA:          ssapkg,
			Package:      pkg,
			Config:       cfg,
			Generated:    map[string]bool{},
			tokenFileMap: map[*token.File]*ast.File{},
		}
		pkg.Inspector = inspector.New(pkg.Syntax)
		for _, f := range pkg.Syntax {
			tf := pkg.Fset.File(f.Pos())
			pkg.tokenFileMap[tf] = f

			path := DisplayPosition(pkg.Fset, f.Pos()).Filename
			pkg.Generated[path] = isGenerated(path)
		}
		pkgMap[ssapkg] = pkg
		pkgs = append(pkgs, pkg)
	}

	prog := &Program{
		SSA:             ssaprog,
		InitialPackages: pkgs,
		AllPackages:     allPkgs,
	}

	for fn := range ssautil.AllFunctions(ssaprog) {
		prog.AllFunctions = append(prog.AllFunctions, fn)
		if fn.Pkg == nil {
			continue
		}
		if pkg, ok := pkgMap[fn.Pkg]; ok {
			pkg.InitialFunctions = append(pkg.InitialFunctions, fn)
		}
	}

	var out []Problem
	l.automaticIgnores = nil
	for _, pkg := range initial {
		for _, f := range pkg.Syntax {
			found := false
		commentLoop:
			for _, cg := range f.Comments {
				for _, c := range cg.List {
					if strings.Contains(c.Text, "//lint:") {
						found = true
						break commentLoop
					}
				}
			}
			if !found {
				continue
			}
			cm := ast.NewCommentMap(pkg.Fset, f, f.Comments)
			for node, cgs := range cm {
				for _, cg := range cgs {
					for _, c := range cg.List {
						if !strings.HasPrefix(c.Text, "//lint:") {
							continue
						}
						cmd, args := parseDirective(c.Text)
						switch cmd {
						case "ignore", "file-ignore":
							if len(args) < 2 {
								// FIXME(dh): this causes duplicated warnings when using megacheck
								p := Problem{
									Position: DisplayPosition(prog.Fset(), c.Pos()),
									Text:     "malformed linter directive; missing the required reason field?",
									Check:    "",
									Package:  nil,
								}
								out = append(out, p)
								continue
							}
						default:
							// unknown directive, ignore
							continue
						}
						checks := strings.Split(args[0], ",")
						pos := DisplayPosition(prog.Fset(), node.Pos())
						var ig Ignore
						switch cmd {
						case "ignore":
							ig = &LineIgnore{
								File:   pos.Filename,
								Line:   pos.Line,
								Checks: checks,
								pos:    c.Pos(),
							}
						case "file-ignore":
							ig = &FileIgnore{
								File:   pos.Filename,
								Checks: checks,
							}
						}
						l.automaticIgnores = append(l.automaticIgnores, ig)
					}
				}
			}
		}
	}

	if stats != nil {
		stats.OtherInitWork = time.Since(t)
	}

	for _, checker := range l.Checkers {
		t := time.Now()
		checker.Init(prog)
		if stats != nil {
			stats.CheckerInits[checker.Name()] = time.Since(t)
		}
	}

	var jobs []*Job
	var allChecks []string

	var wg sync.WaitGroup
	for _, checker := range l.Checkers {
		for _, check := range checker.Checks() {
			allChecks = append(allChecks, check.ID)
			if check.Fn == nil {
				continue
			}
			for _, pkg := range pkgs {
				j := &Job{
					Pkg:       pkg,
					check:     check,
					GoVersion: l.GoVersion,
				}
				jobs = append(jobs, j)
				wg.Add(1)
				go func(check Check, j *Job) {
					t := time.Now()
					check.Fn(j)
					j.duration = time.Since(t)
					wg.Done()
				}(check, j)
			}
		}
	}

	wg.Wait()

	for _, j := range jobs {
		if stats != nil {
			stats.Jobs = append(stats.Jobs, JobStat{j.check.ID, j.duration})
		}
		for _, p := range j.problems {
			if p.Package == nil {
				panic(fmt.Sprintf("internal error: problem at position %s has nil package", p.Position))
			}
			allowedChecks := FilterChecks(allChecks, p.Package.Config.Checks)

			if l.ignore(p) {
				p.Severity = Ignored
			}
			// TODO(dh): support globs in check white/blacklist
			// OPT(dh): this approach doesn't actually disable checks,
			// it just discards their results. For the moment, that's
			// fine. None of our checks are super expensive. In the
			// future, we may want to provide opt-in expensive
			// analysis, which shouldn't run at all. It may be easiest
			// to implement this in the individual checks.
			if (l.ReturnIgnored || p.Severity != Ignored) && allowedChecks[p.Check] {
				out = append(out, p)
			}
		}
	}

	for _, ig := range l.automaticIgnores {
		ig, ok := ig.(*LineIgnore)
		if !ok {
			continue
		}
		if ig.matched {
			continue
		}

		couldveMatched := false
		for _, pkg := range pkgs {
			for _, f := range pkg.tokenFileMap {
				if prog.Fset().Position(f.Pos()).Filename != ig.File {
					continue
				}
				allowedChecks := FilterChecks(allChecks, pkg.Config.Checks)
				for _, c := range ig.Checks {
					if !allowedChecks[c] {
						continue
					}
					couldveMatched = true
					break
				}
				break
			}
		}

		if !couldveMatched {
			// The ignored checks were disabled for the containing package.
			// Don't flag the ignore for not having matched.
			continue
		}
		p := Problem{
			Position: DisplayPosition(prog.Fset(), ig.pos),
			Text:     "this linter directive didn't match anything; should it be removed?",
			Check:    "",
			Package:  nil,
		}
		out = append(out, p)
	}

	sort.Slice(out, func(i int, j int) bool {
		pi, pj := out[i].Position, out[j].Position

		if pi.Filename != pj.Filename {
			return pi.Filename < pj.Filename
		}
		if pi.Line != pj.Line {
			return pi.Line < pj.Line
		}
		if pi.Column != pj.Column {
			return pi.Column < pj.Column
		}

		return out[i].Text < out[j].Text
	})

	if l.PrintStats && stats != nil {
		stats.Print(os.Stderr)
	}

	if len(out) < 2 {
		return out
	}

	uniq := make([]Problem, 0, len(out))
	uniq = append(uniq, out[0])
	prev := out[0]
	for _, p := range out[1:] {
		if prev.Position == p.Position && prev.Text == p.Text {
			continue
		}
		prev = p
		uniq = append(uniq, p)
	}

	return uniq
}

func FilterChecks(allChecks []string, checks []string) map[string]bool {
	// OPT(dh): this entire computation could be cached per package
	allowedChecks := map[string]bool{}

	for _, check := range checks {
		b := true
		if len(check) > 1 && check[0] == '-' {
			b = false
			check = check[1:]
		}
		if check == "*" || check == "all" {
			// Match all
			for _, c := range allChecks {
				allowedChecks[c] = b
			}
		} else if strings.HasSuffix(check, "*") {
			// Glob
			prefix := check[:len(check)-1]
			isCat := strings.IndexFunc(prefix, func(r rune) bool { return unicode.IsNumber(r) }) == -1

			for _, c := range allChecks {
				idx := strings.IndexFunc(c, func(r rune) bool { return unicode.IsNumber(r) })
				if isCat {
					// Glob is S*, which should match S1000 but not SA1000
					cat := c[:idx]
					if prefix == cat {
						allowedChecks[c] = b
					}
				} else {
					// Glob is S1*
					if strings.HasPrefix(c, prefix) {
						allowedChecks[c] = b
					}
				}
			}
		} else {
			// Literal check name
			allowedChecks[check] = b
		}
	}
	return allowedChecks
}

// Pkg represents a package being linted.
type Pkg struct {
	SSA              *ssa.Package
	InitialFunctions []*ssa.Function
	*packages.Package
	Config    config.Config
	Inspector *inspector.Inspector
	// TODO(dh): this map should probably map from *ast.File, not string
	Generated map[string]bool

	tokenFileMap map[*token.File]*ast.File
}

type Positioner interface {
	Pos() token.Pos
}

func DisplayPosition(fset *token.FileSet, p token.Pos) token.Position {
	// Only use the adjusted position if it points to another Go file.
	// This means we'll point to the original file for cgo files, but
	// we won't point to a YACC grammar file.

	pos := fset.PositionFor(p, false)
	adjPos := fset.PositionFor(p, true)

	if filepath.Ext(adjPos.Filename) == ".go" {
		return adjPos
	}
	return pos
}

func (j *Job) Errorf(n Positioner, format string, args ...interface{}) *Problem {
	pos := DisplayPosition(j.Pkg.Fset, n.Pos())
	if j.Pkg.Generated[pos.Filename] && j.check.FilterGenerated {
		return nil
	}
	problem := Problem{
		Position: pos,
		Text:     fmt.Sprintf(format, args...),
		Check:    j.check.ID,
		Package:  j.Pkg,
	}
	j.problems = append(j.problems, problem)
	return &j.problems[len(j.problems)-1]
}

func allPackages(pkgs []*packages.Package) []*packages.Package {
	var out []*packages.Package
	packages.Visit(
		pkgs,
		func(pkg *packages.Package) bool {
			out = append(out, pkg)
			return true
		},
		nil,
	)
	return out
}

var bufferPool = &sync.Pool{
	New: func() interface{} {
		buf := bytes.NewBuffer(nil)
		buf.Grow(64)
		return buf
	},
}

func FuncName(f *types.Func) string {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	if f.Type() != nil {
		sig := f.Type().(*types.Signature)
		if recv := sig.Recv(); recv != nil {
			buf.WriteByte('(')
			if _, ok := recv.Type().(*types.Interface); ok {
				// gcimporter creates abstract methods of
				// named interfaces using the interface type
				// (not the named type) as the receiver.
				// Don't print it in full.
				buf.WriteString("interface")
			} else {
				types.WriteType(buf, recv.Type(), nil)
			}
			buf.WriteByte(')')
			buf.WriteByte('.')
		} else if f.Pkg() != nil {
			writePackage(buf, f.Pkg())
		}
	}
	buf.WriteString(f.Name())
	s := buf.String()
	bufferPool.Put(buf)
	return s
}

func writePackage(buf *bytes.Buffer, pkg *types.Package) {
	if pkg == nil {
		return
	}
	var s string
	s = pkg.Path()
	if s != "" {
		buf.WriteString(s)
		buf.WriteByte('.')
	}
}
