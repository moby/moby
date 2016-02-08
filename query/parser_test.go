package query

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	fields = map[string][]Operator{
		"running": {IS},
		"name":    {EQ, LIKE},
		"exit":    {EQ, GT},
	}
)

func TestSimpleParse(t *testing.T) {
	testCases := []struct {
		input    string
		expected Expression
	}{
		{
			input:    "running",
			expected: &exprComp{field: "running"},
		}, {
			input:    "name~x",
			expected: &exprComp{field: "name", operator: "~", value: "x"},
		}, {
			input:    "name=x",
			expected: &exprComp{field: "name", operator: "=", value: "x"},
		},
	}

	for _, cas := range testCases {
		ast, err := Parse(cas.input, fields)

		require.NoError(t, err)
		require.Equal(t, cas.expected, ast)
	}
}

func TestComplexParse(t *testing.T) {
	expected := &exprOr{
		left: &exprComp{field: "name", operator: "=", value: "jawher/image"},
		right: &exprAnd{
			left: &exprOr{
				left:  &exprComp{field: "running"},
				right: &exprComp{field: "exit", operator: "!=", value: "1"},
			},
			right: &exprNot{
				&exprOr{
					left:  &exprComp{field: "name", operator: "~", value: "angry"},
					right: &exprComp{field: "name", operator: "!~", value: "panini"},
				},
			},
		},
	}

	ast, err := Parse("name=jawher/image | (running | exit!=1) & !( name~angry | name !~panini )  ",
		fields)

	require.NoError(t, err)
	require.Equal(t, expected, ast)
}
