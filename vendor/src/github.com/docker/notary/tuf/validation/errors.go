package validation

import (
	"encoding/json"
	"fmt"
)

// VALIDATION ERRORS

// ErrValidation represents a general validation error
type ErrValidation struct {
	Msg string
}

func (err ErrValidation) Error() string {
	return fmt.Sprintf("An error occurred during validation: %s", err.Msg)
}

// ErrBadHierarchy represents missing metadata.  Currently: a missing snapshot
// at this current time. When delegations are implemented it will also
// represent a missing delegation parent
type ErrBadHierarchy struct {
	Missing string
	Msg     string
}

func (err ErrBadHierarchy) Error() string {
	return fmt.Sprintf("Metadata hierarchy is incomplete: %s", err.Msg)
}

// ErrBadRoot represents a failure validating the root
type ErrBadRoot struct {
	Msg string
}

func (err ErrBadRoot) Error() string {
	return fmt.Sprintf("The root metadata is invalid: %s", err.Msg)
}

// ErrBadTargets represents a failure to validate a targets (incl delegations)
type ErrBadTargets struct {
	Msg string
}

func (err ErrBadTargets) Error() string {
	return fmt.Sprintf("The targets metadata is invalid: %s", err.Msg)
}

// ErrBadSnapshot represents a failure to validate the snapshot
type ErrBadSnapshot struct {
	Msg string
}

func (err ErrBadSnapshot) Error() string {
	return fmt.Sprintf("The snapshot metadata is invalid: %s", err.Msg)
}

// END VALIDATION ERRORS

// SerializableError is a struct that can be used to serialize an error as JSON
type SerializableError struct {
	Name  string
	Error error
}

// UnmarshalJSON attempts to unmarshal the error into the right type
func (s *SerializableError) UnmarshalJSON(text []byte) (err error) {
	var x struct{ Name string }
	err = json.Unmarshal(text, &x)
	if err != nil {
		return
	}
	var theError error
	switch x.Name {
	case "ErrValidation":
		var e struct{ Error ErrValidation }
		err = json.Unmarshal(text, &e)
		theError = e.Error
	case "ErrBadHierarchy":
		var e struct{ Error ErrBadHierarchy }
		err = json.Unmarshal(text, &e)
		theError = e.Error
	case "ErrBadRoot":
		var e struct{ Error ErrBadRoot }
		err = json.Unmarshal(text, &e)
		theError = e.Error
	case "ErrBadTargets":
		var e struct{ Error ErrBadTargets }
		err = json.Unmarshal(text, &e)
		theError = e.Error
	case "ErrBadSnapshot":
		var e struct{ Error ErrBadSnapshot }
		err = json.Unmarshal(text, &e)
		theError = e.Error
	default:
		err = fmt.Errorf("do not know how to unmarshal %s", x.Name)
		return
	}
	if err != nil {
		return
	}
	s.Name = x.Name
	s.Error = theError
	return nil
}

// NewSerializableError serializes one of the above errors into JSON
func NewSerializableError(err error) (*SerializableError, error) {
	// make sure it's one of our errors
	var name string
	switch err.(type) {
	case ErrValidation:
		name = "ErrValidation"
	case ErrBadHierarchy:
		name = "ErrBadHierarchy"
	case ErrBadRoot:
		name = "ErrBadRoot"
	case ErrBadTargets:
		name = "ErrBadTargets"
	case ErrBadSnapshot:
		name = "ErrBadSnapshot"
	default:
		return nil, fmt.Errorf("does not support serializing non-validation errors")
	}
	return &SerializableError{Name: name, Error: err}, nil
}
