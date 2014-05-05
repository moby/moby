package server

import (
	"fmt"
	"testing"
)

func TestMacroSubstitution(t *testing.T) {
	type macroTest struct {
		input, output string
		description   string
		macros        map[string]string
	}

	macroTests := []macroTest{
		// Basic variable substitution
		{
			input:       "var: $VARIABLE",
			output:      "var: testing substitution of macro",
			description: "basic substitution",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			input:       "var: $$VARIABLE",
			output:      "var: $VARIABLE",
			description: "basic substitution - $$",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			input:       "var: \"$VARIABLE\"",
			output:      "var: \"testing substitution of macro\"",
			description: "basic substitution - double quoted",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			input:       "var: \"$$VARIABLE\"",
			output:      "var: \"$VARIABLE\"",
			description: "basic substitution - double quoted $$",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			input:       "var: '$VARIABLE'",
			output:      "var: '$VARIABLE'",
			description: "basic substitution - single quoted",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			input:       "var: '$$VARIABLE'",
			output:      "var: '$$VARIABLE'",
			description: "basic substitution - single quoted $$",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			input:       "var: \"$VARIABLE\" '$VARIABLE' \"$VARIABLE\"",
			output:      "var: \"testing substitution of macro\" '$VARIABLE' \"testing substitution of macro\"",
			description: "basic substitution - mixed",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},

		// Recursive variable substitution
		{
			input:       "var: $VARIABLE",
			output:      "var: testing recursive substitution of macro",
			description: "recursive substitution",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			input:       "var: $$VARIABLE",
			output:      "var: $VARIABLE",
			description: "recursive substitution - $$",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			input:       "var: \"$VARIABLE\"",
			output:      "var: \"testing recursive substitution of macro\"",
			description: "recursive substitution - double quoted",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			input:       "var: \"$$VARIABLE\"",
			output:      "var: \"$VARIABLE\"",
			description: "recursive substitution - double quoted $$",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			input:       "var: '$VARIABLE'",
			output:      "var: '$VARIABLE'",
			description: "recursive substitution - single quoted",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			input:       "var: '$$VARIABLE'",
			output:      "var: '$$VARIABLE'",
			description: "recursive substitution - single quoted $$",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			input:       "var: \"$VARIABLE\" '$VARIABLE' \"$VARIABLE\"",
			output:      "var: \"testing recursive substitution of macro\" '$VARIABLE' \"testing recursive substitution of macro\"",
			description: "recursive substitution - mixed",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
	}

	for _, macroTest := range macroTests {
		out, err := macroSubstitution(macroTest.input, macroTest.macros)

		if err != nil {
			fmt.Printf("[ERROR ]: macro - %s: %s\n", macroTest.description, err.Error())
			t.Fail()
			continue
		}

		if out != macroTest.output {
			fmt.Printf("[FAILED]: macro - %s\n", macroTest.description)
			t.Fail()
			continue
		}

		fmt.Printf("[PASSED]: macro - %s\n", macroTest.description)
	}
}
