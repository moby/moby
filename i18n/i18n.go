package i18n

import "github.com/docker/docker/pkg/jsonmessage"

// Our message catalog/IDs
const (
	Generic            string = "Generic"
	NoContainerID      string = "NoContainerID"
	NoImageID          string = "NoImageID"
	NoContainerImageID string = "NoContainerImageID"
)

func Init() {
	jsonmessage.SetMessageMap(&msgs)
}

func Is(e error, id string) bool {
	return jsonmessage.Is(e, id)
}

func NewError(msgID interface{}, args ...interface{}) error {
	var id string

	if e, ok := msgID.(*jsonmessage.JSONError); ok {
		return e
	} else if err, ok := msgID.(error); ok {
		id = Generic
		args = []interface{}{err.Error()}
	} else if err, ok := msgID.(*error); ok {
		id = Generic
		args = []interface{}{(*err).Error()}
	} else {
		id = msgID.(string)
	}

	// text := jsonmessage.T(id, args...)
	return jsonmessage.NewError(id, args...)
}

var (
	msgs = jsonmessage.MessageMap{
		Generic:            {502, "%s"},
		NoContainerID:      {404, "No SUCH container: %s"},
		NoImageID:          {404, "No SUCH image: %s"},
		NoContainerImageID: {404, "No SUCH image or container: %s"},
	}
)
