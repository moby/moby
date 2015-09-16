package cli

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

var boolFlagTests = []struct {
	name     string
	expected string
}{
	{"help", "--help\t"},
	{"h", "-h\t"},
}

func TestBoolFlagHelpOutput(t *testing.T) {

	for _, test := range boolFlagTests {
		flag := BoolFlag{Name: test.name}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

var stringFlagTests = []struct {
	name     string
	value    string
	expected string
}{
	{"help", "", "--help \t"},
	{"h", "", "-h \t"},
	{"h", "", "-h \t"},
	{"test", "Something", "--test \"Something\"\t"},
}

func TestStringFlagHelpOutput(t *testing.T) {

	for _, test := range stringFlagTests {
		flag := StringFlag{Name: test.name, Value: test.value}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestStringFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_FOO", "derp")
	for _, test := range stringFlagTests {
		flag := StringFlag{Name: test.name, Value: test.value, EnvVar: "APP_FOO"}
		output := flag.String()

		if !strings.HasSuffix(output, " [$APP_FOO]") {
			t.Errorf("%s does not end with [$APP_FOO]", output)
		}
	}
}

var stringSliceFlagTests = []struct {
	name     string
	value    *StringSlice
	expected string
}{
	{"help", func() *StringSlice {
		s := &StringSlice{}
		s.Set("")
		return s
	}(), "--help [--help option --help option]\t"},
	{"h", func() *StringSlice {
		s := &StringSlice{}
		s.Set("")
		return s
	}(), "-h [-h option -h option]\t"},
	{"h", func() *StringSlice {
		s := &StringSlice{}
		s.Set("")
		return s
	}(), "-h [-h option -h option]\t"},
	{"test", func() *StringSlice {
		s := &StringSlice{}
		s.Set("Something")
		return s
	}(), "--test [--test option --test option]\t"},
}

func TestStringSliceFlagHelpOutput(t *testing.T) {

	for _, test := range stringSliceFlagTests {
		flag := StringSliceFlag{Name: test.name, Value: test.value}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestStringSliceFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_QWWX", "11,4")
	for _, test := range stringSliceFlagTests {
		flag := StringSliceFlag{Name: test.name, Value: test.value, EnvVar: "APP_QWWX"}
		output := flag.String()

		if !strings.HasSuffix(output, " [$APP_QWWX]") {
			t.Errorf("%q does not end with [$APP_QWWX]", output)
		}
	}
}

var intFlagTests = []struct {
	name     string
	expected string
}{
	{"help", "--help \"0\"\t"},
	{"h", "-h \"0\"\t"},
}

func TestIntFlagHelpOutput(t *testing.T) {

	for _, test := range intFlagTests {
		flag := IntFlag{Name: test.name}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestIntFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_BAR", "2")
	for _, test := range intFlagTests {
		flag := IntFlag{Name: test.name, EnvVar: "APP_BAR"}
		output := flag.String()

		if !strings.HasSuffix(output, " [$APP_BAR]") {
			t.Errorf("%s does not end with [$APP_BAR]", output)
		}
	}
}

var durationFlagTests = []struct {
	name     string
	expected string
}{
	{"help", "--help \"0\"\t"},
	{"h", "-h \"0\"\t"},
}

func TestDurationFlagHelpOutput(t *testing.T) {

	for _, test := range durationFlagTests {
		flag := DurationFlag{Name: test.name}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestDurationFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_BAR", "2h3m6s")
	for _, test := range durationFlagTests {
		flag := DurationFlag{Name: test.name, EnvVar: "APP_BAR"}
		output := flag.String()

		if !strings.HasSuffix(output, " [$APP_BAR]") {
			t.Errorf("%s does not end with [$APP_BAR]", output)
		}
	}
}

var intSliceFlagTests = []struct {
	name     string
	value    *IntSlice
	expected string
}{
	{"help", &IntSlice{}, "--help [--help option --help option]\t"},
	{"h", &IntSlice{}, "-h [-h option -h option]\t"},
	{"h", &IntSlice{}, "-h [-h option -h option]\t"},
	{"test", func() *IntSlice {
		i := &IntSlice{}
		i.Set("9")
		return i
	}(), "--test [--test option --test option]\t"},
}

func TestIntSliceFlagHelpOutput(t *testing.T) {

	for _, test := range intSliceFlagTests {
		flag := IntSliceFlag{Name: test.name, Value: test.value}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestIntSliceFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_SMURF", "42,3")
	for _, test := range intSliceFlagTests {
		flag := IntSliceFlag{Name: test.name, Value: test.value, EnvVar: "APP_SMURF"}
		output := flag.String()

		if !strings.HasSuffix(output, " [$APP_SMURF]") {
			t.Errorf("%q does not end with [$APP_SMURF]", output)
		}
	}
}

var float64FlagTests = []struct {
	name     string
	expected string
}{
	{"help", "--help \"0\"\t"},
	{"h", "-h \"0\"\t"},
}

func TestFloat64FlagHelpOutput(t *testing.T) {

	for _, test := range float64FlagTests {
		flag := Float64Flag{Name: test.name}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%s does not match %s", output, test.expected)
		}
	}
}

func TestFloat64FlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_BAZ", "99.4")
	for _, test := range float64FlagTests {
		flag := Float64Flag{Name: test.name, EnvVar: "APP_BAZ"}
		output := flag.String()

		if !strings.HasSuffix(output, " [$APP_BAZ]") {
			t.Errorf("%s does not end with [$APP_BAZ]", output)
		}
	}
}

var genericFlagTests = []struct {
	name     string
	value    Generic
	expected string
}{
	{"test", &Parser{"abc", "def"}, "--test \"abc,def\"\ttest flag"},
	{"t", &Parser{"abc", "def"}, "-t \"abc,def\"\ttest flag"},
}

func TestGenericFlagHelpOutput(t *testing.T) {

	for _, test := range genericFlagTests {
		flag := GenericFlag{Name: test.name, Value: test.value, Usage: "test flag"}
		output := flag.String()

		if output != test.expected {
			t.Errorf("%q does not match %q", output, test.expected)
		}
	}
}

func TestGenericFlagWithEnvVarHelpOutput(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_ZAP", "3")
	for _, test := range genericFlagTests {
		flag := GenericFlag{Name: test.name, EnvVar: "APP_ZAP"}
		output := flag.String()

		if !strings.HasSuffix(output, " [$APP_ZAP]") {
			t.Errorf("%s does not end with [$APP_ZAP]", output)
		}
	}
}

func TestParseMultiString(t *testing.T) {
	(&App{
		Flags: []Flag{
			StringFlag{Name: "serve, s"},
		},
		Action: func(ctx *Context) {
			if ctx.String("serve") != "10" {
				t.Errorf("main name not set")
			}
			if ctx.String("s") != "10" {
				t.Errorf("short name not set")
			}
		},
	}).Run([]string{"run", "-s", "10"})
}

func TestParseMultiStringFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_COUNT", "20")
	(&App{
		Flags: []Flag{
			StringFlag{Name: "count, c", EnvVar: "APP_COUNT"},
		},
		Action: func(ctx *Context) {
			if ctx.String("count") != "20" {
				t.Errorf("main name not set")
			}
			if ctx.String("c") != "20" {
				t.Errorf("short name not set")
			}
		},
	}).Run([]string{"run"})
}

func TestParseMultiStringFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_COUNT", "20")
	(&App{
		Flags: []Flag{
			StringFlag{Name: "count, c", EnvVar: "COMPAT_COUNT,APP_COUNT"},
		},
		Action: func(ctx *Context) {
			if ctx.String("count") != "20" {
				t.Errorf("main name not set")
			}
			if ctx.String("c") != "20" {
				t.Errorf("short name not set")
			}
		},
	}).Run([]string{"run"})
}

func TestParseMultiStringSlice(t *testing.T) {
	(&App{
		Flags: []Flag{
			StringSliceFlag{Name: "serve, s", Value: &StringSlice{}},
		},
		Action: func(ctx *Context) {
			if !reflect.DeepEqual(ctx.StringSlice("serve"), []string{"10", "20"}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.StringSlice("s"), []string{"10", "20"}) {
				t.Errorf("short name not set")
			}
		},
	}).Run([]string{"run", "-s", "10", "-s", "20"})
}

func TestParseMultiStringSliceFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	(&App{
		Flags: []Flag{
			StringSliceFlag{Name: "intervals, i", Value: &StringSlice{}, EnvVar: "APP_INTERVALS"},
		},
		Action: func(ctx *Context) {
			if !reflect.DeepEqual(ctx.StringSlice("intervals"), []string{"20", "30", "40"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.StringSlice("i"), []string{"20", "30", "40"}) {
				t.Errorf("short name not set from env")
			}
		},
	}).Run([]string{"run"})
}

func TestParseMultiStringSliceFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	(&App{
		Flags: []Flag{
			StringSliceFlag{Name: "intervals, i", Value: &StringSlice{}, EnvVar: "COMPAT_INTERVALS,APP_INTERVALS"},
		},
		Action: func(ctx *Context) {
			if !reflect.DeepEqual(ctx.StringSlice("intervals"), []string{"20", "30", "40"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.StringSlice("i"), []string{"20", "30", "40"}) {
				t.Errorf("short name not set from env")
			}
		},
	}).Run([]string{"run"})
}

func TestParseMultiInt(t *testing.T) {
	a := App{
		Flags: []Flag{
			IntFlag{Name: "serve, s"},
		},
		Action: func(ctx *Context) {
			if ctx.Int("serve") != 10 {
				t.Errorf("main name not set")
			}
			if ctx.Int("s") != 10 {
				t.Errorf("short name not set")
			}
		},
	}
	a.Run([]string{"run", "-s", "10"})
}

func TestParseMultiIntFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "10")
	a := App{
		Flags: []Flag{
			IntFlag{Name: "timeout, t", EnvVar: "APP_TIMEOUT_SECONDS"},
		},
		Action: func(ctx *Context) {
			if ctx.Int("timeout") != 10 {
				t.Errorf("main name not set")
			}
			if ctx.Int("t") != 10 {
				t.Errorf("short name not set")
			}
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiIntFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "10")
	a := App{
		Flags: []Flag{
			IntFlag{Name: "timeout, t", EnvVar: "COMPAT_TIMEOUT_SECONDS,APP_TIMEOUT_SECONDS"},
		},
		Action: func(ctx *Context) {
			if ctx.Int("timeout") != 10 {
				t.Errorf("main name not set")
			}
			if ctx.Int("t") != 10 {
				t.Errorf("short name not set")
			}
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiIntSlice(t *testing.T) {
	(&App{
		Flags: []Flag{
			IntSliceFlag{Name: "serve, s", Value: &IntSlice{}},
		},
		Action: func(ctx *Context) {
			if !reflect.DeepEqual(ctx.IntSlice("serve"), []int{10, 20}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.IntSlice("s"), []int{10, 20}) {
				t.Errorf("short name not set")
			}
		},
	}).Run([]string{"run", "-s", "10", "-s", "20"})
}

func TestParseMultiIntSliceFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	(&App{
		Flags: []Flag{
			IntSliceFlag{Name: "intervals, i", Value: &IntSlice{}, EnvVar: "APP_INTERVALS"},
		},
		Action: func(ctx *Context) {
			if !reflect.DeepEqual(ctx.IntSlice("intervals"), []int{20, 30, 40}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.IntSlice("i"), []int{20, 30, 40}) {
				t.Errorf("short name not set from env")
			}
		},
	}).Run([]string{"run"})
}

func TestParseMultiIntSliceFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_INTERVALS", "20,30,40")

	(&App{
		Flags: []Flag{
			IntSliceFlag{Name: "intervals, i", Value: &IntSlice{}, EnvVar: "COMPAT_INTERVALS,APP_INTERVALS"},
		},
		Action: func(ctx *Context) {
			if !reflect.DeepEqual(ctx.IntSlice("intervals"), []int{20, 30, 40}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.IntSlice("i"), []int{20, 30, 40}) {
				t.Errorf("short name not set from env")
			}
		},
	}).Run([]string{"run"})
}

func TestParseMultiFloat64(t *testing.T) {
	a := App{
		Flags: []Flag{
			Float64Flag{Name: "serve, s"},
		},
		Action: func(ctx *Context) {
			if ctx.Float64("serve") != 10.2 {
				t.Errorf("main name not set")
			}
			if ctx.Float64("s") != 10.2 {
				t.Errorf("short name not set")
			}
		},
	}
	a.Run([]string{"run", "-s", "10.2"})
}

func TestParseMultiFloat64FromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "15.5")
	a := App{
		Flags: []Flag{
			Float64Flag{Name: "timeout, t", EnvVar: "APP_TIMEOUT_SECONDS"},
		},
		Action: func(ctx *Context) {
			if ctx.Float64("timeout") != 15.5 {
				t.Errorf("main name not set")
			}
			if ctx.Float64("t") != 15.5 {
				t.Errorf("short name not set")
			}
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiFloat64FromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_TIMEOUT_SECONDS", "15.5")
	a := App{
		Flags: []Flag{
			Float64Flag{Name: "timeout, t", EnvVar: "COMPAT_TIMEOUT_SECONDS,APP_TIMEOUT_SECONDS"},
		},
		Action: func(ctx *Context) {
			if ctx.Float64("timeout") != 15.5 {
				t.Errorf("main name not set")
			}
			if ctx.Float64("t") != 15.5 {
				t.Errorf("short name not set")
			}
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiBool(t *testing.T) {
	a := App{
		Flags: []Flag{
			BoolFlag{Name: "serve, s"},
		},
		Action: func(ctx *Context) {
			if ctx.Bool("serve") != true {
				t.Errorf("main name not set")
			}
			if ctx.Bool("s") != true {
				t.Errorf("short name not set")
			}
		},
	}
	a.Run([]string{"run", "--serve"})
}

func TestParseMultiBoolFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_DEBUG", "1")
	a := App{
		Flags: []Flag{
			BoolFlag{Name: "debug, d", EnvVar: "APP_DEBUG"},
		},
		Action: func(ctx *Context) {
			if ctx.Bool("debug") != true {
				t.Errorf("main name not set from env")
			}
			if ctx.Bool("d") != true {
				t.Errorf("short name not set from env")
			}
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiBoolFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_DEBUG", "1")
	a := App{
		Flags: []Flag{
			BoolFlag{Name: "debug, d", EnvVar: "COMPAT_DEBUG,APP_DEBUG"},
		},
		Action: func(ctx *Context) {
			if ctx.Bool("debug") != true {
				t.Errorf("main name not set from env")
			}
			if ctx.Bool("d") != true {
				t.Errorf("short name not set from env")
			}
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiBoolT(t *testing.T) {
	a := App{
		Flags: []Flag{
			BoolTFlag{Name: "serve, s"},
		},
		Action: func(ctx *Context) {
			if ctx.BoolT("serve") != true {
				t.Errorf("main name not set")
			}
			if ctx.BoolT("s") != true {
				t.Errorf("short name not set")
			}
		},
	}
	a.Run([]string{"run", "--serve"})
}

func TestParseMultiBoolTFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_DEBUG", "0")
	a := App{
		Flags: []Flag{
			BoolTFlag{Name: "debug, d", EnvVar: "APP_DEBUG"},
		},
		Action: func(ctx *Context) {
			if ctx.BoolT("debug") != false {
				t.Errorf("main name not set from env")
			}
			if ctx.BoolT("d") != false {
				t.Errorf("short name not set from env")
			}
		},
	}
	a.Run([]string{"run"})
}

func TestParseMultiBoolTFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_DEBUG", "0")
	a := App{
		Flags: []Flag{
			BoolTFlag{Name: "debug, d", EnvVar: "COMPAT_DEBUG,APP_DEBUG"},
		},
		Action: func(ctx *Context) {
			if ctx.BoolT("debug") != false {
				t.Errorf("main name not set from env")
			}
			if ctx.BoolT("d") != false {
				t.Errorf("short name not set from env")
			}
		},
	}
	a.Run([]string{"run"})
}

type Parser [2]string

func (p *Parser) Set(value string) error {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return fmt.Errorf("invalid format")
	}

	(*p)[0] = parts[0]
	(*p)[1] = parts[1]

	return nil
}

func (p *Parser) String() string {
	return fmt.Sprintf("%s,%s", p[0], p[1])
}

func TestParseGeneric(t *testing.T) {
	a := App{
		Flags: []Flag{
			GenericFlag{Name: "serve, s", Value: &Parser{}},
		},
		Action: func(ctx *Context) {
			if !reflect.DeepEqual(ctx.Generic("serve"), &Parser{"10", "20"}) {
				t.Errorf("main name not set")
			}
			if !reflect.DeepEqual(ctx.Generic("s"), &Parser{"10", "20"}) {
				t.Errorf("short name not set")
			}
		},
	}
	a.Run([]string{"run", "-s", "10,20"})
}

func TestParseGenericFromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_SERVE", "20,30")
	a := App{
		Flags: []Flag{
			GenericFlag{Name: "serve, s", Value: &Parser{}, EnvVar: "APP_SERVE"},
		},
		Action: func(ctx *Context) {
			if !reflect.DeepEqual(ctx.Generic("serve"), &Parser{"20", "30"}) {
				t.Errorf("main name not set from env")
			}
			if !reflect.DeepEqual(ctx.Generic("s"), &Parser{"20", "30"}) {
				t.Errorf("short name not set from env")
			}
		},
	}
	a.Run([]string{"run"})
}

func TestParseGenericFromEnvCascade(t *testing.T) {
	os.Clearenv()
	os.Setenv("APP_FOO", "99,2000")
	a := App{
		Flags: []Flag{
			GenericFlag{Name: "foos", Value: &Parser{}, EnvVar: "COMPAT_FOO,APP_FOO"},
		},
		Action: func(ctx *Context) {
			if !reflect.DeepEqual(ctx.Generic("foos"), &Parser{"99", "2000"}) {
				t.Errorf("value not set from env")
			}
		},
	}
	a.Run([]string{"run"})
}
