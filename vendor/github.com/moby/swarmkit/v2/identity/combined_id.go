package identity

import "fmt"

// CombineTwoIDs combines the given IDs into a new ID, e.g. a secret and a task ID.
func CombineTwoIDs(id1, id2 string) string {
	return fmt.Sprintf("%s.%s", id1, id2)
}
