package color

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/mattn/go-colorable"
)

// Testing colors is kinda different. First we test for given colors and their
// escaped formatted results. Next we create some visual tests to be tested.
// Each visual test includes the color name to be compared.
func TestColor(t *testing.T) {
	rb := new(bytes.Buffer)
	Output = rb

	NoColor = false

	testColors := []struct {
		text string
		code Attribute
	}{
		{text: "black", code: FgBlack},
		{text: "red", code: FgRed},
		{text: "green", code: FgGreen},
		{text: "yellow", code: FgYellow},
		{text: "blue", code: FgBlue},
		{text: "magent", code: FgMagenta},
		{text: "cyan", code: FgCyan},
		{text: "white", code: FgWhite},
		{text: "hblack", code: FgHiBlack},
		{text: "hred", code: FgHiRed},
		{text: "hgreen", code: FgHiGreen},
		{text: "hyellow", code: FgHiYellow},
		{text: "hblue", code: FgHiBlue},
		{text: "hmagent", code: FgHiMagenta},
		{text: "hcyan", code: FgHiCyan},
		{text: "hwhite", code: FgHiWhite},
	}

	for _, c := range testColors {
		New(c.code).Print(c.text)

		line, _ := rb.ReadString('\n')
		scannedLine := fmt.Sprintf("%q", line)
		colored := fmt.Sprintf("\x1b[%dm%s\x1b[0m", c.code, c.text)
		escapedForm := fmt.Sprintf("%q", colored)

		fmt.Printf("%s\t: %s\n", c.text, line)

		if scannedLine != escapedForm {
			t.Errorf("Expecting %s, got '%s'\n", escapedForm, scannedLine)
		}
	}
}

func TestColorEquals(t *testing.T) {
	fgblack1 := New(FgBlack)
	fgblack2 := New(FgBlack)
	bgblack := New(BgBlack)
	fgbgblack := New(FgBlack, BgBlack)
	fgblackbgred := New(FgBlack, BgRed)
	fgred := New(FgRed)
	bgred := New(BgRed)

	if !fgblack1.Equals(fgblack2) {
		t.Error("Two black colors are not equal")
	}

	if fgblack1.Equals(bgblack) {
		t.Error("Fg and bg black colors are equal")
	}

	if fgblack1.Equals(fgbgblack) {
		t.Error("Fg black equals fg/bg black color")
	}

	if fgblack1.Equals(fgred) {
		t.Error("Fg black equals Fg red")
	}

	if fgblack1.Equals(bgred) {
		t.Error("Fg black equals Bg red")
	}

	if fgblack1.Equals(fgblackbgred) {
		t.Error("Fg black equals fg black bg red")
	}
}

func TestNoColor(t *testing.T) {
	rb := new(bytes.Buffer)
	Output = rb

	testColors := []struct {
		text string
		code Attribute
	}{
		{text: "black", code: FgBlack},
		{text: "red", code: FgRed},
		{text: "green", code: FgGreen},
		{text: "yellow", code: FgYellow},
		{text: "blue", code: FgBlue},
		{text: "magent", code: FgMagenta},
		{text: "cyan", code: FgCyan},
		{text: "white", code: FgWhite},
		{text: "hblack", code: FgHiBlack},
		{text: "hred", code: FgHiRed},
		{text: "hgreen", code: FgHiGreen},
		{text: "hyellow", code: FgHiYellow},
		{text: "hblue", code: FgHiBlue},
		{text: "hmagent", code: FgHiMagenta},
		{text: "hcyan", code: FgHiCyan},
		{text: "hwhite", code: FgHiWhite},
	}

	for _, c := range testColors {
		p := New(c.code)
		p.DisableColor()
		p.Print(c.text)

		line, _ := rb.ReadString('\n')
		if line != c.text {
			t.Errorf("Expecting %s, got '%s'\n", c.text, line)
		}
	}

	// global check
	NoColor = true
	defer func() {
		NoColor = false
	}()
	for _, c := range testColors {
		p := New(c.code)
		p.Print(c.text)

		line, _ := rb.ReadString('\n')
		if line != c.text {
			t.Errorf("Expecting %s, got '%s'\n", c.text, line)
		}
	}

}

func TestColorVisual(t *testing.T) {
	// First Visual Test
	Output = colorable.NewColorableStdout()

	New(FgRed).Printf("red\t")
	New(BgRed).Print("         ")
	New(FgRed, Bold).Println(" red")

	New(FgGreen).Printf("green\t")
	New(BgGreen).Print("         ")
	New(FgGreen, Bold).Println(" green")

	New(FgYellow).Printf("yellow\t")
	New(BgYellow).Print("         ")
	New(FgYellow, Bold).Println(" yellow")

	New(FgBlue).Printf("blue\t")
	New(BgBlue).Print("         ")
	New(FgBlue, Bold).Println(" blue")

	New(FgMagenta).Printf("magenta\t")
	New(BgMagenta).Print("         ")
	New(FgMagenta, Bold).Println(" magenta")

	New(FgCyan).Printf("cyan\t")
	New(BgCyan).Print("         ")
	New(FgCyan, Bold).Println(" cyan")

	New(FgWhite).Printf("white\t")
	New(BgWhite).Print("         ")
	New(FgWhite, Bold).Println(" white")
	fmt.Println("")

	// Second Visual test
	Black("black")
	Red("red")
	Green("green")
	Yellow("yellow")
	Blue("blue")
	Magenta("magenta")
	Cyan("cyan")
	White("white")

	// Third visual test
	fmt.Println()
	Set(FgBlue)
	fmt.Println("is this blue?")
	Unset()

	Set(FgMagenta)
	fmt.Println("and this magenta?")
	Unset()

	// Fourth Visual test
	fmt.Println()
	blue := New(FgBlue).PrintlnFunc()
	blue("blue text with custom print func")

	red := New(FgRed).PrintfFunc()
	red("red text with a printf func: %d\n", 123)

	put := New(FgYellow).SprintFunc()
	warn := New(FgRed).SprintFunc()

	fmt.Fprintf(Output, "this is a %s and this is %s.\n", put("warning"), warn("error"))

	info := New(FgWhite, BgGreen).SprintFunc()
	fmt.Fprintf(Output, "this %s rocks!\n", info("package"))

	// Fifth Visual Test
	fmt.Println()

	fmt.Fprintln(Output, BlackString("black"))
	fmt.Fprintln(Output, RedString("red"))
	fmt.Fprintln(Output, GreenString("green"))
	fmt.Fprintln(Output, YellowString("yellow"))
	fmt.Fprintln(Output, BlueString("blue"))
	fmt.Fprintln(Output, MagentaString("magenta"))
	fmt.Fprintln(Output, CyanString("cyan"))
	fmt.Fprintln(Output, WhiteString("white"))
}
