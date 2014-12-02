package derror

import (
	"errors"
	"fmt"
)

type Derror struct {
	error
	messageID string
	args      []interface{}
}

type messageEntry struct {
	rc  int    // HTTP response code
	str string // Text of message with Sprintf sub vars
}

func New(msgID interface{}, args ...interface{}) *Derror {
	var id string

	if derr, ok := msgID.(*Derror); ok {
		return derr
	} else if err, ok := msgID.(error); ok {
		id = "Generic"
		args = []interface{}{err.Error()}
	} else if err, ok := msgID.(*error); ok {
		id = "Generic"
		args = []interface{}{(*err).Error()}
	} else {
		id = msgID.(string)
	}

	text := T(id, args...)
	return &Derror{errors.New(text), id, args}
}

func (derr *Derror) Is(id string) bool {
	return derr.messageID == id
}

func (derr *Derror) MessageID() string {
	return derr.messageID
}

func (derr *Derror) Args() []interface{} {
	return derr.args
}

func (derr *Derror) Message() string {
	return T(derr.messageID, derr.args...)
}

func (derr *Derror) Status() int {
	entry, exists := msgs[derr.messageID]
	if !exists {
		entry = msgs["Generic"]
	}

	return entry.rc
}

func (derr Derror) Error() string {
	return derr.Message()
}

func Is(err error, id string) bool {
	if err == nil {
		return false
	}
	if err1, ok := err.(*Derror); ok && err1.Is(id) {
		return true
	}
	return false
}

func T(msgID string, args ...interface{}) string {

	entry, exists := msgs[msgID]
	if !exists {
		entry = msgs["Generic"]
	}

	return fmt.Sprintf(entry.str, args...)
}

var msgs = map[string]messageEntry{
	"Generic":            {502, "%s"},
	"NoContainerID":      {404, "No SUCH container: %s"},
	"NoImageID":          {404, "No SUCH image: %s"},
	"NoContainerImageID": {404, "No SUCH image or container: %s"},
}
