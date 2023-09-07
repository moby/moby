package progressui

import (
	"encoding/csv"
	"errors"
	"strconv"
	"strings"

	"github.com/morikuni/aec"
	"github.com/sirupsen/logrus"
)

var termColorMap = map[string]aec.ANSI{
	"default": aec.DefaultF,

	"black":   aec.BlackF,
	"blue":    aec.BlueF,
	"cyan":    aec.CyanF,
	"green":   aec.GreenF,
	"magenta": aec.MagentaF,
	"red":     aec.RedF,
	"white":   aec.WhiteF,
	"yellow":  aec.YellowF,

	"light-black":   aec.LightBlackF,
	"light-blue":    aec.LightBlueF,
	"light-cyan":    aec.LightCyanF,
	"light-green":   aec.LightGreenF,
	"light-magenta": aec.LightMagentaF,
	"light-red":     aec.LightRedF,
	"light-white":   aec.LightWhiteF,
	"light-yellow":  aec.LightYellowF,
}

func setUserDefinedTermColors(colorsEnv string) {
	fields := readBuildkitColorsEnv(colorsEnv)
	if fields == nil {
		return
	}
	for _, field := range fields {
		k, v, ok := strings.Cut(field, "=")
		if !ok || strings.Contains(v, "=") {
			err := errors.New("A valid entry must have exactly two fields")
			logrus.WithError(err).Warnf("Could not parse BUILDKIT_COLORS component: %s", field)
			continue
		}
		k = strings.ToLower(k)
		if c, ok := termColorMap[strings.ToLower(v)]; ok {
			parseKeys(k, c)
		} else if strings.Contains(v, ",") {
			if c := readRGB(v); c != nil {
				parseKeys(k, c)
			}
		} else {
			err := errors.New("Colors must be a name from the pre-defined list or a valid 3-part RGB value")
			logrus.WithError(err).Warnf("Unknown color value found in BUILDKIT_COLORS: %s=%s", k, v)
		}
	}
}

func readBuildkitColorsEnv(colorsEnv string) []string {
	csvReader := csv.NewReader(strings.NewReader(colorsEnv))
	csvReader.Comma = ':'
	fields, err := csvReader.Read()
	if err != nil {
		logrus.WithError(err).Warnf("Could not parse BUILDKIT_COLORS. Falling back to defaults.")
		return nil
	}
	return fields
}

func readRGB(v string) aec.ANSI {
	csvReader := csv.NewReader(strings.NewReader(v))
	fields, err := csvReader.Read()
	if err != nil {
		logrus.WithError(err).Warnf("Could not parse value %s as valid comma-separated RGB color. Ignoring.", v)
		return nil
	}
	if len(fields) != 3 {
		err = errors.New("A valid RGB color must have three fields")
		logrus.WithError(err).Warnf("Could not parse value %s as valid RGB color. Ignoring.", v)
		return nil
	}
	ok := isValidRGB(fields)
	if ok {
		p1, _ := strconv.Atoi(fields[0])
		p2, _ := strconv.Atoi(fields[1])
		p3, _ := strconv.Atoi(fields[2])
		c := aec.Color8BitF(aec.NewRGB8Bit(uint8(p1), uint8(p2), uint8(p3)))
		return c
	}
	return nil
}

func parseKeys(k string, c aec.ANSI) {
	switch strings.ToLower(k) {
	case "run":
		colorRun = c
	case "cancel":
		colorCancel = c
	case "error":
		colorError = c
	case "warning":
		colorWarning = c
	default:
		logrus.Warnf("Unknown key found in BUILDKIT_COLORS (expected: run, cancel, error, or warning): %s", k)
	}
}

func isValidRGB(s []string) bool {
	for _, n := range s {
		num, err := strconv.Atoi(n)
		if err != nil {
			logrus.Warnf("A field in BUILDKIT_COLORS appears to contain an RGB value that is not an integer: %s", strings.Join(s, ","))
			return false
		}
		ok := isValidRGBValue(num)
		if ok {
			continue
		} else {
			logrus.Warnf("A field in BUILDKIT_COLORS appears to contain an RGB value that is not within the valid range of 0-255: %s", strings.Join(s, ","))
			return false
		}
	}
	return true
}

func isValidRGBValue(i int) bool {
	if (i >= 0) && (i <= 255) {
		return true
	}
	return false
}
