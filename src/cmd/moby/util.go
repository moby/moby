package main

import "os"

func getStringValue(envKey string, flagVal string, defaultVal string) string {
	var res string

	// If defined, take the env variable
	if _, ok := os.LookupEnv(envKey); ok {
		res = os.Getenv(envKey)
	}

	// If a flag is specified, this value takes precedence
	// Ignore cases where the flag carries the default value
	if flagVal != "" && flagVal != defaultVal {
		res = flagVal
	}

	// if we still don't have a value, use the default
	if res == "" {
		res = defaultVal
	}
	return res
}

func getBoolValue(envKey string, flagVal bool) bool {
	var res bool

	// If defined, take the env variable
	if _, ok := os.LookupEnv(envKey); ok {
		switch os.Getenv(envKey) {
		case "":
			res = false
		case "0":
			res = false
		case "false":
			res = false
		case "FALSE":
			res = false
		case "1":
			res = true
		default:
			// catches "true", "TRUE" or anything else
			res = true

		}
	}

	// If a flag is specified, this value takes precedence
	if res != flagVal {
		res = flagVal
	}

	return res
}
