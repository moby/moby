package errdefs // import "github.com/docker/docker/errdefs"

import (
	"errors"
	"testing"

	pkgErrors "github.com/pkg/errors"
)

var errTest = errors.New("this is a test")

type causal interface {
	Cause() error
}

func TestNotFound(t *testing.T) {
	if IsNotFound(errTest) {
		t.Fatalf("did not expect not found error, got %T", errTest)
	}
	e := NotFound(errTest)
	if !IsNotFound(e) {
		t.Fatalf("expected not found error, got: %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected NotFound to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestConflict(t *testing.T) {
	if IsConflict(errTest) {
		t.Fatalf("did not expect conflict error, got %T", errTest)
	}
	e := Conflict(errTest)
	if !IsConflict(e) {
		t.Fatalf("expected conflict error, got: %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected Conflict to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestForbidden(t *testing.T) {
	if IsForbidden(errTest) {
		t.Fatalf("did not expect forbidden error, got %T", errTest)
	}
	e := Forbidden(errTest)
	if !IsForbidden(e) {
		t.Fatalf("expected forbidden error, got: %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected Forbidden to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestInvalidParameter(t *testing.T) {
	if IsInvalidParameter(errTest) {
		t.Fatalf("did not expect invalid argument error, got %T", errTest)
	}
	e := InvalidParameter(errTest)
	if !IsInvalidParameter(e) {
		t.Fatalf("expected invalid argument error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected InvalidParameter to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestNotImplemented(t *testing.T) {
	if IsNotImplemented(errTest) {
		t.Fatalf("did not expect not implemented error, got %T", errTest)
	}
	e := NotImplemented(errTest)
	if !IsNotImplemented(e) {
		t.Fatalf("expected not implemented error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected NotImplemented to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestNotModified(t *testing.T) {
	if IsNotModified(errTest) {
		t.Fatalf("did not expect not modified error, got %T", errTest)
	}
	e := NotModified(errTest)
	if !IsNotModified(e) {
		t.Fatalf("expected not modified error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected NotModified to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestAlreadyExists(t *testing.T) {
	if IsAlreadyExists(errTest) {
		t.Fatalf("did not expect already exists error, got %T", errTest)
	}
	e := AlreadyExists(errTest)
	if !IsAlreadyExists(e) {
		t.Fatalf("expected already exists error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected AlreadyExists to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestUnauthorized(t *testing.T) {
	if IsUnauthorized(errTest) {
		t.Fatalf("did not expect unauthorized error, got %T", errTest)
	}
	e := Unauthorized(errTest)
	if !IsUnauthorized(e) {
		t.Fatalf("expected unauthorized error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected Unauthorized to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestUnknown(t *testing.T) {
	if IsUnknown(errTest) {
		t.Fatalf("did not expect unknown error, got %T", errTest)
	}
	e := Unknown(errTest)
	if !IsUnknown(e) {
		t.Fatalf("expected unknown error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected Unknown to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestCancelled(t *testing.T) {
	if IsCancelled(errTest) {
		t.Fatalf("did not expect cancelled error, got %T", errTest)
	}
	e := Cancelled(errTest)
	if !IsCancelled(e) {
		t.Fatalf("expected cancelled error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected Canceled to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestDeadline(t *testing.T) {
	if IsDeadline(errTest) {
		t.Fatalf("did not expect deadline error, got %T", errTest)
	}
	e := Deadline(errTest)
	if !IsDeadline(e) {
		t.Fatalf("expected deadline error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected Deadline to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestDataLoss(t *testing.T) {
	if IsDataLoss(errTest) {
		t.Fatalf("did not expect data loss error, got %T", errTest)
	}
	e := DataLoss(errTest)
	if !IsDataLoss(e) {
		t.Fatalf("expected data loss error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected DataLoss to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestUnavailable(t *testing.T) {
	if IsUnavailable(errTest) {
		t.Fatalf("did not expect unavaillable error, got %T", errTest)
	}
	e := Unavailable(errTest)
	if !IsUnavailable(e) {
		t.Fatalf("expected unavaillable error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected Unavailable to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestSystem(t *testing.T) {
	if IsSystem(errTest) {
		t.Fatalf("did not expect system error, got %T", errTest)
	}
	e := System(errTest)
	if !IsSystem(e) {
		t.Fatalf("expected system error, got %T", e)
	}
	if !IsKnownErrorType(e) {
		t.Fatalf("expected System to be known error type")
	}
	if cause := e.(causal).Cause(); cause != errTest {
		t.Fatalf("causual should be errTest, got: %v", cause)
	}
}

func TestKnownErrorTypeRecursive(t *testing.T) {
	e := pkgErrors.Wrap(
		pkgErrors.Wrap(Unknown(errTest), "wrapping this error"),
		"wrapping it again",
	)

	if !IsKnownErrorType(e) {
		t.Fatalf("expected a wrapped error to still be a known error type")
	}
}
