package errdefs

import (
	"errors"
	"fmt"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
)

var errTest = errors.New("this is a test")

type wrapped interface {
	Unwrap() error
}

func TestNotFound(t *testing.T) {
	if cerrdefs.IsNotFound(errTest) {
		t.Fatalf("did not expect not found error, got %T", errTest)
	}
	e := NotFound(errTest)
	if !cerrdefs.IsNotFound(e) {
		t.Fatalf("expected not found error, got: %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected not found error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsNotFound(wrapped) {
		t.Fatalf("expected not found error, got: %T", wrapped)
	}
}

func TestConflict(t *testing.T) {
	if cerrdefs.IsConflict(errTest) {
		t.Fatalf("did not expect conflict error, got %T", errTest)
	}
	e := Conflict(errTest)
	if !cerrdefs.IsConflict(e) {
		t.Fatalf("expected conflict error, got: %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected conflict error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsConflict(wrapped) {
		t.Fatalf("expected conflict error, got: %T", wrapped)
	}
}

func TestForbidden(t *testing.T) {
	if cerrdefs.IsPermissionDenied(errTest) {
		t.Fatalf("did not expect forbidden error, got %T", errTest)
	}
	e := Forbidden(errTest)
	if !cerrdefs.IsPermissionDenied(e) {
		t.Fatalf("expected forbidden error, got: %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected forbidden error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsPermissionDenied(wrapped) {
		t.Fatalf("expected forbidden error, got: %T", wrapped)
	}
}

func TestInvalidParameter(t *testing.T) {
	if cerrdefs.IsInvalidArgument(errTest) {
		t.Fatalf("did not expect invalid argument error, got %T", errTest)
	}
	e := InvalidParameter(errTest)
	if !cerrdefs.IsInvalidArgument(e) {
		t.Fatalf("expected invalid argument error, got %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected invalid argument error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsInvalidArgument(wrapped) {
		t.Fatalf("expected invalid argument error, got: %T", wrapped)
	}
}

func TestNotImplemented(t *testing.T) {
	if cerrdefs.IsNotImplemented(errTest) {
		t.Fatalf("did not expect not implemented error, got %T", errTest)
	}
	e := NotImplemented(errTest)
	if !cerrdefs.IsNotImplemented(e) {
		t.Fatalf("expected not implemented error, got %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected not implemented error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsNotImplemented(wrapped) {
		t.Fatalf("expected not implemented error, got: %T", wrapped)
	}
}

func TestNotModified(t *testing.T) {
	if cerrdefs.IsNotModified(errTest) {
		t.Fatalf("did not expect not modified error, got %T", errTest)
	}
	e := NotModified(errTest)
	if !cerrdefs.IsNotModified(e) {
		t.Fatalf("expected not modified error, got %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected not modified error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsNotModified(wrapped) {
		t.Fatalf("expected not modified error, got: %T", wrapped)
	}
}

func TestUnauthorized(t *testing.T) {
	if cerrdefs.IsUnauthorized(errTest) {
		t.Fatalf("did not expect unauthorized error, got %T", errTest)
	}
	e := Unauthorized(errTest)
	if !cerrdefs.IsUnauthorized(e) {
		t.Fatalf("expected unauthorized error, got %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected unauthorized error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsUnauthorized(wrapped) {
		t.Fatalf("expected unauthorized error, got: %T", wrapped)
	}
}

func TestUnknown(t *testing.T) {
	if cerrdefs.IsUnknown(errTest) {
		t.Fatalf("did not expect unknown error, got %T", errTest)
	}
	e := Unknown(errTest)
	if !cerrdefs.IsUnknown(e) {
		t.Fatalf("expected unknown error, got %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected unknown error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsUnknown(wrapped) {
		t.Fatalf("expected unknown error, got: %T", wrapped)
	}
}

func TestCancelled(t *testing.T) {
	if cerrdefs.IsCanceled(errTest) {
		t.Fatalf("did not expect cancelled error, got %T", errTest)
	}
	e := Cancelled(errTest)
	if !cerrdefs.IsCanceled(e) {
		t.Fatalf("expected cancelled error, got %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected cancelled error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsCanceled(wrapped) {
		t.Fatalf("expected cancelled error, got: %T", wrapped)
	}
}

func TestDeadline(t *testing.T) {
	if cerrdefs.IsDeadlineExceeded(errTest) {
		t.Fatalf("did not expect deadline error, got %T", errTest)
	}
	e := Deadline(errTest)
	if !cerrdefs.IsDeadlineExceeded(e) {
		t.Fatalf("expected deadline error, got %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected deadline error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsDeadlineExceeded(wrapped) {
		t.Fatalf("expected deadline error, got: %T", wrapped)
	}
}

func TestDataLoss(t *testing.T) {
	if cerrdefs.IsDataLoss(errTest) {
		t.Fatalf("did not expect data loss error, got %T", errTest)
	}
	e := DataLoss(errTest)
	if !cerrdefs.IsDataLoss(e) {
		t.Fatalf("expected data loss error, got %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected data loss error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsDataLoss(wrapped) {
		t.Fatalf("expected data loss error, got: %T", wrapped)
	}
}

func TestUnavailable(t *testing.T) {
	if cerrdefs.IsUnavailable(errTest) {
		t.Fatalf("did not expect unavaillable error, got %T", errTest)
	}
	e := Unavailable(errTest)
	if !cerrdefs.IsUnavailable(e) {
		t.Fatalf("expected unavaillable error, got %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected unavaillable error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsUnavailable(wrapped) {
		t.Fatalf("expected unavaillable error, got: %T", wrapped)
	}
}

func TestSystem(t *testing.T) {
	if cerrdefs.IsInternal(errTest) {
		t.Fatalf("did not expect system error, got %T", errTest)
	}
	e := System(errTest)
	if !cerrdefs.IsInternal(e) {
		t.Fatalf("expected system error, got %T", e)
	}
	if cause := e.(wrapped).Unwrap(); cause != errTest { //nolint:errorlint // not using errors.Is, as this tests for the unwrapped error.
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
	if !errors.Is(e, errTest) {
		t.Fatalf("expected system error to match errTest")
	}

	wrapped := fmt.Errorf("foo: %w", e)
	if !cerrdefs.IsInternal(wrapped) {
		t.Fatalf("expected system error, got: %T", wrapped)
	}
}
