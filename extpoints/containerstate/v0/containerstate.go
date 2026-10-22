// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate go tool mobyextgen

// Package containerstatev0 defines a daemon-scoped extension point for
// watching container state.
package containerstatev0

import (
	"context"

	"github.com/moby/extensions"
)

// ContainerStateWatcher watches container state.
//
// Watch resolves the container by ID, name, or unique ID prefix.
// It returns the container's state once its status is one of the request's
// statuses, and blocks until then otherwise.
// It returns immediately if the request lists no statuses or the status
// already matches.
// A status that is replaced before Watch observes it can be skipped.
//
// A watch blocked on a container that is removed returns its final state
// with status "removed", even if the request does not list it; a later
// Watch returns NotFound.
// An unknown status returns InvalidArgument.
// Watch returns Unavailable until daemon startup completes.
type ContainerStateWatcher interface {
	// Watch returns the container's state once its status is one of
	// req.Statuses.
	Watch(ctx context.Context, req *WatchRequest) (*WatchReply, error)
}

var Point = extensions.DefineSinglePoint[ContainerStateWatcher]("org.mobyproject.extension.containerstate.v0")

// Container statuses used in [WatchRequest.Statuses] and
// [ContainerState.Status].
const (
	StatusCreated  = "created"
	StatusRunning  = "running"
	StatusPaused   = "paused"
	StatusRemoving = "removing"
	StatusExited   = "exited"
	StatusDead     = "dead"
	StatusRemoved  = "removed"
	// StatusRestarting is the status between restart-policy attempts.
	StatusRestarting = "restarting"
)

// Health statuses used in [Health.Status].
const (
	HealthStarting  = "starting"
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
)

// WatchRequest identifies the container and the statuses to watch for.
type WatchRequest struct {
	ContainerID string `pb:"1"`
	// Statuses lists the [ContainerState.Status] values to return; empty
	// returns any status.
	Statuses []string `pb:"2"`
}

// WatchReply contains the container's state.
type WatchReply struct {
	// Fields 1 and 3 are reserved until the daemon tracks status changes.
	//
	// // ChangedAt is when the daemon observed the current status, in Unix
	// // nanoseconds, or zero if it observed no status change since it started.
	// ChangedAt int64 `pb:"1"`
	//
	// // Previous is the container's state just before its status last
	// // changed, or nil if the daemon observed no status change since it
	// // started.
	// // For a removed container, it is the state at removal, usually with
	// // status [StatusRemoving], and keeps the exit error.
	// Previous *ContainerState `pb:"3"`

	// Current is the container's current state.
	Current *ContainerState `pb:"2"`
}

// ContainerState is a container's status, health, and most recent exit.
type ContainerState struct {
	// Status is one of the Status constants, such as [StatusRunning].
	// A container is [StatusRestarting], not [StatusExited], between
	// restart-policy attempts.
	Status string `pb:"1"`

	// Health is nil if the container has no healthcheck.
	Health *Health `pb:"2"`

	// Exit is the container's most recent exit, or nil while it is running
	// or before it first exits.
	Exit *Exit `pb:"3"`
}

// Health is a container's health status.
type Health struct {
	// Status is one of the Health constants, such as [HealthHealthy].
	Status string `pb:"1"`
}

// Exit describes a container exit.
type Exit struct {
	// ExitCode is the container's recorded exit code.
	ExitCode int64 `pb:"1"`
	// Error is the container's recorded error message, if any.
	// It is empty in a removed container's current state, because removal
	// clears it.
	Error string `pb:"2"`
	// OOMKilled reports whether the container's process was killed because
	// it ran out of memory.
	OOMKilled bool `pb:"3"`
}
