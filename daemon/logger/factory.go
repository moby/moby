package logger

import (
	"fmt"
	"sync"
)

// Creator is a method that builds a logging driver instance with given context
type Creator func(Context) (Logger, error)

// Context provides enough information for a logging driver to do its function
type Context struct {
	Config        map[string]string
	ContainerID   string
	ContainerName string
	LogPath       string
}

type logdriverFactory struct {
	registry map[string]Creator
	m        sync.Mutex
}

func (lf *logdriverFactory) register(name string, c Creator) error {
	lf.m.Lock()
	defer lf.m.Unlock()

	if _, ok := lf.registry[name]; ok {
		return fmt.Errorf("logger: log driver named '%s' is already registered", name)
	}
	lf.registry[name] = c
	return nil
}

func (lf *logdriverFactory) get(name string) (Creator, error) {
	lf.m.Lock()
	defer lf.m.Unlock()

	c, ok := lf.registry[name]
	if !ok {
		return c, fmt.Errorf("logger: no log driver named '%s' is registered", name)
	}
	return c, nil
}

var factory = &logdriverFactory{registry: make(map[string]Creator)} // global factory instance

// RegisterLogDriver registers the given logging driver builder with given logging
// driver name.
func RegisterLogDriver(name string, c Creator) error {
	return factory.register(name, c)
}

// GetLogDriver provides the logging driver builder for a logging driver name.
func GetLogDriver(name string) (Creator, error) {
	return factory.get(name)
}
