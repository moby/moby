package store

type ErrMetaNotFound struct{}

func (err ErrMetaNotFound) Error() string {
	return "no trust data available"
}
