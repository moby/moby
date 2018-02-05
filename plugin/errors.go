package plugin // import "github.com/docker/docker/plugin"

import "fmt"

type errNotFound string

func (name errNotFound) Error() string {
	return fmt.Sprintf("plugin %q not found", string(name))
}

func (errNotFound) NotFound() {}

type errAmbiguous string

func (name errAmbiguous) Error() string {
	return fmt.Sprintf("multiple plugins found for %q", string(name))
}

func (name errAmbiguous) InvalidParameter() {}

type errDisabled string

func (name errDisabled) Error() string {
	return fmt.Sprintf("plugin %s found but disabled", string(name))
}

func (name errDisabled) Conflict() {}

type invalidFilter struct {
	filter string
	value  []string
}

func (e invalidFilter) Error() string {
	msg := "Invalid filter '" + e.filter
	if len(e.value) > 0 {
		msg += fmt.Sprintf("=%s", e.value)
	}
	return msg + "'"
}

func (invalidFilter) InvalidParameter() {}

type inUseError string

func (e inUseError) Error() string {
	return "plugin " + string(e) + " is in use"
}

func (inUseError) Conflict() {}

type enabledError string

func (e enabledError) Error() string {
	return "plugin " + string(e) + " is enabled"
}

func (enabledError) Conflict() {}

type alreadyExistsError string

func (e alreadyExistsError) Error() string {
	return "plugin " + string(e) + " already exists"
}

func (alreadyExistsError) Conflict() {}
