package dispatcher

import (
	"fmt"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/equality"
	"github.com/docker/swarmkit/api/validation"
	"github.com/docker/swarmkit/manager/drivers"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/sirupsen/logrus"
)

type typeAndID struct {
	id      string
	objType api.ResourceType
}

type assignmentSet struct {
	dp                   *drivers.DriverProvider
	tasksMap             map[string]*api.Task
	tasksUsingDependency map[typeAndID]map[string]struct{}
	changes              map[typeAndID]*api.AssignmentChange
	log                  *logrus.Entry
}

func newAssignmentSet(log *logrus.Entry, dp *drivers.DriverProvider) *assignmentSet {
	return &assignmentSet{
		dp:                   dp,
		changes:              make(map[typeAndID]*api.AssignmentChange),
		tasksMap:             make(map[string]*api.Task),
		tasksUsingDependency: make(map[typeAndID]map[string]struct{}),
		log:                  log,
	}
}

func assignSecret(a *assignmentSet, readTx store.ReadTx, mapKey typeAndID, t *api.Task) {
	a.tasksUsingDependency[mapKey] = make(map[string]struct{})
	secret, err := a.secret(readTx, t, mapKey.id)
	if err != nil {
		a.log.WithFields(logrus.Fields{
			"resource.type": "secret",
			"secret.id":     mapKey.id,
			"error":         err,
		}).Debug("failed to fetch secret")
		return
	}
	a.changes[mapKey] = &api.AssignmentChange{
		Assignment: &api.Assignment{
			Item: &api.Assignment_Secret{
				Secret: secret,
			},
		},
		Action: api.AssignmentChange_AssignmentActionUpdate,
	}
}

func assignConfig(a *assignmentSet, readTx store.ReadTx, mapKey typeAndID) {
	a.tasksUsingDependency[mapKey] = make(map[string]struct{})
	config := store.GetConfig(readTx, mapKey.id)
	if config == nil {
		a.log.WithFields(logrus.Fields{
			"resource.type": "config",
			"config.id":     mapKey.id,
		}).Debug("config not found")
		return
	}
	a.changes[mapKey] = &api.AssignmentChange{
		Assignment: &api.Assignment{
			Item: &api.Assignment_Config{
				Config: config,
			},
		},
		Action: api.AssignmentChange_AssignmentActionUpdate,
	}
}

func (a *assignmentSet) addTaskDependencies(readTx store.ReadTx, t *api.Task) {
	for _, resourceRef := range t.Spec.ResourceReferences {
		mapKey := typeAndID{objType: resourceRef.ResourceType, id: resourceRef.ResourceID}
		if len(a.tasksUsingDependency[mapKey]) == 0 {
			switch resourceRef.ResourceType {
			case api.ResourceType_SECRET:
				assignSecret(a, readTx, mapKey, t)
			case api.ResourceType_CONFIG:
				assignConfig(a, readTx, mapKey)
			default:
				a.log.WithField(
					"resource.type", resourceRef.ResourceType,
				).Debug("invalid resource type for a task dependency, skipping")
				continue
			}
		}
		a.tasksUsingDependency[mapKey][t.ID] = struct{}{}
	}

	var secrets []*api.SecretReference
	container := t.Spec.GetContainer()
	if container != nil {
		secrets = container.Secrets
	}

	for _, secretRef := range secrets {
		secretID := secretRef.SecretID
		mapKey := typeAndID{objType: api.ResourceType_SECRET, id: secretID}

		if len(a.tasksUsingDependency[mapKey]) == 0 {
			assignSecret(a, readTx, mapKey, t)
		}
		a.tasksUsingDependency[mapKey][t.ID] = struct{}{}
	}

	var configs []*api.ConfigReference
	if container != nil {
		configs = container.Configs
	}
	for _, configRef := range configs {
		configID := configRef.ConfigID
		mapKey := typeAndID{objType: api.ResourceType_CONFIG, id: configID}

		if len(a.tasksUsingDependency[mapKey]) == 0 {
			assignConfig(a, readTx, mapKey)
		}
		a.tasksUsingDependency[mapKey][t.ID] = struct{}{}
	}
}

func (a *assignmentSet) releaseDependency(mapKey typeAndID, assignment *api.Assignment, taskID string) bool {
	delete(a.tasksUsingDependency[mapKey], taskID)
	if len(a.tasksUsingDependency[mapKey]) != 0 {
		return false
	}
	// No tasks are using the dependency anymore
	delete(a.tasksUsingDependency, mapKey)
	a.changes[mapKey] = &api.AssignmentChange{
		Assignment: assignment,
		Action:     api.AssignmentChange_AssignmentActionRemove,
	}
	return true
}

func (a *assignmentSet) releaseTaskDependencies(t *api.Task) bool {
	var modified bool

	for _, resourceRef := range t.Spec.ResourceReferences {
		var assignment *api.Assignment
		switch resourceRef.ResourceType {
		case api.ResourceType_SECRET:
			assignment = &api.Assignment{
				Item: &api.Assignment_Secret{
					Secret: &api.Secret{ID: resourceRef.ResourceID},
				},
			}
		case api.ResourceType_CONFIG:
			assignment = &api.Assignment{
				Item: &api.Assignment_Config{
					Config: &api.Config{ID: resourceRef.ResourceID},
				},
			}
		default:
			a.log.WithField(
				"resource.type", resourceRef.ResourceType,
			).Debug("invalid resource type for a task dependency, skipping")
			continue
		}

		mapKey := typeAndID{objType: resourceRef.ResourceType, id: resourceRef.ResourceID}
		if a.releaseDependency(mapKey, assignment, t.ID) {
			modified = true
		}
	}

	container := t.Spec.GetContainer()

	var secrets []*api.SecretReference
	if container != nil {
		secrets = container.Secrets
	}

	for _, secretRef := range secrets {
		secretID := secretRef.SecretID
		mapKey := typeAndID{objType: api.ResourceType_SECRET, id: secretID}
		assignment := &api.Assignment{
			Item: &api.Assignment_Secret{
				Secret: &api.Secret{ID: secretID},
			},
		}
		if a.releaseDependency(mapKey, assignment, t.ID) {
			modified = true
		}
	}

	var configs []*api.ConfigReference
	if container != nil {
		configs = container.Configs
	}

	for _, configRef := range configs {
		configID := configRef.ConfigID
		mapKey := typeAndID{objType: api.ResourceType_CONFIG, id: configID}
		assignment := &api.Assignment{
			Item: &api.Assignment_Config{
				Config: &api.Config{ID: configID},
			},
		}
		if a.releaseDependency(mapKey, assignment, t.ID) {
			modified = true
		}
	}

	return modified
}

func (a *assignmentSet) addOrUpdateTask(readTx store.ReadTx, t *api.Task) bool {
	// We only care about tasks that are ASSIGNED or higher.
	if t.Status.State < api.TaskStateAssigned {
		return false
	}

	if oldTask, exists := a.tasksMap[t.ID]; exists {
		// States ASSIGNED and below are set by the orchestrator/scheduler,
		// not the agent, so tasks in these states need to be sent to the
		// agent even if nothing else has changed.
		if equality.TasksEqualStable(oldTask, t) && t.Status.State > api.TaskStateAssigned {
			// this update should not trigger a task change for the agent
			a.tasksMap[t.ID] = t
			// If this task got updated to a final state, let's release
			// the dependencies that are being used by the task
			if t.Status.State > api.TaskStateRunning {
				// If releasing the dependencies caused us to
				// remove something from the assignment set,
				// mark one modification.
				return a.releaseTaskDependencies(t)
			}
			return false
		}
	} else if t.Status.State <= api.TaskStateRunning {
		// If this task wasn't part of the assignment set before, and it's <= RUNNING
		// add the dependencies it references to the assignment.
		// Task states > RUNNING are worker reported only, are never created in
		// a > RUNNING state.
		a.addTaskDependencies(readTx, t)
	}
	a.tasksMap[t.ID] = t
	a.changes[typeAndID{objType: api.ResourceType_TASK, id: t.ID}] = &api.AssignmentChange{
		Assignment: &api.Assignment{
			Item: &api.Assignment_Task{
				Task: t,
			},
		},
		Action: api.AssignmentChange_AssignmentActionUpdate,
	}
	return true
}

func (a *assignmentSet) removeTask(t *api.Task) bool {
	if _, exists := a.tasksMap[t.ID]; !exists {
		return false
	}

	a.changes[typeAndID{objType: api.ResourceType_TASK, id: t.ID}] = &api.AssignmentChange{
		Assignment: &api.Assignment{
			Item: &api.Assignment_Task{
				Task: &api.Task{ID: t.ID},
			},
		},
		Action: api.AssignmentChange_AssignmentActionRemove,
	}

	delete(a.tasksMap, t.ID)

	// Release the dependencies being used by this task.
	// Ignoring the return here. We will always mark this as a
	// modification, since a task is being removed.
	a.releaseTaskDependencies(t)
	return true
}

func (a *assignmentSet) message() api.AssignmentsMessage {
	var message api.AssignmentsMessage
	for _, change := range a.changes {
		message.Changes = append(message.Changes, change)
	}

	// The the set of changes is reinitialized to prepare for formation
	// of the next message.
	a.changes = make(map[typeAndID]*api.AssignmentChange)

	return message
}

// secret populates the secret value from raft store. For external secrets, the value is populated
// from the secret driver.
func (a *assignmentSet) secret(readTx store.ReadTx, task *api.Task, secretID string) (*api.Secret, error) {
	secret := store.GetSecret(readTx, secretID)
	if secret == nil {
		return nil, fmt.Errorf("secret not found")
	}
	if secret.Spec.Driver == nil {
		return secret, nil
	}
	d, err := a.dp.NewSecretDriver(secret.Spec.Driver)
	if err != nil {
		return nil, err
	}
	value, err := d.Get(&secret.Spec, task)
	if err != nil {
		return nil, err
	}
	if err := validation.ValidateSecretPayload(value); err != nil {
		return nil, err
	}
	// Assign the secret
	secret.Spec.Data = value
	return secret, nil
}
