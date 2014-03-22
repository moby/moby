package main

import (
	"fmt"
	"os"
	"text/scanner"
	"strings"
)

func main() {
	s := &scanner.Scanner{}
	s.Init(os.Stdin)
	s.Whitespace = 1<<'\t' | 1<<' '
	//s.Mode = ScanIdents | ScanFloats | ScanChars | ScanStrings | ScanRawStrings | ScanComments | SkipComments
	s.Mode = scanner.ScanStrings | scanner.ScanRawStrings | scanner.ScanIdents
	expr, err := parse(s, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%d commands:\n", len(expr))
	for i, cmd := range expr {
		fmt.Printf("%%%d: %s\n", i, cmd)
	}
}

type Command struct {
	Args []string
	Children []*Command
}

func (cmd *Command) subString(depth int) string {
	var prefix string
	for i:=0; i<depth; i++ {
		prefix += "  "
	}
	s := prefix + strings.Join(cmd.Args, ", ")
	if len(cmd.Children) > 0 {
		s += " {\n"
		for _, subcmd := range cmd.Children {
			s += subcmd.subString(depth + 1)
		}
		s += prefix + "}"
	}
	s += "\n"
	return s
}

func (cmd *Command) String() string {
	return cmd.subString(0)
}

func parseArgs(s *scanner.Scanner) ([]string, rune, error) {
	var args []string
	tok := s.Scan()
	for tok != scanner.EOF {
		text := s.TokenText()
		//fmt.Printf("--> [%s]\n", text)
		if text == "{" || text == "}" || text == "\n" || text == "\r" || text == ";" {
			return args, tok, nil
		}
		args = append(args, text)
		tok = s.Scan()
	}
	return args, tok, nil
}

func parse(s *scanner.Scanner, opener string) ([]*Command, error) {
	var expr []*Command
	for {
		args, tok, err := parseArgs(s)
		if err != nil {
			return nil, err
		}
		cmd := &Command{Args: args}
		if s.TokenText() == "{" {
			children, err := parse(s, "{")
			if err != nil {
				return nil, err
			}
			cmd.Children = children
		}
		if len(cmd.Args) > 0 || len(cmd.Children) > 0 {
			expr = append(expr, cmd)
		}
		if tok == scanner.EOF {
			break
		}
	}
	return expr, nil
}
