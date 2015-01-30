// +build unix linux freebsd

package syslog

import realSyslog "log/syslog"

type syslogWriter struct {
	w *realSyslog.Writer
}

func New(priority Priority, tag string) (Writer, error) {
	return Dial("", "", priority, tag)
}

func Dial(network, raddr string, priority Priority, tag string) (Writer, error) {
	w, err := realSyslog.Dial(network, raddr, realSyslog.Priority(priority), tag)
	if err != nil {
		return nil, err
	}

	return &syslogWriter{w}, nil
}

func (sw *syslogWriter) Write(b []byte) (int, error) {
	return sw.w.Write(b)
}

func (sw *syslogWriter) Close() error {
	return sw.w.Close()
}
