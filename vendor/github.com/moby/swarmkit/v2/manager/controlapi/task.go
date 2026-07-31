package controlapi

import (
	"context"
	"slices"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/api/naming"
	"github.com/moby/swarmkit/v2/manager/orchestrator"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetTask returns a Task given a TaskID.
// - Returns `InvalidArgument` if TaskID is not provided.
// - Returns `NotFound` if the Task is not found.
func (s *Server) GetTask(_ context.Context, request *api.GetTaskRequest) (*api.GetTaskResponse, error) {
	if request.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, errInvalidArgument.Error())
	}

	var task *api.Task
	s.store.View(func(tx store.ReadTx) {
		task = store.GetTask(tx, request.TaskId)
	})
	if task == nil {
		return nil, status.Errorf(codes.NotFound, "task %s not found", request.TaskId)
	}
	return &api.GetTaskResponse{
		Task: task,
	}, nil
}

// RemoveTask removes a Task referenced by TaskID.
// - Returns `InvalidArgument` if TaskID is not provided.
// - Returns `NotFound` if the Task is not found.
// - Returns an error if the deletion fails.
func (s *Server) RemoveTask(_ context.Context, request *api.RemoveTaskRequest) (*api.RemoveTaskResponse, error) {
	if request.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, errInvalidArgument.Error())
	}

	err := s.store.Update(func(tx store.Tx) error {
		return store.DeleteTask(tx, request.TaskId)
	})
	if err != nil {
		if err == store.ErrNotExist {
			return nil, status.Errorf(codes.NotFound, "task %s not found", request.TaskId)
		}
		return nil, err
	}
	return &api.RemoveTaskResponse{}, nil
}

func filterTasks(candidates []*api.Task, filters ...func(*api.Task) bool) []*api.Task {
	result := []*api.Task{}

	for _, c := range candidates {
		match := true
		for _, f := range filters {
			if !f(c) {
				match = false
				break
			}
		}
		if match {
			result = append(result, c)
		}
	}

	return result
}

// ListTasks returns a list of all tasks.
func (s *Server) ListTasks(_ context.Context, request *api.ListTasksRequest) (*api.ListTasksResponse, error) {
	var (
		tasks []*api.Task
		err   error
	)

	s.store.View(func(tx store.ReadTx) {
		switch {
		case request.Filters != nil && len(request.Filters.Names) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByName, request.Filters.Names))
		case request.Filters != nil && len(request.Filters.NamePrefixes) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByNamePrefix, request.Filters.NamePrefixes))
		case request.Filters != nil && len(request.Filters.IdPrefixes) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByIDPrefix, request.Filters.IdPrefixes))
		case request.Filters != nil && len(request.Filters.ServiceIds) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByServiceID, request.Filters.ServiceIds))
		case request.Filters != nil && len(request.Filters.Runtimes) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByRuntime, request.Filters.Runtimes))
		case request.Filters != nil && len(request.Filters.NodeIds) > 0:
			tasks, err = store.FindTasks(tx, buildFilters(store.ByNodeID, request.Filters.NodeIds))
		case request.Filters != nil && len(request.Filters.DesiredStates) > 0:
			filters := make([]store.By, 0, len(request.Filters.DesiredStates))
			for _, v := range request.Filters.DesiredStates {
				filters = append(filters, store.ByDesiredState(v))
			}
			tasks, err = store.FindTasks(tx, store.Or(filters...))
		default:
			tasks, err = store.FindTasks(tx, store.All)
		}

		if err != nil || request.Filters == nil {
			return
		}

		tasks = filterTasks(tasks,
			func(e *api.Task) bool {
				return filterContains(naming.Task(e), request.Filters.Names)
			},
			func(e *api.Task) bool {
				return filterContainsPrefix(naming.Task(e), request.Filters.NamePrefixes)
			},
			func(e *api.Task) bool {
				return filterContainsPrefix(e.Id, request.Filters.IdPrefixes)
			},
			func(e *api.Task) bool {
				return filterMatchLabels(e.GetServiceAnnotations().GetLabels(), request.Filters.Labels)
			},
			func(e *api.Task) bool {
				return filterContains(e.ServiceId, request.Filters.ServiceIds)
			},
			func(e *api.Task) bool {
				return filterContains(e.NodeId, request.Filters.NodeIds)
			},
			func(e *api.Task) bool {
				if len(request.Filters.Runtimes) == 0 {
					return true
				}
				r, err := naming.Runtime(e.Spec)
				if err != nil {
					return false
				}
				return filterContains(r, request.Filters.Runtimes)
			},
			func(e *api.Task) bool {
				ds := request.Filters.DesiredStates
				return len(ds) == 0 || slices.Contains(ds, e.DesiredState)
			},
			func(e *api.Task) bool {
				if !request.Filters.UpToDate {
					return true
				}

				service := store.GetService(tx, e.ServiceId)
				if service == nil {
					return false
				}

				n := store.GetNode(tx, e.NodeId)
				return !orchestrator.IsTaskDirty(service, e, n)
			},
		)
	})

	if err != nil {
		return nil, err
	}

	return &api.ListTasksResponse{
		Tasks: tasks,
	}, nil
}
