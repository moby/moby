package dispatcher

import (
	"fmt"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/api/equality"
	"github.com/moby/swarmkit/v2/api/validation"
	"github.com/moby/swarmkit/v2/identity"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/drivers"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"github.com/sirupsen/logrus"
)

type typeAndID struct {
	id      string
	objType api.ResourceType
}

type assignmentSet struct {
	nodeID   string
	dp       *drivers.DriverProvider
	tasksMap map[string]*api.Task
	// volumesMap keeps track of the VolumePublishStatus of the given volumes.
	// this tells us both which volumes are assigned to the node, and what the
	// last known VolumePublishStatus was, so we can understand if we need to
	// send an update.
	volumesMap map[string]*api.VolumePublishStatus
	// tasksUsingDependency tracks both tasks and volumes using a given
	// dependency. this works because the ID generated for swarm comes from a
	// large enough space that it is reliably astronomically unlikely that IDs
	// will ever collide.
	tasksUsingDependency map[typeAndID]map[string]struct{}
	changes              map[typeAndID]*api.AssignmentChange
	log                  *logrus.Entry
}

func newAssignmentSet(nodeID string, log *logrus.Entry, dp *drivers.DriverProvider) *assignmentSet {
	return &assignmentSet{
		nodeID:               nodeID,
		dp:                   dp,
		changes:              make(map[typeAndID]*api.AssignmentChange),
		tasksMap:             make(map[string]*api.Task),
		volumesMap:           make(map[string]*api.VolumePublishStatus),
		tasksUsingDependency: make(map[typeAndID]map[string]struct{}),
		log:                  log,
	}
}

func assignSecret(a *assignmentSet, readTx store.ReadTx, mapKey typeAndID, t *api.Task) {
	if _, exists := a.tasksUsingDependency[mapKey]; !exists {
		a.tasksUsingDependency[mapKey] = make(map[string]struct{})
	}
	secret, doNotReuse, err := a.secret(readTx, t, mapKey.id)
	if err != nil {
		a.log.WithFields(log.Fields{
			"resource.type": "secret",
			"secret.id":     mapKey.id,
			"error":         err,
		}).Debug("failed to fetch secret")
		return
	}
	// If the secret should not be reused for other tasks, give it a unique ID
	// for the task to allow different values for different tasks.
	if doNotReuse {
		// Give the secret a new ID and mark it as internal
		originalSecretID := secret.Id
		taskSpecificID := identity.CombineTwoIDs(originalSecretID, t.Id)
		secret.Id = taskSpecificID
		secret.Internal = true
		// Create a new mapKey with the new ID and insert it into the
		// dependencies map for the task.  This will make the changes map
		// contain an entry with the new ID rather than the original one.
		mapKey = typeAndID{objType: mapKey.objType, id: secret.Id}
		a.tasksUsingDependency[mapKey] = make(map[string]struct{})
		a.tasksUsingDependency[mapKey][t.Id] = struct{}{}
	}
	a.changes[mapKey] = &api.AssignmentChange{
		Assignment: &api.Assignment{
			Item: &api.Assignment_Secret{
				Secret: secret,
			},
		},
		Action: api.AssignmentChange_UPDATE,
	}
}

func assignConfig(a *assignmentSet, readTx store.ReadTx, mapKey typeAndID) {
	a.tasksUsingDependency[mapKey] = make(map[string]struct{})
	config := store.GetConfig(readTx, mapKey.id)
	if config == nil {
		a.log.WithFields(log.Fields{
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
		Action: api.AssignmentChange_UPDATE,
	}
}

func (a *assignmentSet) addTaskDependencies(readTx store.ReadTx, t *api.Task) {
	// first, we go through all ResourceReferences, which give us the necessary
	// information about which secrets and configs are in use.
	for _, resourceRef := range t.Spec.GetResourceReferences() {
		mapKey := typeAndID{objType: resourceRef.ResourceType, id: resourceRef.ResourceId}
		// if there are no tasks using this dependency yet, then we can assign
		// it.
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
		// otherwise, we don't need to add a new assignment. we just need to
		// track the fact that another task is now using this dependency.
		a.tasksUsingDependency[mapKey][t.Id] = struct{}{}
	}

	var secrets []*api.SecretReference
	container := t.Spec.GetContainer()
	if container != nil {
		secrets = container.Secrets
	}

	for _, secretRef := range secrets {
		secretID := secretRef.SecretId
		mapKey := typeAndID{objType: api.ResourceType_SECRET, id: secretID}

		// This checks for the presence of each task in the dependency map for the
		// secret. This is currently only done for secrets since the other types of
		// dependencies do not support driver plugins. Arguably, the same task would
		// not have the same secret as a dependency more than once, but this check
		// makes sure the task only gets the secret assigned once.
		if _, exists := a.tasksUsingDependency[mapKey][t.Id]; !exists {
			assignSecret(a, readTx, mapKey, t)
		}
		a.tasksUsingDependency[mapKey][t.Id] = struct{}{}
	}

	var configs []*api.ConfigReference
	if container != nil {
		configs = container.Configs
	}
	for _, configRef := range configs {
		configID := configRef.ConfigId
		mapKey := typeAndID{objType: api.ResourceType_CONFIG, id: configID}

		if len(a.tasksUsingDependency[mapKey]) == 0 {
			assignConfig(a, readTx, mapKey)
		}
		a.tasksUsingDependency[mapKey][t.Id] = struct{}{}
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
		Action:     api.AssignmentChange_REMOVE,
	}
	return true
}

// releaseTaskDependencies needs a store transaction because volumes have
// associated Secrets which need to be released.
func (a *assignmentSet) releaseTaskDependencies(_ store.ReadTx, t *api.Task) bool {
	var modified bool

	for _, resourceRef := range t.Spec.GetResourceReferences() {
		var assignment *api.Assignment
		switch resourceRef.ResourceType {
		case api.ResourceType_SECRET:
			assignment = &api.Assignment{
				Item: &api.Assignment_Secret{
					Secret: &api.Secret{Id: resourceRef.ResourceId},
				},
			}
		case api.ResourceType_CONFIG:
			assignment = &api.Assignment{
				Item: &api.Assignment_Config{
					Config: &api.Config{Id: resourceRef.ResourceId},
				},
			}
		default:
			a.log.WithField(
				"resource.type", resourceRef.ResourceType,
			).Debug("invalid resource type for a task dependency, skipping")
			continue
		}

		mapKey := typeAndID{objType: resourceRef.ResourceType, id: resourceRef.ResourceId}
		if a.releaseDependency(mapKey, assignment, t.Id) {
			modified = true
		}
	}

	container := t.Spec.GetContainer()

	var secrets []*api.SecretReference
	if container != nil {
		secrets = container.Secrets
	}

	for _, secretRef := range secrets {
		secretID := secretRef.SecretId
		mapKey := typeAndID{objType: api.ResourceType_SECRET, id: secretID}
		assignment := &api.Assignment{
			Item: &api.Assignment_Secret{
				Secret: &api.Secret{Id: secretID},
			},
		}
		if a.releaseDependency(mapKey, assignment, t.Id) {
			modified = true
		}
	}

	var configs []*api.ConfigReference
	if container != nil {
		configs = container.Configs
	}

	for _, configRef := range configs {
		configID := configRef.ConfigId
		mapKey := typeAndID{objType: api.ResourceType_CONFIG, id: configID}
		assignment := &api.Assignment{
			Item: &api.Assignment_Config{
				Config: &api.Config{Id: configID},
			},
		}
		if a.releaseDependency(mapKey, assignment, t.Id) {
			modified = true
		}
	}

	return modified
}

func (a *assignmentSet) addOrUpdateTask(readTx store.ReadTx, t *api.Task) bool {
	// We only care about tasks that are ASSIGNED or higher.
	if t.Status.GetState() < api.TaskState_ASSIGNED {
		return false
	}

	if oldTask, exists := a.tasksMap[t.Id]; exists {
		// States ASSIGNED and below are set by the orchestrator/scheduler,
		// not the agent, so tasks in these states need to be sent to the
		// agent even if nothing else has changed.
		if equality.TasksEqualStable(oldTask, t) && t.Status.GetState() > api.TaskState_ASSIGNED {
			// this update should not trigger a task change for the agent
			a.tasksMap[t.Id] = t
			// If this task got updated to a final state, let's release
			// the dependencies that are being used by the task
			if t.Status.GetState() > api.TaskState_RUNNING {
				// If releasing the dependencies caused us to
				// remove something from the assignment set,
				// mark one modification.
				return a.releaseTaskDependencies(readTx, t)
			}
			return false
		}
	} else if t.Status.GetState() <= api.TaskState_RUNNING {
		// If this task wasn't part of the assignment set before, and it's <= RUNNING
		// add the dependencies it references to the assignment.
		// Task states > RUNNING are worker reported only, are never created in
		// a > RUNNING state.
		a.addTaskDependencies(readTx, t)
	}
	a.tasksMap[t.Id] = t
	a.changes[typeAndID{objType: api.ResourceType_TASK, id: t.Id}] = &api.AssignmentChange{
		Assignment: &api.Assignment{
			Item: &api.Assignment_Task{
				Task: t,
			},
		},
		Action: api.AssignmentChange_UPDATE,
	}
	return true
}

// addOrUpdateVolume tracks a Volume assigned to a node.
func (a *assignmentSet) addOrUpdateVolume(readTx store.ReadTx, v *api.Volume) bool {
	var publishStatus *api.VolumePublishStatus
	for _, status := range v.PublishStatus {
		if status.NodeId == a.nodeID {
			publishStatus = status
			break
		}
	}

	// if there is no publishStatus for this Volume on this Node, or if the
	// Volume has not yet been published to this node, then we do not need to
	// track this assignment.
	if publishStatus == nil || publishStatus.State < api.VolumePublishStatus_PUBLISHED {
		return false
	}

	// check if we are already tracking this volume, and what its old status
	// is. if the states are identical, then we don't have any update to make.
	if oldStatus, ok := a.volumesMap[v.Id]; ok && oldStatus.State == publishStatus.State {
		return false
	}

	// if the volume has already been confirmed as unpublished, we can stop
	// tracking it and remove its dependencies.
	if publishStatus.State > api.VolumePublishStatus_PENDING_NODE_UNPUBLISH {
		return a.removeVolume(readTx, v)
	}

	for _, secret := range v.Spec.GetSecrets() {
		mapKey := typeAndID{objType: api.ResourceType_SECRET, id: secret.Secret}
		if len(a.tasksUsingDependency[mapKey]) == 0 {
			// we can call assignSecret with task being nil, but it does mean
			// that any secret that uses a driver will not work. we'll call
			// that a limitation of volumes for now.
			assignSecret(a, readTx, mapKey, nil)
		}
		a.tasksUsingDependency[mapKey][v.Id] = struct{}{}
	}

	// volumes are sent to nodes as VolumeAssignments. This is because a node
	// needs node-specific information (the PublishContext from
	// ControllerPublishVolume).
	assignment := &api.VolumeAssignment{
		Id:             v.Id,
		VolumeId:       v.VolumeInfo.VolumeId,
		Driver:         v.Spec.GetDriver(),
		VolumeContext:  v.VolumeInfo.VolumeContext,
		PublishContext: publishStatus.PublishContext,
		AccessMode:     v.Spec.GetAccessMode(),
		Secrets:        v.Spec.GetSecrets(),
	}

	volumeKey := typeAndID{objType: api.ResourceType_VOLUME, id: v.Id}
	// assignmentChange is the whole assignment without the action, which we
	// will set next
	assignmentChange := &api.AssignmentChange{
		Assignment: &api.Assignment{
			Item: &api.Assignment_Volume{
				Volume: assignment,
			},
		},
	}

	// if we're in state PENDING_NODE_UNPUBLISH, we actually need to send a
	// remove message. we do this every time, even if the node never got the
	// first add assignment. This is because the node might not know that it
	// has a volume published; for example, the node may be restarting, and
	// the in-memory store does not have knowledge of the volume.
	if publishStatus.State == api.VolumePublishStatus_PENDING_NODE_UNPUBLISH {
		assignmentChange.Action = api.AssignmentChange_REMOVE
	} else {
		assignmentChange.Action = api.AssignmentChange_UPDATE
	}
	a.changes[volumeKey] = assignmentChange
	a.volumesMap[v.Id] = publishStatus
	return true
}

func (a *assignmentSet) removeVolume(_ store.ReadTx, v *api.Volume) bool {
	if _, exists := a.volumesMap[v.Id]; !exists {
		return false
	}

	modified := false

	// if the volume does exists, we can release its secrets
	for _, secret := range v.Spec.GetSecrets() {
		mapKey := typeAndID{objType: api.ResourceType_SECRET, id: secret.Secret}
		assignment := &api.Assignment{
			Item: &api.Assignment_Secret{
				Secret: &api.Secret{Id: secret.Secret},
			},
		}
		if a.releaseDependency(mapKey, assignment, v.Id) {
			modified = true
		}
	}

	// we don't need to add a removal message. the removal of the
	// VolumeAssignment will have already happened.
	delete(a.volumesMap, v.Id)

	return modified
}

func (a *assignmentSet) removeTask(readTx store.ReadTx, t *api.Task) bool {
	if _, exists := a.tasksMap[t.Id]; !exists {
		return false
	}

	a.changes[typeAndID{objType: api.ResourceType_TASK, id: t.Id}] = &api.AssignmentChange{
		Assignment: &api.Assignment{
			Item: &api.Assignment_Task{
				Task: &api.Task{Id: t.Id},
			},
		},
		Action: api.AssignmentChange_REMOVE,
	}

	delete(a.tasksMap, t.Id)

	// Release the dependencies being used by this task.
	// Ignoring the return here. We will always mark this as a
	// modification, since a task is being removed.
	a.releaseTaskDependencies(readTx, t)
	return true
}

func (a *assignmentSet) message() *api.AssignmentsMessage {
	message := &api.AssignmentsMessage{}
	for _, change := range a.changes {
		message.Changes = append(message.Changes, change)
	}

	// The the set of changes is reinitialized to prepare for formation
	// of the next message.
	a.changes = make(map[typeAndID]*api.AssignmentChange)

	return message
}

// secret populates the secret value from raft store. For external secrets, the value is populated
// from the secret driver. The function returns: a secret object; an indication of whether the value
// is to be reused across tasks; and an error if the secret is not found in the store, if the secret
// driver responds with one or if the payload does not pass validation.
func (a *assignmentSet) secret(readTx store.ReadTx, task *api.Task, secretID string) (*api.Secret, bool, error) {
	secret := store.GetSecret(readTx, secretID)
	if secret == nil {
		return nil, false, fmt.Errorf("secret not found")
	}
	if secret.Spec.GetDriver() == nil {
		return secret, false, nil
	}
	d, err := a.dp.NewSecretDriver(secret.Spec.GetDriver())
	if err != nil {
		return nil, false, err
	}
	value, doNotReuse, err := d.Get(secret.Spec, task)
	if err != nil {
		return nil, false, err
	}
	if err := validation.ValidateSecretPayload(value); err != nil {
		return nil, false, err
	}
	// Assign the secret
	secret.Spec.Data = value
	return secret, doNotReuse, nil
}
