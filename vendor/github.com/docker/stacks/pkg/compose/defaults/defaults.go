package defaults

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/docker/stacks/pkg/compose/template"
)

// InvalidTemplateError is returned when a variable template is not in a valid
// format
type InvalidTemplateError struct {
	Template string
}

func (e InvalidTemplateError) Error() string {
	return fmt.Sprintf("Invalid template: %#v", e.Template)
}

// RecordVariablesWithDefaults will record the variables and any default values
// through the mapping function.  The original variables will be left unchanged.
func RecordVariablesWithDefaults(tmpl string, mapping template.Mapping) (string, error) {
	var err error
	result := template.DefaultPattern.ReplaceAllStringFunc(tmpl, func(substring string) string {
		matches := template.DefaultPattern.FindStringSubmatch(substring)
		groups := matchGroups(matches, template.DefaultPattern)
		if escaped := groups["escaped"]; escaped != "" {
			return escaped
		}

		substitution := groups["named"]
		if substitution == "" {
			substitution = groups["braced"]
		}

		if substitution == "" {
			err = &InvalidTemplateError{Template: tmpl}
			return ""
		}

		matched := false
		// Check for default values
		var name, defaultValue, errString string
		for _, sep := range []string{":-", "-"} {
			name, defaultValue = partition(substitution, sep)
			if defaultValue != "" {
				name = name + "=" + defaultValue
				matched = true
				break
			}
		}
		// Check for mandatory fields
		if !matched {
			for _, sep := range []string{":?", "?"} {
				name, errString = partition(substitution, sep)
				if errString != "" {
					break
				}
			}
		}
		// Call the provided mapping function to record the discovered variable
		_, _ = mapping(name)

		// but leave the variable present
		return substring
	})

	return result, err
}

func matchGroups(matches []string, pattern *regexp.Regexp) map[string]string {
	groups := make(map[string]string)
	for i, name := range pattern.SubexpNames()[1:] {
		groups[name] = matches[i+1]
	}
	return groups
}

// Split the string at the first occurrence of sep, and return the part before the separator,
// and the part after the separator.
//
// If the separator is not found, return the string itself, followed by an empty string.
func partition(s, sep string) (string, string) {
	if strings.Contains(s, sep) {
		parts := strings.SplitN(s, sep, 2)
		return parts[0], parts[1]
	}
	return s, ""
}
