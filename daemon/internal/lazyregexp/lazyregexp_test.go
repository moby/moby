package lazyregexp

import (
	"testing"
)

func TestCompileOnce(t *testing.T) {
	t.Run("invalid regexp", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected a panic")
			}
		}()
		_ = New("[")
	})
	t.Run("valid regexp", func(t *testing.T) {
		re := New("[a-z]")
		ok := re.MatchString("hello")
		if !ok {
			t.Errorf("expected a match")
		}
	})
}
