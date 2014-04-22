package dockerscript

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/dockerscript/scanner"
	"io"
	"strings"
)

type Command struct {
	Args       []string
	Children   []*Command
	Background bool
}

type Scanner struct {
	scanner.Scanner
	commentLine bool
}

func Parse(src io.Reader) ([]*Command, error) {
	s := &Scanner{}
	s.Init(src)
	s.Whitespace = 1<<'\t' | 1<<' '
	s.Mode = scanner.ScanStrings | scanner.ScanRawStrings | scanner.ScanIdents
	expr, err := parse(s, "")
	if err != nil {
		return nil, fmt.Errorf("line %d:%d: %v\n", s.Pos().Line, s.Pos().Column, err)
	}
	return expr, nil
}

func (cmd *Command) subString(depth int) string {
	var prefix string
	for i := 0; i < depth; i++ {
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

func parseArgs(s *Scanner) ([]string, rune, error) {
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
		// Toggle line comment
		if strings.HasPrefix(text, "#") {
			s.commentLine = true
		} else if text == "\n" || text == "\r" {
			s.commentLine = false
			return args, tok, nil
		}
		if !s.commentLine {
			if text == "{" || text == "}" || text == "\n" || text == "\r" || text == ";" || text == "&" {
				return args, tok, nil
			}
			args = append(args, text)
		}
		tok = s.Scan()
	}
	return args, tok, nil
}

func parse(s *Scanner, opener string) (expr []*Command, err error) {
	/*
		defer func() {
			fmt.Printf("parse() returned %d commands:\n", len(expr))
			for _, c := range expr {
				fmt.Printf("\t----> %s\n", c)
			}
		}()
	*/
	for {
		args, tok, err := parseArgs(s)
		if err != nil {
			return nil, err
		}
		cmd := &Command{Args: args}
		afterArgs := s.TokenText()
		if afterArgs == "{" {
			children, err := parse(s, "{")
			if err != nil {
				return nil, err
			}
			cmd.Children = children
		} else if afterArgs == "}" && opener != "{" {
			return nil, fmt.Errorf("unexpected end of block '}'")
		} else if afterArgs == "&" {
			cmd.Background = true
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
