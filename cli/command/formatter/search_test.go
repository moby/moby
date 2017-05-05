package formatter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestSearchContext(t *testing.T) {
	name := "nginx"
	starCount := 5000

	var ctx searchContext
	cases := []struct {
		searchCtx searchContext
		expValue  string
		call      func() string
	}{
		{searchContext{
			s: registrytypes.SearchResult{Name: name},
		}, name, ctx.Name},
		{searchContext{
			s: registrytypes.SearchResult{StarCount: starCount},
		}, "5000", ctx.StarCount},
		{searchContext{
			s: registrytypes.SearchResult{IsOfficial: true},
		}, "[OK]", ctx.IsOfficial},
		{searchContext{
			s: registrytypes.SearchResult{IsOfficial: false},
		}, "", ctx.IsOfficial},
		{searchContext{
			s: registrytypes.SearchResult{IsAutomated: true},
		}, "[OK]", ctx.IsAutomated},
		{searchContext{
			s: registrytypes.SearchResult{IsAutomated: false},
		}, "", ctx.IsAutomated},
	}

	for _, c := range cases {
		ctx = c.searchCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}
	}
}

func TestSearchContext_Description(t *testing.T) {
	shortDescription := "Official build of Nginx."
	longDescription := "Automated Nginx reverse proxy for docker containers"
	descriptionWReturns := "Automated\nNginx reverse\rproxy\rfor docker\ncontainers"

	var ctx searchContext
	cases := []struct {
		searchCtx searchContext
		expValue  string
		call      func() string
	}{
		{searchContext{
			s:     registrytypes.SearchResult{Description: shortDescription},
			trunc: true,
		}, shortDescription, ctx.Description},
		{searchContext{
			s:     registrytypes.SearchResult{Description: shortDescription},
			trunc: false,
		}, shortDescription, ctx.Description},
		{searchContext{
			s:     registrytypes.SearchResult{Description: longDescription},
			trunc: false,
		}, longDescription, ctx.Description},
		{searchContext{
			s:     registrytypes.SearchResult{Description: longDescription},
			trunc: true,
		}, stringutils.Ellipsis(longDescription, 45), ctx.Description},
		{searchContext{
			s:     registrytypes.SearchResult{Description: descriptionWReturns},
			trunc: false,
		}, longDescription, ctx.Description},
		{searchContext{
			s:     registrytypes.SearchResult{Description: descriptionWReturns},
			trunc: true,
		}, stringutils.Ellipsis(longDescription, 45), ctx.Description},
	}

	for _, c := range cases {
		ctx = c.searchCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}
	}
}

func TestSearchContextWrite(t *testing.T) {
	cases := []struct {
		context  Context
		expected string
	}{

		// Errors
		{
			Context{Format: "{{InvalidFunction}}"},
			`Template parsing error: template: :1: function "InvalidFunction" not defined
`,
		},
		{
			Context{Format: "{{nil}}"},
			`Template parsing error: template: :1:2: executing "" at <nil>: nil is not a command
`,
		},
		// Table format
		{
			Context{Format: NewSearchFormat("table")},
			`NAME                DESCRIPTION         STARS               OFFICIAL            AUTOMATED
result1             Official build      5000                [OK]                
result2             Not official        5                                       [OK]
`,
		},
		{
			Context{Format: NewSearchFormat("table {{.Name}}")},
			`NAME
result1
result2
`,
		},
		// Custom Format
		{
			Context{Format: NewSearchFormat("{{.Name}}")},
			`result1
result2
`,
		},
		// Custom Format with CreatedAt
		{
			Context{Format: NewSearchFormat("{{.Name}} {{.StarCount}}")},
			`result1 5000
result2 5
`,
		},
	}

	for _, testcase := range cases {
		results := []registrytypes.SearchResult{
			{Name: "result1", Description: "Official build", StarCount: 5000, IsOfficial: true, IsAutomated: false},
			{Name: "result2", Description: "Not official", StarCount: 5, IsOfficial: false, IsAutomated: true},
		}
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		err := SearchWrite(testcase.context, results, false, 0)
		if err != nil {
			assert.Error(t, err, testcase.expected)
		} else {
			assert.Equal(t, out.String(), testcase.expected)
		}
	}
}

func TestSearchContextWrite_Automated(t *testing.T) {
	cases := []struct {
		context  Context
		expected string
	}{

		// Table format
		{
			Context{Format: NewSearchFormat("table")},
			`NAME                DESCRIPTION         STARS               OFFICIAL            AUTOMATED
result2             Not official        5                                       [OK]
`,
		},
		{
			Context{Format: NewSearchFormat("table {{.Name}}")},
			`NAME
result2
`,
		},
	}

	for _, testcase := range cases {
		results := []registrytypes.SearchResult{
			{Name: "result1", Description: "Official build", StarCount: 5000, IsOfficial: true, IsAutomated: false},
			{Name: "result2", Description: "Not official", StarCount: 5, IsOfficial: false, IsAutomated: true},
		}
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		err := SearchWrite(testcase.context, results, true, 0)
		if err != nil {
			assert.Error(t, err, testcase.expected)
		} else {
			assert.Equal(t, out.String(), testcase.expected)
		}
	}
}

func TestSearchContextWrite_Stars(t *testing.T) {
	cases := []struct {
		context  Context
		expected string
	}{

		// Table format
		{
			Context{Format: NewSearchFormat("table")},
			`NAME                DESCRIPTION         STARS               OFFICIAL            AUTOMATED
result1             Official build      5000                [OK]                
`,
		},
		{
			Context{Format: NewSearchFormat("table {{.Name}}")},
			`NAME
result1
`,
		},
	}

	for _, testcase := range cases {
		results := []registrytypes.SearchResult{
			{Name: "result1", Description: "Official build", StarCount: 5000, IsOfficial: true, IsAutomated: false},
			{Name: "result2", Description: "Not official", StarCount: 5, IsOfficial: false, IsAutomated: true},
		}
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		err := SearchWrite(testcase.context, results, false, 6)
		if err != nil {
			assert.Error(t, err, testcase.expected)
		} else {
			assert.Equal(t, out.String(), testcase.expected)
		}
	}
}

func TestSearchContextWriteJSON(t *testing.T) {
	results := []registrytypes.SearchResult{
		{Name: "result1", Description: "Official build", StarCount: 5000, IsOfficial: true, IsAutomated: false},
		{Name: "result2", Description: "Not official", StarCount: 5, IsOfficial: false, IsAutomated: true},
	}
	expectedJSONs := []map[string]interface{}{
		{"Name": "result1", "Description": "Official build", "StarCount": "5000", "IsOfficial": "true", "IsAutomated": "false"},
		{"Name": "result2", "Description": "Not official", "StarCount": "5", "IsOfficial": "false", "IsAutomated": "true"},
	}

	out := bytes.NewBufferString("")
	err := SearchWrite(Context{Format: "{{json .}}", Output: out}, results, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		t.Logf("Output: line %d: %s", i, line)
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatal(err)
		}
		assert.DeepEqual(t, m, expectedJSONs[i])
	}
}

func TestSearchContextWriteJSONField(t *testing.T) {
	results := []registrytypes.SearchResult{
		{Name: "result1", Description: "Official build", StarCount: 5000, IsOfficial: true, IsAutomated: false},
		{Name: "result2", Description: "Not official", StarCount: 5, IsOfficial: false, IsAutomated: true},
	}
	out := bytes.NewBufferString("")
	err := SearchWrite(Context{Format: "{{json .Name}}", Output: out}, results, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		t.Logf("Output: line %d: %s", i, line)
		var s string
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, s, results[i].Name)
	}
}
