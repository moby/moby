package omslogs

import (
	"bytes"
	"sync"
)

// OmsLogClientMock mocks an OmsLogClient.
type OmsLogClientMock struct {
	lock     sync.RWMutex
	messages []string
}

// PostData saves the buffer for later analysis.
func (l *OmsLogClientMock) PostData(data *[]byte, name string) error {
	l.lock.Lock()
	defer l.lock.Unlock()

	buffer := bytes.NewBuffer(*data)
	l.messages = append(l.messages, buffer.String())

	return nil
}

func (l *OmsLogClientMock) getMessages() []string {
	l.lock.Lock()
	defer l.lock.Unlock()

	messages := l.messages[0:]
	l.messages = l.messages[:0]

	return messages
}
