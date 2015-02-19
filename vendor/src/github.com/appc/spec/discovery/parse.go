package discovery

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/appc/spec/schema/types"
)

type App struct {
	Name   types.ACName
	Labels map[string]string
}

func NewApp(name string, labels map[string]string) (*App, error) {
	if labels == nil {
		labels = make(map[string]string, 0)
	}
	acn, err := types.NewACName(name)
	if err != nil {
		return nil, err
	}
	return &App{
		Name:   *acn,
		Labels: labels,
	}, nil
}

// NewAppFromString takes a command line app parameter and returns a map of labels.
//
// Example app parameters:
// 	example.com/reduce-worker:1.0.0
// 	example.com/reduce-worker,channel=alpha,label=value
func NewAppFromString(app string) (*App, error) {
	var (
		name   string
		labels map[string]string
	)

	app = strings.Replace(app, ":", ",version=", -1)
	app = "name=" + app
	v, err := url.ParseQuery(strings.Replace(app, ",", "&", -1))
	if err != nil {
		return nil, err
	}
	labels = make(map[string]string, 0)
	for key, val := range v {
		if len(val) > 1 {
			return nil, fmt.Errorf("label %s with multiple values %q", key, val)
		}
		if key == "name" {
			name = val[0]
			continue
		}
		labels[key] = val[0]
	}
	a, err := NewApp(name, labels)
	if err != nil {
		return nil, err
	}
	return a, nil
}
