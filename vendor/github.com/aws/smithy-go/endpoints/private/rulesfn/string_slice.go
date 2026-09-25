package rulesfn

// StringSlice is a string slice with a negative-index-aware Get method for use
// in endpoint rule evaluation.
type StringSlice []string

// Get returns a pointer to the string at index i, or nil if the index is out
// of bounds. Negative indices count from the end of the slice.
func (s StringSlice) Get(i int) *string {
	if i < 0 {
		i = len(s) + i
	}
	if i < 0 || i >= len(s) {
		return nil
	}
	v := s[i]
	return &v
}
