package query

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLexerSingleTokens(t *testing.T) {
	cases := []struct {
		input    string
		expected token
	}{
		{"a", token{class: tkLiteral, value: "a", pos: 0}},
		{"a.b", token{class: tkLiteral, value: "a.b", pos: 0}},
		{"a/b", token{class: tkLiteral, value: "a/b", pos: 0}},
		{`"a b"`, token{class: tkLiteral, value: "a b", pos: 1}},
		{`"a\"b"`, token{class: tkLiteral, value: `a"b`, pos: 1}},

		{"=", token{class: tkCompOp, value: "=", pos: 0}},
		{"~", token{class: tkCompOp, value: "~", pos: 0}},
		{"!=", token{class: tkCompOp, value: "!=", pos: 0}},
		{"!~", token{class: tkCompOp, value: "!~", pos: 0}},
		{">", token{class: tkCompOp, value: ">", pos: 0}},
		{">=", token{class: tkCompOp, value: ">=", pos: 0}},
		{"<", token{class: tkCompOp, value: "<", pos: 0}},
		{"<=", token{class: tkCompOp, value: "<=", pos: 0}},

		{"(", token{class: tkLparen, value: "(", pos: 0}},
		{")", token{class: tkRparen, value: ")", pos: 0}},

		{"!", token{class: tkNot, value: "!", pos: 0}},
		{"|", token{class: tkOr, value: "|", pos: 0}},
		{"&", token{class: tkAnd, value: "&", pos: 0}},

		{"", token{class: tkEOF, value: "", pos: 0}},
	}

	for _, cas := range cases {
		lx := newLexer(cas.input)
		require.Equal(t, cas.expected, lx.next())
	}
}

func TestLexerMultiTokens(t *testing.T) {
	cases := []struct {
		input    string
		expected []token
	}{
		{"lit1 lit2", []token{
			{class: tkLiteral, value: "lit1", pos: 0},
			{class: tkLiteral, value: "lit2", pos: 5},
			{class: tkEOF, value: "", pos: 9},
		}},
		{"lit1/lit2=", []token{
			{class: tkLiteral, value: "lit1/lit2", pos: 0},
			{class: tkCompOp, value: "=", pos: 9},
			{class: tkEOF, value: "", pos: 10},
		}},
		{"!lit1/lit2", []token{
			{class: tkNot, value: "!", pos: 0},
			{class: tkLiteral, value: "lit1/lit2", pos: 1},
			{class: tkEOF, value: "", pos: 10},
		}},
		{`!"!"!=!`, []token{
			{class: tkNot, value: "!", pos: 0},
			{class: tkLiteral, value: "!", pos: 2},
			{class: tkCompOp, value: "!=", pos: 4},
			{class: tkNot, value: "!", pos: 6},
			{class: tkEOF, value: "", pos: 7},
		}},
		{"=>=> =", []token{
			{class: tkCompOp, value: "=", pos: 0},
			{class: tkCompOp, value: ">=", pos: 1},
			{class: tkCompOp, value: ">", pos: 3},
			{class: tkCompOp, value: "=", pos: 5},
			{class: tkEOF, value: "", pos: 6},
		}},
	}

	for _, cas := range cases {
		lx := newLexer(cas.input)
		for _, expected := range cas.expected {
			require.Equal(t, expected, lx.next())
		}

		next := lx.next()
		require.True(t, next.class == tkEOF, "Lexer returned more tokens than expected: %v", next)
	}
}
func TestLexer(t *testing.T) {
	lx := newLexer(`image=jawher/image:3.0.2 & (exit=0 | exit!=1) & !( name~angry | name !~panini | label="arch=arm\"11\"")  `)
	expected := []token{
		{class: tkLiteral, value: "image", pos: 0},
		{class: tkCompOp, value: "=", pos: 5},
		{class: tkLiteral, value: "jawher/image:3.0.2", pos: 6},
		{class: tkAnd, value: "&", pos: 25},
		{class: tkLparen, value: "(", pos: 27},
		{class: tkLiteral, value: "exit", pos: 28},
		{class: tkCompOp, value: "=", pos: 32},
		{class: tkLiteral, value: "0", pos: 33},
		{class: tkOr, value: "|", pos: 35},
		{class: tkLiteral, value: "exit", pos: 37},
		{class: tkCompOp, value: "!=", pos: 41},
		{class: tkLiteral, value: "1", pos: 43},
		{class: tkRparen, value: ")", pos: 44},
		{class: tkAnd, value: "&", pos: 46},
		{class: tkNot, value: "!", pos: 48},
		{class: tkLparen, value: "(", pos: 49},
		{class: tkLiteral, value: "name", pos: 51},
		{class: tkCompOp, value: "~", pos: 55},
		{class: tkLiteral, value: "angry", pos: 56},
		{class: tkOr, value: "|", pos: 62},
		{class: tkLiteral, value: "name", pos: 64},
		{class: tkCompOp, value: "!~", pos: 69},
		{class: tkLiteral, value: "panini", pos: 71},
		{class: tkOr, value: "|", pos: 78},
		{class: tkLiteral, value: "label", pos: 80},
		{class: tkCompOp, value: "=", pos: 85},
		{class: tkLiteral, value: `arch=arm"11"`, pos: 87},
		{class: tkRparen, value: ")", pos: 102},
		{class: tkEOF, value: "", pos: 105},
	}

	for _, exTk := range expected {
		tk := lx.next()
		require.Equal(t, exTk, tk)
	}

	require.Equal(t, token{class: tkEOF, value: "", pos: 105}, lx.next(), "was expecting EOF")
}
