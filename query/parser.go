package query

import (
	"fmt"
	"strings"
)

type parser struct {
	lexer   *lexer
	matched token
	next    token

	fields map[string][]Operator
}

// ParseError is returned if a query cannot be successfuly parsed
type ParseError struct {
	// The original query
	Input string
	// The position where the parsing fails
	Pos int
	// The error message
	Message string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("Parse error: %s\n%s\n%s^", e.Message, e.Input, strings.Repeat(" ", e.Pos))
}

/*
Parse accepts an input string and the list and types of valid fields and returns either a matcher expression if the query
is valid, or else an error
*/
func Parse(input string, fields map[string][]Operator) (Expression, error) {
	lexer := newLexer(input)
	return (&parser{
		lexer:  lexer,
		next:   lexer.next(),
		fields: fields,
	}).parse()
}

func (p *parser) parse() (ast Expression, err error) {
	defer func() {
		if r := recover(); r != nil {
			ast = nil
			err = ParseError{
				Input:   p.lexer.input,
				Pos:     p.matched.pos,
				Message: fmt.Sprintf("%v", r),
			}
		}
	}()
	ast = p.or()
	if !p.found(tkEOF) {
		p.advance()
		panic("Unexpected input")
	}
	return
}

func (p *parser) or() Expression {
	left := p.and()
	for p.found(tkOr) {
		right := p.and()
		left = &exprOr{left, right}
	}
	return left
}

func (p *parser) and() Expression {
	left := p.atom()
	for p.found(tkAnd) {
		right := p.atom()
		left = &exprAnd{left, right}
	}
	return left
}

func (p *parser) atom() Expression {
	switch {
	case p.found(tkNot):
		return &exprNot{p.atom()}
	case p.found(tkLparen):
		res := p.or()
		if !p.found(tkRparen) {
			p.advance()
			panic("was expecting a closing parenthesis")
		}
		return res
	case p.found(tkLiteral):
		field := p.matched.value
		operators, found := p.fieldOperators(field)
		if !found {
			panic(fmt.Sprintf("Unknown field %s", field))
		}
		if !p.found(tkCompOp) {
			if !hasOperator(operators, IS) {
				panic(fmt.Sprintf("field %s cannot be used without an operator", field))
			}
			return &exprComp{field: field}
		}

		operator := p.matched.value
		if !hasOperator(operators, operatorMapping[operator]) {
			panic(fmt.Sprintf("field %s doesn not support operator %s", field, operator))
		}
		if !p.found(tkLiteral) {
			p.advance()
			panic("was expecting a comparison value")
		}
		value := p.matched.value
		return &exprComp{field: field, operator: operator, value: value}
	case p.found(tkEOF):
		panic("Unexpected end of query")
	default:
		panic("unexpected input")
	}
}

func (p *parser) fieldOperators(field string) ([]Operator, bool) {
	operators, found := p.fields[field]
	if found {
		return operators, true
	}
	for k, v := range p.fields {
		if strings.HasSuffix(k, ".*") && strings.HasPrefix(field, strings.TrimSuffix(k, ".*")) {
			return v, true
		}
	}

	return nil, false
}

var operatorMapping = map[string]Operator{
	"=":  EQ,
	"!=": EQ,
	"~":  LIKE,
	"!~": LIKE,
	">":  GT,
	">=": GT,
	"<":  GT,
	"<=": GT,
}

func hasOperator(operators []Operator, op Operator) bool {
	for _, o := range operators {
		if o == op {
			return true
		}
	}
	return false
}

func (p *parser) expect(class tokenClass) {
	if !p.found(class) {
		panic(fmt.Sprintf("was expecting %v", class))
	}
}

func (p *parser) found(class tokenClass) bool {
	if p.next.class == class {
		p.matched = p.next
		p.next = p.lexer.next()
		return true
	}
	return false
}

func (p *parser) advance() {
	p.matched = p.next
	p.next = p.lexer.next()
}
