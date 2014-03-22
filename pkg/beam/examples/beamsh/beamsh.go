package main

import (
	"fmt"
	"os"
	scanner "github.com/dotcloud/docker/pkg/dockerscript"
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
		fmt.Fprintf(os.Stderr, "line %d:%d: %v\n", s.Pos().Line, s.Pos().Column, err)
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
	var parseError error
	// FIXME: we overwrite previously set error
	s.Error = func(s *scanner.Scanner, msg string) {
		parseError = fmt.Errorf(msg)
		// parseError = fmt.Errorf("line %d:%d: %s\n", s.Pos().Line, s.Pos().Column, msg)
	}
	var args []string
	tok := s.Scan()
	for tok != scanner.EOF {
		if parseError != nil {
			return args, tok, parseError
		}
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

func parse(s *scanner.Scanner, opener string) (expr []*Command, err error) {
	defer func() {
		fmt.Printf("parse() returned %d commands:\n", len(expr))
		for _, c := range expr {
			fmt.Printf("\t----> %s\n", c)
		}
	}()
	for {
		args, tok, err := parseArgs(s)
		if err != nil {
			return nil, err
		}
		cmd := &Command{Args: args}
		fmt.Printf("---> args=%v finished by %s\n", args, s.TokenText())
		afterArgs := s.TokenText()
		if afterArgs == "{" {
			fmt.Printf("---> therefore calling parse() of sub-expression\n")
			children, err := parse(s, "{")
			fmt.Printf("---> parse() of sub-epxression returned %d commands (ended by %s) and error=%v\n", children, s.TokenText(), err)
			if err != nil {
				return nil, err
			}
			cmd.Children = children
		} else if afterArgs == "}" && opener != "{" {
			return nil, fmt.Errorf("unexpected end of block '}'")
		}
		if len(cmd.Args) > 0 || len(cmd.Children) > 0 {
			expr = append(expr, cmd)
		}
		if tok == scanner.EOF || afterArgs == "}" {
			break
		}
	}
	return expr, nil
}
