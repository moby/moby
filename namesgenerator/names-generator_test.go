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
