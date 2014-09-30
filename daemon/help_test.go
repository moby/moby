package daemon

import (
	"testing"
)

func TestHelpInfo(t *testing.T) {
	testInfo := HelpInfo{}
	testInfo.Add("images", "filter", Blurb{"dangling=<true|false>", "shows only images with no name", "(would show in the list as <none>)"})
	if len(testInfo.Commands()) != 1 || testInfo.Commands()[0] != "images" {
		t.Fatalf("expected only 1 command, got %d. %#v", len(testInfo.Commands()), testInfo)
	}
	if len(testInfo.Flags("images")) != 1 || testInfo.Flags("images")[0] != "filter" {
		t.Fatalf("expected only 1 flag for images, got %d. %#v", len(testInfo.Flags("images")), testInfo)
	}
}
