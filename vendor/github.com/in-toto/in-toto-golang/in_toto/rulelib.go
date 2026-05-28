package in_toto

import (
	"fmt"
	"strings"
)

// An error message issued in UnpackRule if it receives a malformed rule.
var errorMsg = "Wrong rule format, available formats are:\n" +
	"\tMATCH <pattern> [IN <source-path-prefix>] WITH (MATERIALS|PRODUCTS)" +
	" [IN <destination-path-prefix>] FROM <step>,\n" +
	"\tCREATE <pattern>,\n" +
	"\tDELETE <pattern>,\n" +
	"\tMODIFY <pattern>,\n" +
	"\tALLOW <pattern>,\n" +
	"\tDISALLOW <pattern>,\n" +
	"\tREQUIRE <filename>\n\n"

/*
UnpackRule parses the passed rule and extracts and returns the information
required for rule processing.  It can be used to verify if a rule has a valid
format.  Available rule formats are:

	MATCH <pattern> [IN <source-path-prefix>] WITH (MATERIALS|PRODUCTS)
		[IN <destination-path-prefix>] FROM <step>,
	CREATE <pattern>,
	DELETE <pattern>,
	MODIFY <pattern>,
	ALLOW <pattern>,
	DISALLOW <pattern>

Rule tokens are normalized to lower case before returning.  The returned map
has the following format:

	{
		"type": "match" | "create" | "delete" |"modify" | "allow" | "disallow"
		"pattern": "<file name pattern>",
		"srcPrefix": "<path or empty string>", // MATCH rule only
		"dstPrefix": "<path or empty string>", // MATCH rule only
		"dstType": "materials" | "products">, // MATCH rule only
		"dstName": "<step name>", // Match rule only
	}

If the rule does not match any of the available formats the first return value
is nil and the second return value is the error.
*/
func UnpackRule(rule []string) (map[string]string, error) {
	// Cache rule len
	ruleLen := len(rule)

	// Create all lower rule copy to case-insensitively parse out tokens whose
	// position we don't know yet. We keep the original rule to retain the
	// non-token elements' case.
	ruleLower := make([]string, ruleLen)
	for i, val := range rule {
		ruleLower[i] = strings.ToLower(val)
	}

	switch ruleLower[0] {
	case "create", "modify", "delete", "allow", "disallow", "require":
		if ruleLen != 2 {
			return nil,
				fmt.Errorf("%s Got:\n\t %s", errorMsg, rule)
		}

		return map[string]string{
			"type":    ruleLower[0],
			"pattern": rule[1],
		}, nil

	case "match":
		var srcPrefix string
		var dstType string
		var dstPrefix string
		var dstName string

		// MATCH <pattern> IN <source-path-prefix> WITH (MATERIALS|PRODUCTS) \
		// IN <destination-path-prefix> FROM <step>
		if ruleLen == 10 && ruleLower[2] == "in" &&
			ruleLower[4] == "with" && ruleLower[6] == "in" &&
			ruleLower[8] == "from" {
			srcPrefix = rule[3]
			dstType = ruleLower[5]
			dstPrefix = rule[7]
			dstName = rule[9]
			// MATCH <pattern> IN <source-path-prefix> WITH (MATERIALS|PRODUCTS) \
			// FROM <step>
		} else if ruleLen == 8 && ruleLower[2] == "in" &&
			ruleLower[4] == "with" && ruleLower[6] == "from" {
			srcPrefix = rule[3]
			dstType = ruleLower[5]
			dstPrefix = ""
			dstName = rule[7]

			// MATCH <pattern> WITH (MATERIALS|PRODUCTS) IN <destination-path-prefix>
			// FROM <step>
		} else if ruleLen == 8 && ruleLower[2] == "with" &&
			ruleLower[4] == "in" && ruleLower[6] == "from" {
			srcPrefix = ""
			dstType = ruleLower[3]
			dstPrefix = rule[5]
			dstName = rule[7]

			// MATCH <pattern> WITH (MATERIALS|PRODUCTS) FROM <step>
		} else if ruleLen == 6 && ruleLower[2] == "with" &&
			ruleLower[4] == "from" {
			srcPrefix = ""
			dstType = ruleLower[3]
			dstPrefix = ""
			dstName = rule[5]

		} else {
			return nil,
				fmt.Errorf("%s Got:\n\t %s", errorMsg, rule)

		}

		return map[string]string{
			"type":      ruleLower[0],
			"pattern":   rule[1],
			"srcPrefix": srcPrefix,
			"dstPrefix": dstPrefix,
			"dstType":   dstType,
			"dstName":   dstName,
		}, nil

	default:
		return nil,
			fmt.Errorf("%s Got:\n\t %s", errorMsg, rule)
	}
}
