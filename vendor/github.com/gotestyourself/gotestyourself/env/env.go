/*Package env provides functions to test code that read environment variables
or the current working directory.
*/
package env

import (
	"os"
	"strings"

	"github.com/gotestyourself/gotestyourself/assert"
)

type helperT interface {
	Helper()
}

// Patch changes the value of an environment variable, and returns a
// function which will reset the the value of that variable back to the
// previous state.
func Patch(t assert.TestingT, key, value string) func() {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	oldValue, ok := os.LookupEnv(key)
	assert.NilError(t, os.Setenv(key, value))
	return func() {
		if ht, ok := t.(helperT); ok {
			ht.Helper()
		}
		if !ok {
			assert.NilError(t, os.Unsetenv(key))
			return
		}
		assert.NilError(t, os.Setenv(key, oldValue))
	}
}

// PatchAll sets the environment to env, and returns a function which will
// reset the environment back to the previous state.
func PatchAll(t assert.TestingT, env map[string]string) func() {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	oldEnv := os.Environ()
	os.Clearenv()

	for key, value := range env {
		assert.NilError(t, os.Setenv(key, value))
	}
	return func() {
		if ht, ok := t.(helperT); ok {
			ht.Helper()
		}
		os.Clearenv()
		for key, oldVal := range ToMap(oldEnv) {
			assert.NilError(t, os.Setenv(key, oldVal))
		}
	}
}

// ToMap takes a list of strings in the format returned by os.Environ() and
// returns a mapping of keys to values.
func ToMap(env []string) map[string]string {
	result := map[string]string{}
	for _, raw := range env {
		parts := strings.SplitN(raw, "=", 2)
		switch len(parts) {
		case 1:
			result[raw] = ""
		case 2:
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// ChangeWorkingDir to the directory, and return a function which restores the
// previous working directory.
func ChangeWorkingDir(t assert.TestingT, dir string) func() {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	cwd, err := os.Getwd()
	assert.NilError(t, err)
	assert.NilError(t, os.Chdir(dir))
	return func() {
		if ht, ok := t.(helperT); ok {
			ht.Helper()
		}
		assert.NilError(t, os.Chdir(cwd))
	}
}
