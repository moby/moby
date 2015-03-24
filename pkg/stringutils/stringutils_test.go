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
