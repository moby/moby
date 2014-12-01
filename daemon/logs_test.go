package daemon

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseSinceDate(t *testing.T) {
	tz := strings.Split(time.Now().Local().String(), "+")[1]
	testData := []struct {
		t string
		v string
	}{
		{"2012-03-12 05:04", "2012-03-12 05:04:00 +" + tz},
		{"2012-03-12", "2012-03-12 00:00:00 +" + tz},
		{"05:04", fmt.Sprintf("%d-%02d-%02d 05:04:00 +%s", time.Now().Year(), time.Now().Month(), time.Now().Day(), tz)},
	}

	for _, d := range testData {
		date, err := parseSinceDate(d.t)
		if err != nil {
			t.Errorf("%s: %s", d.t, err)
		}
		if d.v != date.String() {
			t.Errorf("%s: Expecting %v, got %v", d.t, d.v, date)
		}
	}
}

func TestParseSinceDateError(t *testing.T) {
	testData := []struct {
		t string
	}{
		{"05:04 2012-03-12"},
		{"2012-13-03"},
		{"2012-2-3"},
		{"25:66"},
		{"05:60"},
		{"5:6"},
	}

	for _, d := range testData {
		if date, err := parseSinceDate(d.t); err == nil {
			t.Errorf("%s: Expecting error, got %s", d.t, date)
		}
	}
}
