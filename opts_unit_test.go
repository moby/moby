package docker

import (
	"testing"
)

func TestValidateIP4(t *testing.T) {
	if ret, err := ValidateIp4Address(`1.2.3.4`); err != nil || ret == "" {
		t.Fatalf("ValidateIp4Address(`1.2.3.4`) got %s %s", ret, err)
	}

	if ret, err := ValidateIp4Address(`127.0.0.1`); err != nil || ret == "" {
		t.Fatalf("ValidateIp4Address(`127.0.0.1`) got %s %s", ret, err)
	}

	if ret, err := ValidateIp4Address(`127`); err == nil || ret != "" {
		t.Fatalf("ValidateIp4Address(`127`) got %s %s", ret, err)
	}

	if ret, err := ValidateIp4Address(`random invalid string`); err == nil || ret != "" {
		t.Fatalf("ValidateIp4Address(`random invalid string`) got %s %s", ret, err)
	}

}
