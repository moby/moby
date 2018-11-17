package testutils

import "github.com/sirupsen/logrus"

type logHook struct {
	enabled bool
	entries []*logrus.Entry
}

func (h *logHook) clear() {
	h.entries = []*logrus.Entry{}
}

func (h *logHook) getEntries() []*logrus.Entry {
	return h.entries
}

func (h *logHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel}
}

func (h *logHook) Fire(entry *logrus.Entry) error {
	if !h.enabled {
		return nil
	}

	h.entries = append(h.entries, entry)
	return nil
}

// Make sure hook implements the logrus.Hook interface
var _ logrus.Hook = (*logHook)(nil)

var logHookInstance *logHook

func EnableLogHook() {
	if logHookInstance == nil {
		logHookInstance = &logHook{enabled: true}
		logrus.AddHook(logHookInstance)
	}

	logHookInstance.clear()
}

func GetLogHookEntries() []*logrus.Entry {
	if logHookInstance == nil {
		return nil
	}

	return logHookInstance.getEntries()
}

func DisableLogHook() {
	if logHookInstance == nil {
		return
	}

	logHookInstance.enabled = false
}
