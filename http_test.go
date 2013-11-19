package docker

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBoolParam(t *testing.T) {
	if ret, err := getBoolParam("true"); err != nil || !ret {
		t.Fatalf("true -> true, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("True"); err != nil || !ret {
		t.Fatalf("True -> true, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("1"); err != nil || !ret {
		t.Fatalf("1 -> true, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam(""); err != nil || ret {
		t.Fatalf("\"\" -> false, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("false"); err != nil || ret {
		t.Fatalf("false -> false, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("0"); err != nil || ret {
		t.Fatalf("0 -> false, nil | got %t %s", ret, err)
	}
	if ret, err := getBoolParam("faux"); err == nil || ret {
		t.Fatalf("faux -> false, err | got %t %s", ret, err)
	}
}

func TesthttpError(t *testing.T) {
	r := httptest.NewRecorder()

	httpError(r, fmt.Errorf("No such method"))
	if r.Code != http.StatusNotFound {
		t.Fatalf("Expected %d, got %d", http.StatusNotFound, r.Code)
	}

	httpError(r, fmt.Errorf("This accound hasn't been activated"))
	if r.Code != http.StatusForbidden {
		t.Fatalf("Expected %d, got %d", http.StatusForbidden, r.Code)
	}

	httpError(r, fmt.Errorf("Some error"))
	if r.Code != http.StatusInternalServerError {
		t.Fatalf("Expected %d, got %d", http.StatusInternalServerError, r.Code)
	}
}
