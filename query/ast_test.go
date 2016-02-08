package query

import (
	"testing"

	"fmt"

	"github.com/stretchr/testify/require"
)

type constAst bool

func (c constAst) Match(queryable Queryable) bool {
	return bool(c)
}

func TestOr(t *testing.T) {
	cases := []struct{ left, right, expected bool }{
		{false, false, false},
		{false, true, true},
		{true, false, true},
		{true, true, true},
	}

	for _, cas := range cases {
		require.Equal(t, cas.expected, (&exprOr{constAst(cas.left), constAst(cas.right)}).Match(nil))
	}
}

func TestAnd(t *testing.T) {
	cases := []struct{ left, right, expected bool }{
		{false, false, false},
		{false, true, false},
		{true, false, false},
		{true, true, true},
	}

	for _, cas := range cases {
		require.Equal(t, cas.expected, (&exprAnd{constAst(cas.left), constAst(cas.right)}).Match(nil))
	}
}

func TestNot(t *testing.T) {
	require.Equal(t, true, (&exprNot{constAst(false)}).Match(nil))
	require.Equal(t, false, (&exprNot{constAst(true)}).Match(nil))
}

func TestComp(t *testing.T) {
	cases := []struct {
		q        Queryable
		comp     *exprComp
		expected bool
	}{
		// bool
		{
			q:        &mockQueryable{t: t, field: "x", operator: IS, result: true},
			comp:     &exprComp{field: "x"},
			expected: true,
		},
		{
			q:        &mockQueryable{t: t, field: "x", operator: IS, result: false},
			comp:     &exprComp{field: "x"},
			expected: false,
		},

		//str, =
		{
			q:        &mockQueryable{t: t, field: "field", operator: EQ, value: "jawher/query", result: true},
			comp:     &exprComp{field: "field", operator: "=", value: "jawher/query"},
			expected: true,
		},
		{
			q:        &mockQueryable{t: t, field: "field", operator: EQ, value: "jaer/query", result: false},
			comp:     &exprComp{field: "field", operator: "=", value: "jaer/query"},
			expected: false,
		},

		//str, !=
		{
			q:        &mockQueryable{t: t, field: "field", operator: EQ, value: "jawher/query", result: true},
			comp:     &exprComp{field: "field", operator: "!=", value: "jawher/query"},
			expected: false,
		},
		{
			q:        &mockQueryable{t: t, field: "field", operator: EQ, value: "jaer/query", result: false},
			comp:     &exprComp{field: "field", operator: "!=", value: "jaer/query"},
			expected: true,
		},

		//str, ~
		{
			q:        &mockQueryable{t: t, field: "field", operator: LIKE, value: "quer", result: true},
			comp:     &exprComp{field: "field", operator: "~", value: "quer"},
			expected: true,
		},
		{
			q:        &mockQueryable{t: t, field: "field", operator: LIKE, value: "bateau", result: false},
			comp:     &exprComp{field: "field", operator: "~", value: "bateau"},
			expected: false,
		},

		//str, !~
		{
			q:        &mockQueryable{t: t, field: "field", operator: LIKE, value: "quer", result: true},
			comp:     &exprComp{field: "field", operator: "!~", value: "quer"},
			expected: false,
		},
		{
			q:        &mockQueryable{t: t, field: "field", operator: LIKE, value: "bateau", result: false},
			comp:     &exprComp{field: "field", operator: "!~", value: "bateau"},
			expected: true,
		},

		// >
		{
			q: &mockQueryable{
				t:        t,
				field:    "field",
				operator: GT,
				value:    "42",
				result:   true,
			},
			comp:     &exprComp{field: "field", operator: ">", value: "42"},
			expected: true,
		},
		{
			q: &mockQueryable{
				t:        t,
				field:    "field",
				operator: GT,
				value:    "42",
				result:   false,
			},
			comp:     &exprComp{field: "field", operator: ">", value: "42"},
			expected: false,
		},

		// >=
		{
			q: &mockQueryable{
				t:        t,
				field:    "field",
				operator: GT,
				value:    "42",
				result:   true,
			},
			comp:     &exprComp{field: "field", operator: ">=", value: "42"},
			expected: true,
		},
		{
			q: &mockQueryable{
				t:        t,
				field:    "field",
				operator: GT,
				value:    "42",
				result:   false,
				then:     &mockQueryable{operator: EQ, result: true},
			},
			comp:     &exprComp{field: "field", operator: ">=", value: "42"},
			expected: true,
		},

		// <
		{
			q: &mockQueryable{
				t:        t,
				field:    "field",
				operator: GT,
				value:    "42",
				result:   true,
			},
			comp:     &exprComp{field: "field", operator: "<", value: "42"},
			expected: false,
		},
		{
			q: &mockQueryable{
				t:        t,
				field:    "field",
				operator: GT,
				value:    "42",
				result:   false,
				then:     &mockQueryable{operator: EQ, result: true},
			},
			comp:     &exprComp{field: "field", operator: "<", value: "42"},
			expected: false,
		},

		// <=
		{
			q: &mockQueryable{
				t:        t,
				field:    "field",
				operator: GT,
				value:    "42",
				result:   true,
			},
			comp:     &exprComp{field: "field", operator: "<=", value: "42"},
			expected: false,
		},
		{
			q: &mockQueryable{
				t:        t,
				field:    "field",
				operator: GT,
				value:    "42",
				result:   false,
			},
			comp:     &exprComp{field: "field", operator: "<=", value: "42"},
			expected: true,
		},
	}

	for _, cas := range cases {
		require.Equal(t, cas.expected, cas.comp.Match(cas.q), "comparison %v failed with %v", cas.comp, cas.q)
	}
}

type mockQueryable struct {
	t        *testing.T
	field    string
	operator Operator
	value    string
	result   bool
	then     *mockQueryable
}

func (c *mockQueryable) Is(field string, operator Operator, value string) bool {
	if field != c.field {
		c.t.Fatalf("Unexpected field %s", field)
	}
	if operator != c.operator {
		c.t.Fatalf("Unexpected operator %v", operator)
	}
	if value != c.value {
		c.t.Fatalf("Unexpected value %s", value)
	}
	res := c.result

	if c.then != nil {
		c.operator, c.result = c.then.operator, c.then.result
	}
	return res
}

func (c *mockQueryable) String() string {
	return fmt.Sprintf("%s %v %s => %v", c.field, c.operator, c.value, c.result)
}
