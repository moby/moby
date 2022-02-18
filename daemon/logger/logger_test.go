package logger // import "github.com/moby/moby/daemon/logger"

import (
	"github.com/moby/moby/api/types/backend"
)

func (m *Message) copy() *Message {
	msg := &Message{
		Source:       m.Source,
		PLogMetaData: m.PLogMetaData,
		Timestamp:    m.Timestamp,
	}

	if m.Attrs != nil {
		msg.Attrs = make([]backend.LogAttr, len(m.Attrs))
		copy(msg.Attrs, m.Attrs)
	}

	msg.Line = append(make([]byte, 0, len(m.Line)), m.Line...)
	return msg
}
