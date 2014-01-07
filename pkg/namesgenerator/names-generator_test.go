package namesgenerator

import (
	"testing"
)

type FalseChecker struct{}

func (n *FalseChecker) Exists(name string) bool {
	return false
}

type TrueChecker struct{}

func (n *TrueChecker) Exists(name string) bool {
	return true
}

func TestGenerateRandomName(t *testing.T) {
	if _, err := GenerateRandomName(&FalseChecker{}); err != nil {
		t.Error(err)
	}

	if _, err := GenerateRandomName(&TrueChecker{}); err == nil {
		t.Error("An error was expected")
	}

}

// Make sure the generated names are awesome
func TestGenerateAwesomeNames(t *testing.T) {
	name, err := GenerateRandomName(&FalseChecker{})
	if err != nil {
		t.Error(err)
	}
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
