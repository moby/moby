package image

import (
	"runtime"
	"testing"
)

func TestValidateOSCompatibility(t *testing.T) {
	err := ValidateOSCompatibility(runtime.GOOS, runtime.GOARCH, getOSVersion(), nil)
	if err != nil {
		t.Error(err)
	}

	err = ValidateOSCompatibility("DOS", runtime.GOARCH, getOSVersion(), nil)
	if err == nil {
		t.Error("expected OS compat error")
	}

	err = ValidateOSCompatibility(runtime.GOOS, "pdp-11", getOSVersion(), nil)
	if err == nil {
		t.Error("expected architecture compat error")
	}

	err = ValidateOSCompatibility(runtime.GOOS, runtime.GOARCH, "98 SE", nil)
	if err == nil {
		t.Error("expected OS version compat error")
	}
}
