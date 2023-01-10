package ini

import (
	"fmt"
	"sort"
	"strings"
)

// Visitor is an interface used by walkers that will
// traverse an array of ASTs.
type Visitor interface {
	VisitExpr(AST) error
	VisitStatement(AST) error
}

// DefaultVisitor is used to visit statements and expressions
// and ensure that they are both of the correct format.
// In addition, upon visiting this will build sections and populate
// the Sections field which can be used to retrieve profile
// configuration.
type DefaultVisitor struct {

	// scope is the profile which is being visited
	scope string

	// path is the file path which the visitor is visiting
	path string

	// Sections defines list of the profile section
	Sections Sections
}

// NewDefaultVisitor returns a DefaultVisitor. It takes in a filepath
// which points to the file it is visiting.
func NewDefaultVisitor(filepath string) *DefaultVisitor {
	return &DefaultVisitor{
		Sections: Sections{
			container: map[string]Section{},
		},
		path: filepath,
	}
}

// VisitExpr visits expressions...
func (v *DefaultVisitor) VisitExpr(expr AST) error {
	t := v.Sections.container[v.scope]
	if t.values == nil {
		t.values = values{}
	}
	if t.SourceFile == nil {
		t.SourceFile = make(map[string]string, 0)
	}

	switch expr.Kind {
	case ASTKindExprStatement:
		opExpr := expr.GetRoot()
		switch opExpr.Kind {
		case ASTKindEqualExpr:
			children := opExpr.GetChildren()
			if len(children) <= 1 {
				return NewParseError("unexpected token type")
			}

			rhs := children[1]

			// The right-hand value side the equality expression is allowed to contain '[', ']', ':', '=' in the values.
			// If the token is not either a literal or one of the token types that identifies those four additional
			// tokens then error.
			if !(rhs.Root.Type() == TokenLit || rhs.Root.Type() == TokenOp || rhs.Root.Type() == TokenSep) {
				return NewParseError("unexpected token type")
			}

			key := EqualExprKey(opExpr)
			val, err := newValue(rhs.Root.ValueType, rhs.Root.base, rhs.Root.Raw())
			if err != nil {
				return err
			}

			// lower case key to standardize
			k := strings.ToLower(key)

			// identify if the section already had this key, append log on section
			if t.Has(k) {
				t.Logs = append(t.Logs,
					fmt.Sprintf("For profile: %v, overriding %v value, "+
						"with a %v value found in a duplicate profile defined later in the same file %v. \n",
						t.Name, k, k, v.path))
			}

			// assign the value
			t.values[k] = val
			// update the source file path for region
			t.SourceFile[k] = v.path
		default:
			return NewParseError(fmt.Sprintf("unsupported expression %v", expr))
		}
	default:
		return NewParseError(fmt.Sprintf("unsupported expression %v", expr))
	}

	v.Sections.container[v.scope] = t
	return nil
}

// VisitStatement visits statements...
func (v *DefaultVisitor) VisitStatement(stmt AST) error {
	switch stmt.Kind {
	case ASTKindCompletedSectionStatement:
		child := stmt.GetRoot()
		if child.Kind != ASTKindSectionStatement {
			return NewParseError(fmt.Sprintf("unsupported child statement: %T", child))
		}

		name := string(child.Root.Raw())

		// trim start and end space
		name = strings.TrimSpace(name)

		// if has prefix "profile " + [ws+] + "profile-name",
		// we standardize by removing the [ws+] between prefix and profile-name.
		if strings.HasPrefix(name, "profile ") {
			names := strings.SplitN(name, " ", 2)
			name = names[0] + " " + strings.TrimLeft(names[1], " ")
		}

		// attach profile name on section
		if !v.Sections.HasSection(name) {
			v.Sections.container[name] = NewSection(name)
		}
		v.scope = name
	default:
		return NewParseError(fmt.Sprintf("unsupported statement: %s", stmt.Kind))
	}

	return nil
}

// Sections is a map of Section structures that represent
// a configuration.
type Sections struct {
	container map[string]Section
}

// NewSections returns empty ini Sections
func NewSections() Sections {
	return Sections{
		container: make(map[string]Section, 0),
	}
}

// GetSection will return section p. If section p does not exist,
// false will be returned in the second parameter.
func (t Sections) GetSection(p string) (Section, bool) {
	v, ok := t.container[p]
	return v, ok
}

// HasSection denotes if Sections consist of a section with
// provided name.
func (t Sections) HasSection(p string) bool {
	_, ok := t.container[p]
	return ok
}

// SetSection sets a section value for provided section name.
func (t Sections) SetSection(p string, v Section) Sections {
	t.container[p] = v
	return t
}

// DeleteSection deletes a section entry/value for provided section name./
func (t Sections) DeleteSection(p string) {
	delete(t.container, p)
}

// values represents a map of union values.
type values map[string]Value

// List will return a list of all sections that were successfully
// parsed.
func (t Sections) List() []string {
	keys := make([]string, len(t.container))
	i := 0
	for k := range t.container {
		keys[i] = k
		i++
	}

	sort.Strings(keys)
	return keys
}

// Section contains a name and values. This represent
// a sectioned entry in a configuration file.
type Section struct {
	// Name is the Section profile name
	Name string

	// values are the values within parsed profile
	values values

	// Errors is the list of errors
	Errors []error

	// Logs is the list of logs
	Logs []string

	// SourceFile is the INI Source file from where this section
	// was retrieved. They key is the property, value is the
	// source file the property was retrieved from.
	SourceFile map[string]string
}

// NewSection returns an initialize section for the name
func NewSection(name string) Section {
	return Section{
		Name:       name,
		values:     values{},
		SourceFile: map[string]string{},
	}
}

// UpdateSourceFile updates source file for a property to provided filepath.
func (t Section) UpdateSourceFile(property string, filepath string) {
	t.SourceFile[property] = filepath
}

// UpdateValue updates value for a provided key with provided value
func (t Section) UpdateValue(k string, v Value) error {
	t.values[k] = v
	return nil
}

// Has will return whether or not an entry exists in a given section
func (t Section) Has(k string) bool {
	_, ok := t.values[k]
	return ok
}

// ValueType will returned what type the union is set to. If
// k was not found, the NoneType will be returned.
func (t Section) ValueType(k string) (ValueType, bool) {
	v, ok := t.values[k]
	return v.Type, ok
}

// Bool returns a bool value at k
func (t Section) Bool(k string) bool {
	return t.values[k].BoolValue()
}

// Int returns an integer value at k
func (t Section) Int(k string) int64 {
	return t.values[k].IntValue()
}

// Float64 returns a float value at k
func (t Section) Float64(k string) float64 {
	return t.values[k].FloatValue()
}

// String returns the string value at k
func (t Section) String(k string) string {
	_, ok := t.values[k]
	if !ok {
		return ""
	}
	return t.values[k].StringValue()
}
