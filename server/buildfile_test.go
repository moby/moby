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
		failure       bool
	}

	macroTests := []macroTest{
		// Basic macro substitution
		{
			failure:     false,
			input:       "var: $VARIABLE",
			output:      "var: testing substitution of macro",
			description: "basic substitution",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			failure:     false,
			input:       "var: $VARIABLE",
			output:      "var: $VARIABLE",
			description: "basic substitution - non set macro",
			macros: map[string]string{
				"NOTVARIABLE": "testing substitution of macro",
			},
		},
		{
			failure:     false,
			input:       "var: $$VARIABLE",
			output:      "var: $VARIABLE",
			description: "basic substitution - $$",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			failure:     false,
			input:       "var: \"$VARIABLE\"",
			output:      "var: \"testing substitution of macro\"",
			description: "basic substitution - double quoted",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			failure:     false,
			input:       "var: \"$$VARIABLE\"",
			output:      "var: \"$VARIABLE\"",
			description: "basic substitution - double quoted $$",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			failure:     false,
			input:       "var: '$VARIABLE'",
			output:      "var: '$VARIABLE'",
			description: "basic substitution - single quoted",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			failure:     false,
			input:       "var: '$$VARIABLE'",
			output:      "var: '$$VARIABLE'",
			description: "basic substitution - single quoted $$",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},
		{
			failure:     false,
			input:       "var: \"$VARIABLE\" '$VARIABLE' \"$VARIABLE\"",
			output:      "var: \"testing substitution of macro\" '$VARIABLE' \"testing substitution of macro\"",
			description: "basic substitution - mixed",
			macros: map[string]string{
				"VARIABLE": "testing substitution of macro",
			},
		},

		// Empty / no macros
		// Compatability tests.
		{
			failure:     false,
			input:       "var: $VARIABLE",
			output:      "var: $VARIABLE",
			description: "no macros",
			macros:      map[string]string{},
		},
		{
			failure:     false,
			input:       "var: $VARIABLE",
			output:      "var: $VARIABLE",
			description: "no macros",
			macros:      nil,
		},
		{
			failure:     false,
			input:       "var: $$VARIABLE",
			output:      "var: $VARIABLE",
			description: "no macros - $$",
			macros:      map[string]string{},
		},
		{
			failure:     false,
			input:       "var: $$VARIABLE",
			output:      "var: $VARIABLE",
			description: "no macros - $$",
			macros:      nil,
		},
		{
			failure:     false,
			input:       "var: \"$VARIABLE\"",
			output:      "var: \"$VARIABLE\"",
			description: "no macros - double quoted",
			macros:      map[string]string{},
		},
		{
			failure:     false,
			input:       "var: \"$VARIABLE\"",
			output:      "var: \"$VARIABLE\"",
			description: "no macros - double quoted",
			macros:      nil,
		},
		{
			failure:     false,
			input:       "var: \"$$VARIABLE\"",
			output:      "var: \"$VARIABLE\"",
			description: "no macros - double quoted $$",
			macros:      map[string]string{},
		},
		{
			failure:     false,
			input:       "var: \"$$VARIABLE\"",
			output:      "var: \"$VARIABLE\"",
			description: "no macros - double quoted $$",
			macros:      nil,
		},
		{
			failure:     false,
			input:       "var: '$VARIABLE'",
			output:      "var: '$VARIABLE'",
			description: "no macros - single quoted",
			macros:      map[string]string{},
		},
		{
			failure:     false,
			input:       "var: '$VARIABLE'",
			output:      "var: '$VARIABLE'",
			description: "no macros - single quoted",
			macros:      nil,
		},
		{
			failure:     false,
			input:       "var: '$$VARIABLE'",
			output:      "var: '$$VARIABLE'",
			description: "no macros - single quoted $$",
			macros:      map[string]string{},
		},
		{
			failure:     false,
			input:       "var: '$$VARIABLE'",
			output:      "var: '$$VARIABLE'",
			description: "no macros - single quoted $$",
			macros:      nil,
		},
		{
			failure:     false,
			input:       "var: \"$VARIABLE\" '$VARIABLE' \"$VARIABLE\"",
			output:      "var: \"$VARIABLE\" '$VARIABLE' \"$VARIABLE\"",
			description: "no macro - mixed",
			macros:      map[string]string{},
		},
		{
			failure:     false,
			input:       "var: \"$VARIABLE\" '$VARIABLE' \"$VARIABLE\"",
			output:      "var: \"$VARIABLE\" '$VARIABLE' \"$VARIABLE\"",
			description: "basic substitution - mixed",
			macros:      nil,
		},

		// Recursive macro substitution
		{
			failure:     false,
			input:       "var: $VARIABLE",
			output:      "var: testing recursive substitution of macro",
			description: "recursive substitution",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			failure:     false,
			input:       "var: $VARIABLE",
			output:      "var: testing $RECUR of macro",
			description: "recursive substitution - non set macro",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
			},
		},
		{
			failure:     false,
			input:       "var: $$VARIABLE",
			output:      "var: $VARIABLE",
			description: "recursive substitution - $$",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			failure:     false,
			input:       "var: \"$VARIABLE\"",
			output:      "var: \"testing recursive substitution of macro\"",
			description: "recursive substitution - double quoted",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			failure:     false,
			input:       "var: \"$$VARIABLE\"",
			output:      "var: \"$VARIABLE\"",
			description: "recursive substitution - double quoted $$",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			failure:     false,
			input:       "var: '$VARIABLE'",
			output:      "var: '$VARIABLE'",
			description: "recursive substitution - single quoted",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			failure:     false,
			input:       "var: '$$VARIABLE'",
			output:      "var: '$$VARIABLE'",
			description: "recursive substitution - single quoted $$",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},
		{
			failure:     false,
			input:       "var: \"$VARIABLE\" '$VARIABLE' \"$VARIABLE\"",
			output:      "var: \"testing recursive substitution of macro\" '$VARIABLE' \"testing recursive substitution of macro\"",
			description: "recursive substitution - mixed",
			macros: map[string]string{
				"VARIABLE": "testing $RECUR of macro",
				"RECUR":    "recursive substitution",
			},
		},

		// Miscellaneous
		{
			failure:     false,
			input:       "many dollars: $$$$$$",
			output:      "many dollars: $$$",
			description: "misc - many dollars",
			macros: map[string]string{
				"VARIABLE": "not used at all",
			},
		},
		{
			failure:     false,
			input:       "many dollars: $$$$$$",
			output:      "many dollars: $$$",
			description: "misc - many dollars - no macros",
			macros:      map[string]string{},
		},
		{
			failure:     false,
			input:       "many dollars: $$$$$$",
			output:      "many dollars: $$$",
			description: "misc - many dollars - no macros",
			macros:      nil,
		},
		{
			failure:     false,
			input:       "one dollar: $",
			output:      "one dollar: $",
			description: "misc - one dollar",
			macros: map[string]string{
				"VARIABLE": "not used at all",
			},
		},
		{
			failure:     false,
			input:       "one dollar: $",
			output:      "one dollar: $",
			description: "misc - one dollar - no macros",
			macros:      map[string]string{},
		},
		{
			failure:     false,
			input:       "one dollar: $",
			output:      "one dollar: $",
			description: "misc - one dollar - no macros",
			macros:      nil,
		},
		{
			failure:     false,
			input:       "whoops: $DOESNOTEXIST",
			output:      "whoops: $DOESNOTEXIST",
			description: "misc - non-existant macros",
			macros: map[string]string{
				"VARIABLE": "not used at all",
			},
		},
		{
			failure:     false,
			input:       "whoops: \"$DOESNOTEXIST\"",
			output:      "whoops: \"$DOESNOTEXIST\"",
			description: "misc - non-existant macros - double quoted",
			macros: map[string]string{
				"VARIABLE": "not used at all",
			},
		},
		{
			failure:     false,
			input:       "whoops: $DOESNOTEXIST",
			output:      "whoops: $DOESNOTEXIST",
			description: "misc - non-existant macros - no macros",
			macros:      map[string]string{},
		},
		{
			failure:     false,
			input:       "whoops: $DOESNOTEXIST",
			output:      "whoops: $DOESNOTEXIST",
			description: "misc - non-existant macros - no macros",
			macros:      nil,
		},

		// Error-related tests
		{
			failure:     true,
			input:       "error: $VARIABLE",
			output:      "<should have caused an error>",
			description: "error - recursion error",
			macros: map[string]string{
				"VARIABLE": "error here: $VARIABLE",
			},
		},
		{
			failure:     true,
			input:       "error: $VARIABLE",
			output:      "<should have caused an error>",
			description: "error - recursion error",
			macros: map[string]string{
				"VARIABLE": "no error: $RECUR",
				"RECUR":    "no error: $CHILD",
				"CHILD":    "error here: $VARIABLE",
			},
		},
	}

	for _, macroTest := range macroTests {
		out, err := macroSubstitution(macroTest.input, macroTest.macros, nil)

		if !macroTest.failure && err != nil {
			fmt.Printf("[ERROR ]: macro - %s: unexpected error: %s\n", macroTest.description, err.Error())
			t.Fail()
			continue
		}

		if macroTest.failure && err == nil {
			fmt.Printf("[FAILED]: macro - %s: expected error, got none\n", macroTest.description)
			t.Fail()
			continue
		}

		// Check output -- only for non-error tests
		if !macroTest.failure && out != macroTest.output {
			fmt.Printf("[FAILED]: macro - %s\n", macroTest.description)
			fmt.Printf("          expected: %s\n", macroTest.output)
			fmt.Printf("          got     : %s\n", out)
			t.Fail()
			continue
		}

		fmt.Printf("[PASSED]: macro - %s\n", macroTest.description)
	}
}
