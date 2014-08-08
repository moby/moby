package helpinfo

import (
	"testing"
)

func TestHelpInfo(t *testing.T) {
	testInfo := HelpInfo{}
	testInfo.Add("images", "filter", Blurb{"dangling = <true>", "shows only images with no name", "(would show in the list as <none>)"})
	if len(testInfo.Commands()) != 1 || testInfo.Commands()[0] != "images" {
		t.Fatalf("expected only 1 command, got %d. %#v", len(testInfo.Commands()), testInfo)
	}
	if len(testInfo.Flags("images")) != 1 || testInfo.Flags("images")[0] != "filter" {
		t.Fatalf("expected only 1 flag for images, got %d. %#v", len(testInfo.Flags("images")), testInfo)
	}
	testInfo.Add("ps", "filter", Blurb{"exited = <int>", "some text here"})
	if len(testInfo.Commands()) != 2 {
		t.Fatalf("expected 2 commands, got %d. %#v", len(testInfo.Commands()), testInfo)
	}
	if len(testInfo.Flags("ps")) != 1 || testInfo.Flags("ps")[0] != "filter" {
		t.Fatalf("expected only 1 flag for images, got %d. %#v", len(testInfo.Flags("ps")), testInfo)
	}
}

func TestFormat(t *testing.T) {
	testInfo := HelpInfo{}
	for i := 0; i < 3; i++ {
		testInfo.Add("images", "filter", Blurb{"dangling = <true>", "shows only images with no name", "(would show in the list as <none>)"})
	}

	expected := `- dangling = <true>
  shows only images with no name
  (would show in the list as <none>)
- dangling = <true>
  shows only images with no name
  (would show in the list as <none>)
- dangling = <true>
  shows only images with no name
  (would show in the list as <none>)`
	got := UsageFormat(testInfo.Blurbs("images", "filter"))
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}

}
