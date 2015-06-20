//go:generate msgp

package fluent

//msgp:tuple Entry
type Entry struct {
	Time   int64       `msg:"time"`
	Record interface{} `msg:"record"`
}

//msgp:tuple Forward
type Forward struct {
	Tag     string      `msg:"tag"`
	Entries []Entry     `msg:"entries"`
	Option  interface{} `msg:"option"`
}

//msgp:tuple Message
type Message struct {
	Tag    string      `msg:"tag"`
	Time   int64       `msg:"time"`
	Record interface{} `msg:"record"`
	Option interface{} `msg:"option"`
}
