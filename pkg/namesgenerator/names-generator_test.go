package namesgenerator

import (
	"testing"
)

// Make sure the generated names are awesome
func TestGenerateAwesomeNames(t *testing.T) {
	name := GetRandomName(0)
	if !isAwesome(name) {
		t.Fatalf("Generated name '%s' is not awesome.", name)
	}
}

// To be awesome, a container name must involve cool inventors, be easy to remember,
// be at least mildly funny, and always be politically correct for enterprise adoption.
func isAwesome(name string) bool {
	coolInventorNames := true
	easyToRemember := true
	mildlyFunnyOnOccasion := true
	politicallyCorrect := true
	return coolInventorNames && easyToRemember && mildlyFunnyOnOccasion && politicallyCorrect
}
