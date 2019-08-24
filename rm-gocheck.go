// +build ignore

package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	shouldCommit = flag.Bool("commit", false, "if set, each step will result in a commit")
	filter       = flag.String("filter", "", "only run on files matching filter")
	titlePrefix  = flag.String("prefix", "rm-gocheck: ", "commit title prefix")
	allFiles     []string
	fileToCmp    = map[string]string{}
	cmps         = map[string][]string{}
)

type action func(*step) string

type step struct {
	files []string
	pkgs  map[string]string

	title   string
	pattern string
	action  action
	comment string
}

func mustSh(format string, args ...interface{}) (output []string) {
	var err error
	output, err = sh(format, args...)
	if err != nil {
		panic(err)
	}
	return
}

func sh(format string, args ...interface{}) (output []string, err error) {
	cmdargs := fmt.Sprintf(format, args...)
	out, err := exec.Command("sh", "-c", cmdargs).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("cmd=%s\nout=%s\n", cmdargs, out)
	}
	l := strings.Split(string(out), "\n")
	// remove last element if empty
	if len(l[len(l)-1]) == 0 {
		l = l[:len(l)-1]
	}
	return l, nil
}

func listToArgs(l []string) string {
	s := fmt.Sprintf("%q", l)
	s = s[1 : len(s)-1]
	return s
}

func Replace(subst string) action {
	return func(s *step) string {
		return fmt.Sprintf("sed -E -i 's#%s#%s#g' \\\n-- %s", s.pattern, subst, listToArgs(s.files))
	}
}

func CmpReplace(subst string) action {
	return func(s *step) string {
		var allCmdArgs, filesNeedingCmpImport []string
		for _, file := range s.files {
			cmp, ok := fileToCmp[file]
			if !ok {
				cmp = "cmp"
				l := mustSh(`grep -m 1 -F '"gotest.tools/assert/cmp"' %s | awk '{print $1}'`, file)
				if len(l) > 0 {
					cmp = l[0]
				} else {
					filesNeedingCmpImport = append(filesNeedingCmpImport, file)
				}
				fileToCmp[file] = cmp
				cmps[cmp] = append(cmps[cmp], file)
			}
		}

		if len(filesNeedingCmpImport) > 0 {
			linesep := " \\\n"
			importCmd := fmt.Sprintf(`sed -E -i '0,/^import "github\.com/ s/^(import "github\.com.*)/\1\nimport "gotest.tools\/assert\/cmp")/'%s-- %s`, linesep, listToArgs(filesNeedingCmpImport))
			allCmdArgs = append(allCmdArgs, importCmd)
			importCmd = fmt.Sprintf(`sed -E -i '0,/^\t+"github\.com/ s/(^\t+"github\.com.*)/\1\n"gotest.tools\/assert\/cmp"/'%s-- %s`, linesep, listToArgs(filesNeedingCmpImport))
			allCmdArgs = append(allCmdArgs, importCmd)
		}

		for cmp, files := range cmps {
			cmdargs := fmt.Sprintf("sed -E -i 's#%s#%s#g' \\\n-- %s", s.pattern, strings.ReplaceAll(subst, "${cmp}", cmp), listToArgs(files))
			allCmdArgs = append(allCmdArgs, cmdargs)
		}
		return strings.Join(allCmdArgs, " \\\n&& \\\n")
	}
}

func redress(pattern string, files ...string) error {
	rgx, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no files provided")
	}
	fn := func(file string) error {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		tmpName := file + ".tmp"
		fixed, err := os.Create(tmpName)
		if err != nil {
			return err
		}
		defer fixed.Close()

		const (
			searching = iota
			found
			line_done
		)
		state := searching
		s := bufio.NewScanner(f)
		for s.Scan() {
			b := s.Bytes()
			if state != found {
				bb := bytes.TrimRight(b, " \t")
				if state == line_done && len(bb) == 0 {
					continue
				}
				state = searching
				if !rgx.Match(b) {
					fixed.Write(b)
					fixed.Write([]byte{'\n'})
				} else {
					fixed.Write(bb)
					fixed.Write([]byte{' '})
					state = found
				}
				continue
			}
			b = bytes.TrimRight(b, " \t")
			fixed.Write(b)
			if len(b) > 0 {
				switch b[len(b)-1] {
				case ',', '(':
					fixed.Write([]byte{' '})
					continue
				case ')':
					fixed.Write([]byte{'\n'})
					state = line_done
				}
			}
		}
		if err := s.Err(); err != nil {
			return err
		}

		fixed.Close()
		f.Close()
		src, err := ioutil.ReadFile(tmpName)
		if err != nil {
			return err
		}
		src, err = format.Source(src)
		if err != nil {
			return err
		}
		os.Remove(tmpName)
		return ioutil.WriteFile(file, src, 0644)
	}

	var wg sync.WaitGroup
	wg.Add(len(files))
	for _, file := range files {
		go func(file string) {
			defer wg.Done()
			if err := fn(file); err != nil {
				panic(fmt.Sprintf("redress %s: %v", file, err))
			}
		}(file)
	}
	wg.Wait()
	return nil
}

func Redress(s *step) string {
	return fmt.Sprintf("go run rm-gocheck.go redress '%s' \\\n %s", s.pattern, listToArgs(s.files))
}

func Format(s *step) string {
	pkgs := make([]string, 0, len(s.pkgs))
	for dir := range s.pkgs {
		pkgs = append(pkgs, "./"+dir)
	}
	files := listToArgs(pkgs)
	return fmt.Sprintf("goimports -w \\\n-- %s \\\n&& \\\n gofmt -w -s \\\n-- %s", files, files)
}

func CommentInterface(s *step) string {
	cmds := make([]string, 0, len(s.pkgs))
	for dir := range s.pkgs {
		cmd := fmt.Sprintf(`while :; do \
	out=$(go test -c ./%s 2>&1 | grep 'cannot use nil as type string in return argument') || break
	echo "$out" | while read line; do
		file=$(echo "$line" | cut -d: -f1)
		n=$(echo "$line" | cut -d: -f2)
		sed -E -i "${n}"'s#\b(return .*, )nil#\1""#g' "$file"
	done
done`, dir)
		cmds = append(cmds, cmd)
	}
	return strings.Join(cmds, " \\\n&& \\\n")
}

func Eg(template string, prehook action, helperTypes string) action {
	return func(s *step) string {
		cmds := make([]string, 0, 3+4*len(s.pkgs))

		if prehook != nil {
			cmds = append(cmds, prehook(s))
		}

		cmdstr := fmt.Sprintf(`go get -d golang.org/x/tools/cmd/eg && dir=$(go env GOPATH)/src/golang.org/x/tools && git -C "$dir" fetch https://github.com/tiborvass/tools handle-variadic && git -C "$dir" checkout 61a94b82347c29b3289e83190aa3dda74d47abbb && go install golang.org/x/tools/cmd/eg`)
		cmds = append(cmds, cmdstr)

		for dir, pkg := range s.pkgs {
			cmds = append(cmds, fmt.Sprintf(`/bin/echo -e 'package %s\n%s' > ./%s/eg_helper.go`, pkg, helperTypes, dir))
			cmds = append(cmds, fmt.Sprintf(`goimports -w ./%s`, dir))
			cmds = append(cmds, fmt.Sprintf(`eg -w -t %s -- ./%s`, template, dir))
			cmds = append(cmds, fmt.Sprintf(`rm -f ./%s/eg_helper.go`, dir))
		}
		cmds = append(cmds, fmt.Sprintf("go run rm-gocheck.go redress '%s' \\\n %s", `\bassert\.Assert\b.*(\(|,)\s*$`, listToArgs(s.files)))
		return strings.Join(cmds, " \\\n&& \\\n")
	}
}

func do(steps []step) {
	fileArgs := listToArgs(allFiles)
	for _, s := range steps {
		fmt.Print(s.title, "... ")
		s.files, _ = sh(`git grep --name-only -E '%s' -- %s`, s.pattern, fileArgs)
		if len(s.files) == 0 {
			fmt.Println("no files match")
			continue
		}
		s.pkgs = map[string]string{}
		pkg := ""
		if len(s.files) > 0 {
			x := mustSh(`grep -m1 '^package ' -- %s | cut -d' ' -f2`, s.files[0])
			pkg = x[0]
		}
		for _, file := range s.files {
			s.pkgs[filepath.Dir(file)] = pkg
		}
		cmdstr := s.action(&s)
		mustSh(cmdstr)
		if *shouldCommit {
			if len(s.comment) > 0 {
				s.comment = "\n\n" + s.comment
			}
			msg := fmt.Sprintf("%s%s\n\n%s%s", *titlePrefix, s.title, cmdstr, s.comment)
			sh(`git add %s`, listToArgs(s.files))
			cmd := exec.Command("git", "commit", "-s", "-F-")
			cmd.Stdin = strings.NewReader(msg)
			out, err := cmd.CombinedOutput()
			if err != nil {
				panic(string(out))
			}
			fmt.Println("committed")
		} else {
			fmt.Println("done")
		}
	}
}

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) > 0 {
		switch cmd := args[0]; cmd {
		case "redress":
			if len(args) < 3 {
				panic(fmt.Sprintf("usage: %s [flags] redress <pattern> <files...>", os.Args[0]))
			}
			if err := redress(args[1], args[2:]...); err != nil {
				panic(fmt.Sprintf("redress: %v", err))
			}
			return
		default:
			panic(fmt.Sprintf("unknown command %s", cmd))
		}
	}

	allFiles, _ = sh(`git grep --name-only '"github.com/go-check/check"' :**.go | grep -vE '^(vendor/|integration-cli/checker|rm-gocheck\.go|template\..*\.go)' | grep -E '%s'`, *filter)
	if len(allFiles) == 0 {
		return
	}

	do([]step{
		{
			title:   "normalize c.Check to c.Assert",
			pattern: `\bc\.Check\(`,
			action:  Replace(`c.Assert(`),
		},
		{
			title:   "redress multiline c.Assert calls",
			pattern: `\bc\.Assert\b.*(,|\()\s*$`,
			action:  Redress,
		},
		{
			title:   "c.Assert(...) -> assert.Assert(c, ...)",
			pattern: `\bc\.Assert\(`,
			action:  Replace(`assert.Assert(c, `),
		},
		{
			title:   "check.C -> testing.B for BenchmarkXXX",
			pattern: `( Benchmark[^\(]+\([^ ]+ \*)check\.C\b`,
			action:  Replace(`\1testing.B`),
		},
		{
			title:   "check.C -> testing.T",
			pattern: `\bcheck\.C\b`,
			action:  Replace(`testing.T`),
		},
		{
			title:   "ErrorMatches -> assert.ErrorContains",
			pattern: `\bassert\.Assert\(c, (.*), check\.ErrorMatches,`,
			action:  Replace(`assert.ErrorContains(c, \1,`),
		},
		{
			title:   "normalize to use checker",
			pattern: `\bcheck\.(Equals|DeepEquals|HasLen|IsNil|Matches|Not|NotNil)\b`,
			action:  Replace(`checker.\1`),
		},
		{
			title:   "Not(IsNil) -> != nil",
			pattern: `\bassert\.Assert\(c, (.*), checker\.Not\(checker\.IsNil\)`,
			action:  Replace(`assert.Assert(c, \1 != nil`),
		},
		{
			title:   "Not(Equals) -> a != b",
			pattern: `\bassert\.Assert\(c, (.*), checker\.Not\(checker\.Equals\), (.*)`,
			action:  Replace(`assert.Assert(c, \1 != \2`),
		},
		{
			title:   "Not(Matches) -> !cmp.Regexp",
			pattern: `\bassert\.Assert\(c, (.*), checker\.Not\(checker\.Matches\), (.*)\)`,
			action:  CmpReplace(`assert.Assert(c, !${cmp}.Regexp("^"+\2+"$", \1)().Success())`),
		},
		{
			title:   "Equals -> assert.Equal",
			pattern: `\bassert\.Assert\(c, (.*), checker\.Equals, (.*)`,
			action:  Replace(`assert.Equal(c, \1, \2`),
		},
		{
			title:   "DeepEquals -> assert.DeepEqual",
			pattern: `\bassert\.Assert\(c, (.*), checker\.DeepEquals, (.*)`,
			action:  Replace(`assert.DeepEqual(c, \1, \2`),
		},
		{
			title:   "HasLen -> assert.Equal + len()",
			pattern: `\bassert\.Assert\(c, (.*), checker\.HasLen, (.*)`,
			action:  Replace(`assert.Equal(c, len(\1), \2`),
		},
		{
			title:   "IsNil",
			pattern: `\bassert\.Assert\(c, (.*), checker\.IsNil\b`,
			action:  Replace(`assert.Assert(c, \1 == nil`),
		},
		{
			title:   "NotNil",
			pattern: `\bassert\.Assert\(c, (.*), checker\.NotNil\b`,
			action:  Replace(`assert.Assert(c, \1 != nil`),
		},
		{
			title:   "False",
			pattern: `\bassert\.Assert\(c, (.*), checker\.False\b`,
			action:  Replace(`assert.Assert(c, !\1`),
		},
		{
			title:   "True",
			pattern: `\bassert\.Assert\(c, (.*), checker\.True`,
			action:  Replace(`assert.Assert(c, \1`),
		},
		{
			title:   "redress check.Suite calls",
			pattern: `[^/]\bcheck\.Suite\(.*\{\s*$`,
			action:  Redress,
		},
		{
			title:   "comment out check.Suite calls",
			pattern: `^([^*])+?((var .*)?check\.Suite\(.*\))`,
			action:  Replace(`\1/*\2*/`),
		},
		{
			title:   "comment out check.TestingT",
			pattern: `([^*])(check\.TestingT\([^\)]+\))`,
			action:  Replace(`\1/*\2*/`),
		},
		{
			title:  "run goimports to compile successfully",
			action: Format,
		},
		{
			title:   "Matches -> cmp.Regexp",
			pattern: `\bassert\.Assert\(c, (.*), checker\.Matches, (.*)\)$`,
			action: Eg("template.matches.go",
				CmpReplace(`assert.Assert(c, eg_matches(${cmp}.Regexp, \1, \2))`),
				`var eg_matches func(func(cmp.RegexOrPattern, string) cmp.Comparison, interface{}, string, ...interface{}) bool`),
		},
		{
			title:   "Not(Contains) -> !strings.Contains",
			pattern: `\bassert\.Assert\(c, (.*), checker\.Not\(checker\.Contains\), (.*)\)$`,
			action: Eg("template.not_contains.go",
				Replace(`assert.Assert(c, !eg_contains(\1, \2))`),
				`var eg_contains func(arg1, arg2 string, extra ...interface{}) bool`),
		},
		{
			title:   "Contains -> strings.Contains",
			pattern: `\bassert\.Assert\(c, (.*), checker\.Contains, (.*)\)$`,
			action: Eg("template.contains.go",
				Replace(`assert.Assert(c, eg_contains(\1, \2))`),
				`var eg_contains func(arg1, arg2 string, extra ...interface{}) bool`),
		},
		{
			title:   "convert check.Commentf to string - with multiple args",
			pattern: `\bcheck.Commentf\(([^,]+),(.*)\)`,
			action:  Replace(`fmt.Sprintf(\1,\2)`),
		},
		{
			title:   "convert check.Commentf to string - with just one string",
			pattern: `\bcheck.Commentf\(("[^"]+")\)`,
			action:  Replace(`\1`),
		},
		{
			title:   "convert check.Commentf to string - other",
			pattern: `\bcheck.Commentf\(([^\)]+)\)`,
			action:  Replace(`\1`),
		},
		{
			title:   "check.CommentInterface -> string",
			pattern: `(\*testing\.T\b.*)check\.CommentInterface\b`,
			action:  Replace(`\1string`),
		},
		{
			title:  "goimports",
			action: Format,
		},
		{
			title:  "fix compile errors from converting check.CommentInterface to string",
			action: CommentInterface,
		},
	})
}
