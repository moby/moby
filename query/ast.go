package query

import "fmt"

/*
Expression is a predicate which can be applied to a queryable
*/
type Expression interface {
	// Match accepts or rejects a queryable depending on the expression implementation
	Match(queryable Queryable) bool
}

type exprOr struct {
	left, right Expression
}

func (or *exprOr) String() string {
	return fmt.Sprintf("(%v | %v)", or.left, or.right)
}

func (or *exprOr) Match(queryable Queryable) bool {
	if or.left.Match(queryable) {
		return true
	}
	return or.right.Match(queryable)
}

type exprAnd struct {
	left, right Expression
}

func (and *exprAnd) String() string {
	return fmt.Sprintf("(%v & %v)", and.left, and.right)
}

func (and *exprAnd) Match(queryable Queryable) bool {
	if !and.left.Match(queryable) {
		return false
	}
	return and.right.Match(queryable)
}

type exprNot struct {
	expression Expression
}

func (not *exprNot) String() string {
	return fmt.Sprintf("!(%v)", not.expression)
}

func (not *exprNot) Match(queryable Queryable) bool {
	return !not.expression.Match(queryable)
}

type exprComp struct {
	field    string
	operator string
	value    string
}

func (c *exprComp) String() string {
	return fmt.Sprintf("%s%s'%v'", c.field, c.operator, c.value)
}

func (c *exprComp) Match(queryable Queryable) bool {
	if len(c.operator) == 0 {
		return queryable.Is(c.field, IS, "")
	}
	switch c.operator {
	case "=":
		return queryable.Is(c.field, EQ, c.value)
	case "!=":
		return !queryable.Is(c.field, EQ, c.value)
	case "~":
		return queryable.Is(c.field, LIKE, c.value)
	case "!~":
		return !queryable.Is(c.field, LIKE, c.value)
	case ">":
		return queryable.Is(c.field, GT, c.value)
	case ">=":
		return queryable.Is(c.field, GT, c.value) || queryable.Is(c.field, EQ, c.value)
	case "<":
		return !queryable.Is(c.field, GT, c.value) && !queryable.Is(c.field, EQ, c.value)
	case "<=":
		return !queryable.Is(c.field, GT, c.value)
	default:
		panic(fmt.Sprintf("Operator %s is not implemented", c.operator))
	}
}
