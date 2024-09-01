package versions

import (
	"context"
	"testing"
)

func TestContext(t *testing.T) {
	t.Run("empty version", func(t *testing.T) {
		const expectedVersion = ""
		ctx := WithVersion(context.TODO(), expectedVersion)
		if v := FromContext(ctx); v != expectedVersion {
			t.Errorf("expected: %q, got: %q", expectedVersion, v)
		}
	})
	t.Run("non-empty version", func(t *testing.T) {
		const expectedVersion = "v9.99"
		ctx := WithVersion(context.TODO(), expectedVersion)
		if v := FromContext(ctx); v != expectedVersion {
			t.Errorf("expected: %q, got: %q", expectedVersion, v)
		}
	})
	t.Run("empty version should not modify parent", func(t *testing.T) {
		const expectedVersion = "v9.99"
		ctx := WithVersion(context.TODO(), expectedVersion)
		if v := FromContext(ctx); v != expectedVersion {
			t.Errorf("expected: %q, got: %q", expectedVersion, v)
		}

		ctx2 := WithVersion(ctx, "")
		if v := FromContext(ctx2); v != expectedVersion {
			t.Errorf("expected: %q, got: %q", expectedVersion, v)
		}
	})
	t.Run("update version", func(t *testing.T) {
		const (
			expectedVersion  = "v9.99"
			expectedVersion2 = "v10.10"
		)
		ctx := WithVersion(context.TODO(), expectedVersion)
		if v := FromContext(ctx); v != expectedVersion {
			t.Errorf("expected: %q, got: %q", expectedVersion, v)
		}

		ctx2 := WithVersion(ctx, expectedVersion2)
		if v := FromContext(ctx2); v != expectedVersion2 {
			t.Errorf("expected: %q, got: %q", expectedVersion2, v)
		}
	})
}
