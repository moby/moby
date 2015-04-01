package stringutils

import "testing"

func TestRandomString(t *testing.T) {
	str := GenerateRandomString()
	if len(str) != 64 {
		t.Fatalf("Id returned is incorrect: %s", str)
	}
}

func TestRandomStringUniqueness(t *testing.T) {
	repeats := 25
	set := make(map[string]struct{}, repeats)
	for i := 0; i < repeats; i = i + 1 {
		str := GenerateRandomString()
		if len(str) != 64 {
			t.Fatalf("Id returned is incorrect: %s", str)
		}
		if _, ok := set[str]; ok {
			t.Fatalf("Random number is repeated")
		}
		set[str] = struct{}{}
	}
}

func testLengthHelper(generator func(int) string, t *testing.T) {
	expectedLength := 20
	s := generator(expectedLength)
	if len(s) != expectedLength {
		t.Fatalf("Length of %s was %d but expected length %d", s, len(s), expectedLength)
	}	
}

func testUniquenessHelper(generator func(int) string, t *testing.T) {
	repeats := 25
	set := make(map[string]struct{}, repeats)
	for i := 0; i < repeats; i = i + 1 {
		str := generator(64)
		if len(str) != 64 {
			t.Fatalf("Id returned is incorrect: %s", str)
		}
		if _, ok := set[str]; ok {
			t.Fatalf("Random number is repeated")
		}
		set[str] = struct{}{}
	}
}

func isASCII(s string) bool {
	for _, c := range s {
		if c > 127 {
			return false
		}
	}
	return true
}

func TestGenerateRandomAlphaOnlyStringLength(t *testing.T) {
	testLengthHelper(GenerateRandomAlphaOnlyString, t)
}

func TestGenerateRandomAlphaOnlyStringUniqueness(t *testing.T) {
	testUniquenessHelper(GenerateRandomAlphaOnlyString, t)
}

func TestGenerateRandomAsciiStringLength(t *testing.T) {
	testLengthHelper(GenerateRandomAsciiString, t) 
}

func TestGenerateRandomAsciiStringUniqueness(t *testing.T) {
	testUniquenessHelper(GenerateRandomAsciiString, t)
}

func TestGenerateRandomAsciiStringIsAscii(t *testing.T) {
	str := GenerateRandomAsciiString(64)
	if !isASCII(str) {
		t.Fatalf("%s contained non-ascii characters", str)
	}
}
