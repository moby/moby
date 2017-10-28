package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testLengthHelper(generator func(int) string, t *testing.T) {
	expectedLength := 20
	s := generator(expectedLength)
	assert.Equal(t, expectedLength, len(s))
}

func testUniquenessHelper(generator func(int) string, t *testing.T) {
	repeats := 25
	set := make(map[string]struct{}, repeats)
	for i := 0; i < repeats; i = i + 1 {
		str := generator(64)
		assert.Equal(t, 64, len(str))
		_, ok := set[str]
		assert.False(t, ok, "Random number is repeated")
		set[str] = struct{}{}
	}
}

func TestGenerateRandomAlphaOnlyStringLength(t *testing.T) {
	testLengthHelper(GenerateRandomAlphaOnlyString, t)
}

func TestGenerateRandomAlphaOnlyStringUniqueness(t *testing.T) {
	testUniquenessHelper(GenerateRandomAlphaOnlyString, t)
}
