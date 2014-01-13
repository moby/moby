package metricdriver

import (
	"fmt"
)

type InitFunc func() (Driver, error)

type Memory struct {
	Rss int64
}

type Cpu struct {
	LoadAverage float64
	NumOfCPU    int
}

type Metric struct {
	Cpu    *Cpu
	Memory *Memory
}

var drivers map[string]InitFunc

func init() {
	drivers = make(map[string]InitFunc)
}

func Register(name string, initFunc InitFunc) error {
	if _, exists := drivers[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}
	drivers[name] = initFunc

	return nil
}

func GetDriver(name string) (Driver, error) {
	if initFunc, exists := drivers[name]; exists {
		return initFunc()
	}
	return nil, fmt.Errorf("No such driver: %s", name)
}

func NewMetric() *Metric {
	return &Metric{Cpu: &Cpu{}, Memory: &Memory{}}
}

type Driver interface {
	Get(id string) (metric *Metric, err error)
}
