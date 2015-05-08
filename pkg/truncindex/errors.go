package truncindex

import "fmt"

type ErrEmptyPrefix struct{}

func (err ErrEmptyPrefix) Error() string {
	return fmt.Sprint("Prefix can't be empty")
}

type ErrAmbiguousPrefix struct{}

func (err ErrAmbiguousPrefix) Error() string {
	return fmt.Sprint("Multiple IDs found with provided prefix")
}

type ErrNoSuchID struct {
	ID string
}

func (err ErrNoSuchID) Error() string {
	return fmt.Sprintf("no such id: '%s'", err.ID)
}

type ErrIllegalChar struct{}

func (err ErrIllegalChar) Error() string {
	return fmt.Sprint("illegal character: ' '")
}

type ErrIDAlreadyExists struct {
	ID string
}

func (err ErrIDAlreadyExists) Error() string {
	return fmt.Sprintf("id already exists: '%s'", err.ID)
}

type ErrInsertFailed struct {
	ID string
}

func (err ErrInsertFailed) Error() string {
	return fmt.Sprintf("failed to insert id: %s", err.ID)
}
