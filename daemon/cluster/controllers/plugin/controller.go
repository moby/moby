package plugin

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
)

// Controller is the controller for the plugin backend
type Controller struct{}

// NewController returns a new cluster plugin controller
func NewController() (*Controller, error) {
	return &Controller{}, nil
}

// Update is the update phase from swarmkit
func (p *Controller) Update(ctx context.Context, t *api.Task) error {
	logrus.WithFields(logrus.Fields{
		"controller": "plugin",
	}).Debug("Update")
	return nil
}

// Prepare is the prepare phase from swarmkit
func (p *Controller) Prepare(ctx context.Context) error {
	logrus.WithFields(logrus.Fields{
		"controller": "plugin",
	}).Debug("Prepare")
	return nil
}

// Start is the start phase from swarmkit
func (p *Controller) Start(ctx context.Context) error {
	logrus.WithFields(logrus.Fields{
		"controller": "plugin",
	}).Debug("Start")
	return nil
}

// Wait causes the task to wait until returned
func (p *Controller) Wait(ctx context.Context) error {
	logrus.WithFields(logrus.Fields{
		"controller": "plugin",
	}).Debug("Wait")
	return nil
}

// Shutdown is the shutdown phase from swarmkit
func (p *Controller) Shutdown(ctx context.Context) error {
	logrus.WithFields(logrus.Fields{
		"controller": "plugin",
	}).Debug("Shutdown")
	return nil
}

// Terminate is the terminate phase from swarmkit
func (p *Controller) Terminate(ctx context.Context) error {
	logrus.WithFields(logrus.Fields{
		"controller": "plugin",
	}).Debug("Terminate")
	return nil
}

// Remove is the remove phase from swarmkit
func (p *Controller) Remove(ctx context.Context) error {
	logrus.WithFields(logrus.Fields{
		"controller": "plugin",
	}).Debug("Remove")
	return nil
}

// Close is the close phase from swarmkit
func (p *Controller) Close() error {
	logrus.WithFields(logrus.Fields{
		"controller": "plugin",
	}).Debug("Close")
	return nil
}
