package progressui

import (
	"os"
	"runtime"

	"github.com/morikuni/aec"
)

var colorRun aec.ANSI
var colorCancel aec.ANSI
var colorWarning aec.ANSI
var colorError aec.ANSI

func init() {
	// As recommended on https://no-color.org/
	if v := os.Getenv("NO_COLOR"); v != "" {
		// nil values will result in no ANSI color codes being emitted.
		return
	} else if runtime.GOOS == "windows" {
		colorRun = termColorMap["cyan"]
		colorCancel = termColorMap["yellow"]
		colorWarning = termColorMap["yellow"]
		colorError = termColorMap["red"]
	} else {
		colorRun = termColorMap["blue"]
		colorCancel = termColorMap["yellow"]
		colorWarning = termColorMap["yellow"]
		colorError = termColorMap["red"]
	}

	// Loosely based on the standard set by Linux LS_COLORS.
	if _, ok := os.LookupEnv("BUILDKIT_COLORS"); ok {
		envColorString := os.Getenv("BUILDKIT_COLORS")
		setUserDefinedTermColors(envColorString)
	}
}
