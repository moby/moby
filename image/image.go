package image

import (
	"fmt"
	"regexp"
)

var validHex = regexp.MustCompile(`^([a-f0-9]{64})$`)

// Check wheather id is a valid image ID or not
func ValidateID(id string) error {
	if ok := validHex.MatchString(id); !ok {
		return fmt.Errorf("image ID '%s' is invalid", id)
	}
	return nil
}
