package logger

func (m *Message) copy() *Message {
	msg := &Message{
		Source:    m.Source,
		Partial:   m.Partial,
		Timestamp: m.Timestamp,
	}

	if m.Attrs != nil {
		msg.Attrs = make(map[string]string, len(m.Attrs))
		for k, v := range m.Attrs {
			msg.Attrs[k] = v
		}
	}

	msg.Line = append(make([]byte, 0, len(m.Line)), m.Line...)
	return msg
}
