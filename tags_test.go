package docker

import (
	"testing"
)

func TestLookupImage(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	if img, err := runtime.repositories.LookupImage(unitTestImageName); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}

	if img, err := runtime.repositories.LookupImage(unitTestImageName + ":" + DEFAULTTAG); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}

	if img, err := runtime.repositories.LookupImage(unitTestImageName + ":" + "fail"); err == nil {
		t.Errorf("Expected error, none found")
	} else if img != nil {
		t.Errorf("Expected 0 image, 1 found")
	}

	if img, err := runtime.repositories.LookupImage("fail:fail"); err == nil {
		t.Errorf("Expected error, none found")
	} else if img != nil {
		t.Errorf("Expected 0 image, 1 found")
	}

	if img, err := runtime.repositories.LookupImage(unitTestImageID); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}

	if img, err := runtime.repositories.LookupImage(unitTestImageName + ":" + unitTestImageID); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}
}
