package query

type Operator string

const (
	IS   Operator = ""
	EQ            = "="
	LIKE          = "~"
	GT            = ">"
)

/*
Queryable is the abstraction used by this library to work on arbitrary data structures.
Implementing this interface's methods for a user type makes it compatible with the Expression
predicates
*/
type Queryable interface {
	// Returns true if the provided field is set
	Is(field string, operator Operator, value string) bool
}
