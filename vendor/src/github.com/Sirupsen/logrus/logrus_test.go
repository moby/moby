package logrus

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func LogAndAssertJSON(t *testing.T, log func(*Logger), assertions func(fields Fields)) {
	var buffer bytes.Buffer
	var fields Fields

	logger := New()
	logger.Out = &buffer
	logger.Formatter = new(JSONFormatter)

	log(logger)

	err := json.Unmarshal(buffer.Bytes(), &fields)
	assert.Nil(t, err)

	assertions(fields)
}

func TestPrint(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.Print("test")
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "test")
		assert.Equal(t, fields["level"], "info")
	})
}

func TestInfo(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.Info("test")
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "test")
		assert.Equal(t, fields["level"], "info")
	})
}

func TestWarn(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.Warn("test")
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "test")
		assert.Equal(t, fields["level"], "warning")
	})
}

func TestInfolnShouldAddSpacesBetweenStrings(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.Infoln("test", "test")
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "test test")
	})
}

func TestInfolnShouldAddSpacesBetweenStringAndNonstring(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.Infoln("test", 10)
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "test 10")
	})
}

func TestInfolnShouldAddSpacesBetweenTwoNonStrings(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.Infoln(10, 10)
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "10 10")
	})
}

func TestInfoShouldAddSpacesBetweenTwoNonStrings(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.Infoln(10, 10)
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "10 10")
	})
}

func TestInfoShouldNotAddSpacesBetweenStringAndNonstring(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.Info("test", 10)
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "test10")
	})
}

func TestInfoShouldNotAddSpacesBetweenStrings(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.Info("test", "test")
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "testtest")
	})
}

func TestWithFieldsShouldAllowAssignments(t *testing.T) {
	var buffer bytes.Buffer
	var fields Fields

	logger := New()
	logger.Out = &buffer
	logger.Formatter = new(JSONFormatter)

	localLog := logger.WithFields(Fields{
		"key1": "value1",
	})

	localLog.WithField("key2", "value2").Info("test")
	err := json.Unmarshal(buffer.Bytes(), &fields)
	assert.Nil(t, err)

	assert.Equal(t, "value2", fields["key2"])
	assert.Equal(t, "value1", fields["key1"])

	buffer = bytes.Buffer{}
	fields = Fields{}
	localLog.Info("test")
	err = json.Unmarshal(buffer.Bytes(), &fields)
	assert.Nil(t, err)

	_, ok := fields["key2"]
	assert.Equal(t, false, ok)
	assert.Equal(t, "value1", fields["key1"])
}

func TestUserSuppliedFieldDoesNotOverwriteDefaults(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.WithField("msg", "hello").Info("test")
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "test")
	})
}

func TestUserSuppliedMsgFieldHasPrefix(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.WithField("msg", "hello").Info("test")
	}, func(fields Fields) {
		assert.Equal(t, fields["msg"], "test")
		assert.Equal(t, fields["fields.msg"], "hello")
	})
}

func TestUserSuppliedTimeFieldHasPrefix(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.WithField("time", "hello").Info("test")
	}, func(fields Fields) {
		assert.Equal(t, fields["fields.time"], "hello")
	})
}

func TestUserSuppliedLevelFieldHasPrefix(t *testing.T) {
	LogAndAssertJSON(t, func(log *Logger) {
		log.WithField("level", 1).Info("test")
	}, func(fields Fields) {
		assert.Equal(t, fields["level"], "info")
		assert.Equal(t, fields["fields.level"], 1)
	})
}

func TestConvertLevelToString(t *testing.T) {
	assert.Equal(t, "debug", DebugLevel.String())
	assert.Equal(t, "info", InfoLevel.String())
	assert.Equal(t, "warning", WarnLevel.String())
	assert.Equal(t, "error", ErrorLevel.String())
	assert.Equal(t, "fatal", FatalLevel.String())
	assert.Equal(t, "panic", PanicLevel.String())
}
