package logger

import (
	"fmt"
	"sync"
)

// Creator builds a logging driver instance with given context.
type Creator func(Info) (Logger, error)

// LogOptValidator checks the options specific to the underlying
// logging implementation.
type LogOptValidator func(cfg map[string]string) error

// LogOptOutput generates the displayable options for `docker info`.
type LogOptOutput func(cfg map[string]string) (map[string]string, error)

type logdriverFactory struct {
	registry     map[string]Creator
	optValidator map[string]LogOptValidator
	optOutput    map[string]LogOptOutput
	m            sync.Mutex
}

func (lf *logdriverFactory) register(name string, c Creator) error {
	if lf.driverRegistered(name) {
		return fmt.Errorf("logger: log driver named '%s' is already registered", name)
	}

	lf.m.Lock()
	lf.registry[name] = c
	lf.m.Unlock()
	return nil
}

func (lf *logdriverFactory) driverRegistered(name string) bool {
	lf.m.Lock()
	_, ok := lf.registry[name]
	lf.m.Unlock()
	return ok
}

func (lf *logdriverFactory) registerLogOptValidator(name string, l LogOptValidator) error {
	lf.m.Lock()
	defer lf.m.Unlock()

	if _, ok := lf.optValidator[name]; ok {
		return fmt.Errorf("logger: log validator named '%s' is already registered", name)
	}
	lf.optValidator[name] = l
	return nil
}

func (lf *logdriverFactory) registerLogOptOutput(name string, l LogOptOutput) error {
	lf.m.Lock()
	defer lf.m.Unlock()

	if _, ok := lf.optOutput[name]; ok {
		return fmt.Errorf("logger: log output named '%s' is already registered", name)
	}
	lf.optOutput[name] = l
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

func (lf *logdriverFactory) getLogOptValidator(name string) LogOptValidator {
	lf.m.Lock()
	defer lf.m.Unlock()

	c, _ := lf.optValidator[name]
	return c
}

func (lf *logdriverFactory) getLogOptOutput(name string) LogOptOutput {
	lf.m.Lock()
	defer lf.m.Unlock()

	c, _ := lf.optOutput[name]
	return c
}

var factory = &logdriverFactory{
	registry:     make(map[string]Creator),
	optValidator: make(map[string]LogOptValidator),
	optOutput:    make(map[string]LogOptOutput),
} // global factory instance

// RegisterLogDriver registers the given logging driver builder with given logging
// driver name.
func RegisterLogDriver(name string, c Creator) error {
	return factory.register(name, c)
}

// RegisterLogOptValidator registers the logging option validator with
// the given logging driver name.
func RegisterLogOptValidator(name string, l LogOptValidator) error {
	return factory.registerLogOptValidator(name, l)
}

// RegisterLogOptOutput registers the logging option output with
// the given logging driver name.
func RegisterLogOptOutput(name string, l LogOptOutput) error {
	return factory.registerLogOptOutput(name, l)
}

// GetLogDriver provides the logging driver builder for a logging driver name.
func GetLogDriver(name string) (Creator, error) {
	return factory.get(name)
}

// ValidateLogOpts checks the options for the given log driver. The
// options supported are specific to the LogDriver implementation.
func ValidateLogOpts(name string, cfg map[string]string) error {
	if name == "none" {
		return nil
	}

	if !factory.driverRegistered(name) {
		return fmt.Errorf("logger: no log driver named '%s' is registered", name)
	}

	validator := factory.getLogOptValidator(name)
	if validator != nil {
		return validator(cfg)
	}
	return nil
}

// OutputLogOpts takes the options for the given log driver, and
// generate the displayable options for `docker info`.
// This is useful in case some of the information in the options
// is not suitable for `docker info` (e.g., tokens for splunk)
// A log driver could register the OutputLogOpts to customerize
// the information to output. Otherwise all options will be output.
func OutputLogOpts(name string, cfg map[string]string) (map[string]string, error) {
	if name == "none" {
		return nil, nil
	}

	if !factory.driverRegistered(name) {
		return nil, fmt.Errorf("logger: no log driver named '%s' is registered", name)
	}

	output := factory.getLogOptOutput(name)
	if output != nil {
		return output(cfg)
	}
	return cfg, nil
}
