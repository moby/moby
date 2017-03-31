package cli

import (
    "testing"
    "errors"
)

func TestTwoErrors(t *testing.T) {
    var errorsList Errors
    errorsList = append(errorsList, errors.New("Error1"))
    errorsList = append(errorsList, errors.New("Error2"))

    if errorsList.Error() != "Error1, Error2" {
        t.Error(errorsList)
    }
}

func TestEmptyErrors(t *testing.T) {
    errorsList := make(Errors, 0)

    if errorsList.Error() != "" {
        t.Error(errorsList)
    }
}