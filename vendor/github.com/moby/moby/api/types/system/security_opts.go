package system

// SecurityOpt contains the name and options of a security option
type SecurityOpt struct {
	Name    string
	Options []KeyValue
}

// KeyValue holds a key/value pair.
type KeyValue struct {
	Key, Value string
}
