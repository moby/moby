package systembackend

// SecurityOption contains the name and options of a security option
type SecurityOption struct {
	Name    string
	Options []KeyValue
}

// KeyValue holds a key/value pair.
type KeyValue struct {
	Key, Value string
}
