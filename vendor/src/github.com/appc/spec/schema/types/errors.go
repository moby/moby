package types

import "fmt"

// An ACKindError is returned when the wrong ACKind is set in a manifest
type ACKindError string

func (e ACKindError) Error() string {
	return string(e)
}

func InvalidACKindError(kind ACKind) ACKindError {
	return ACKindError(fmt.Sprintf("missing or bad ACKind (must be %#v)", kind))
}

// An ACVersionError is returned when a bad ACVersion is set in a manifest
type ACVersionError string

func (e ACVersionError) Error() string {
	return string(e)
}

// An ACNameError is returned when a bad value is used for an ACName
type ACNameError string

func (e ACNameError) Error() string {
	return string(e)
}

// An AMStartedOnError is returned when the wrong StartedOn is set in an ImageManifest
type AMStartedOnError string

func (e AMStartedOnError) Error() string {
	return string(e)
}
