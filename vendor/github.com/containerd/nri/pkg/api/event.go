/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package api

import (
	"fmt"
	"strings"
)

const (
	// ValidEvents is the event mask of all valid events.
	ValidEvents = EventMask((1 << (Event_LAST - 1)) - 1)
)

// nolint
type (
	// Define *Request/*Response type aliases for *Event/Empty pairs.

	StateChangeResponse          = Empty
	RunPodSandboxRequest         = StateChangeEvent
	RunPodSandboxResponse        = Empty
	StopPodSandboxRequest        = StateChangeEvent
	StopPodSandboxResponse       = Empty
	RemovePodSandboxRequest      = StateChangeEvent
	RemovePodSandboxResponse     = Empty
	PostUpdatePodSandboxRequest  = StateChangeEvent
	PostUpdatePodSandboxResponse = Empty
	StartContainerRequest        = StateChangeEvent
	StartContainerResponse       = Empty
	RemoveContainerRequest       = StateChangeEvent
	RemoveContainerResponse      = Empty
	PostCreateContainerRequest   = StateChangeEvent
	PostCreateContainerResponse  = Empty
	PostStartContainerRequest    = StateChangeEvent
	PostStartContainerResponse   = Empty
	PostUpdateContainerRequest   = StateChangeEvent
	PostUpdateContainerResponse  = Empty

	ShutdownRequest  = Empty
	ShutdownResponse = Empty
)

// EventMask corresponds to a set of enumerated Events.
type EventMask int32

// ParseEventMask parses a string representation into an EventMask.
func ParseEventMask(events ...string) (EventMask, error) {
	var mask EventMask

	bits := map[string]Event{
		"runpodsandbox":               Event_RUN_POD_SANDBOX,
		"updatepodsandbox":            Event_UPDATE_POD_SANDBOX,
		"postupdatepodsandbox":        Event_POST_UPDATE_POD_SANDBOX,
		"stoppodsandbox":              Event_STOP_POD_SANDBOX,
		"removepodsandbox":            Event_REMOVE_POD_SANDBOX,
		"createcontainer":             Event_CREATE_CONTAINER,
		"postcreatecontainer":         Event_POST_CREATE_CONTAINER,
		"startcontainer":              Event_START_CONTAINER,
		"poststartcontainer":          Event_POST_START_CONTAINER,
		"updatecontainer":             Event_UPDATE_CONTAINER,
		"postupdatecontainer":         Event_POST_UPDATE_CONTAINER,
		"stopcontainer":               Event_STOP_CONTAINER,
		"removecontainer":             Event_REMOVE_CONTAINER,
		"validatecontaineradjustment": Event_VALIDATE_CONTAINER_ADJUSTMENT,
	}

	for _, event := range events {
		lcEvents := strings.ToLower(event)
		for _, name := range strings.Split(lcEvents, ",") {
			switch name {
			case "all":
				mask |= ValidEvents
				continue
			case "pod", "podsandbox":
				for name, bit := range bits {
					if strings.Contains(name, "pod") {
						mask.Set(bit)
					}
				}
				continue
			case "container":
				for name, bit := range bits {
					if strings.Contains(name, "container") {
						mask.Set(bit)
					}
				}
				continue
			}

			bit, ok := bits[strings.TrimSpace(name)]
			if !ok {
				return 0, fmt.Errorf("unknown event %q", name)
			}
			mask.Set(bit)
		}
	}

	return mask, nil
}

// MustParseEventMask parses the given events, panic()ing on errors.
func MustParseEventMask(events ...string) EventMask {
	mask, err := ParseEventMask(events...)
	if err != nil {
		panic(fmt.Sprintf("failed to parse events %s", strings.Join(events, " ")))
	}
	return mask
}

// PrettyString returns a human-readable string representation of an EventMask.
func (m *EventMask) PrettyString() string {
	names := map[Event]string{
		Event_RUN_POD_SANDBOX:               "RunPodSandbox",
		Event_UPDATE_POD_SANDBOX:            "UpdatePodSandbox",
		Event_POST_UPDATE_POD_SANDBOX:       "PostUpdatePodSandbox",
		Event_STOP_POD_SANDBOX:              "StopPodSandbox",
		Event_REMOVE_POD_SANDBOX:            "RemovePodSandbox",
		Event_CREATE_CONTAINER:              "CreateContainer",
		Event_POST_CREATE_CONTAINER:         "PostCreateContainer",
		Event_START_CONTAINER:               "StartContainer",
		Event_POST_START_CONTAINER:          "PostStartContainer",
		Event_UPDATE_CONTAINER:              "UpdateContainer",
		Event_POST_UPDATE_CONTAINER:         "PostUpdateContainer",
		Event_STOP_CONTAINER:                "StopContainer",
		Event_REMOVE_CONTAINER:              "RemoveContainer",
		Event_VALIDATE_CONTAINER_ADJUSTMENT: "ValidateContainerAdjustment",
	}

	mask := *m
	events, sep := "", ""

	for bit := Event_UNKNOWN + 1; bit <= Event_LAST; bit++ {
		if mask.IsSet(bit) {
			events += sep + names[bit]
			sep = ","
			mask.Clear(bit)
		}
	}

	if mask != 0 {
		events += sep + fmt.Sprintf("unknown(0x%x)", mask)
	}

	return events
}

// Set sets the given Events in the mask.
func (m *EventMask) Set(events ...Event) *EventMask {
	for _, e := range events {
		*m |= (1 << (e - 1))
	}
	return m
}

// Clear clears the given Events in the mask.
func (m *EventMask) Clear(events ...Event) *EventMask {
	for _, e := range events {
		*m &^= (1 << (e - 1))
	}
	return m
}

// IsSet check if the given Event is set in the mask.
func (m *EventMask) IsSet(e Event) bool {
	return *m&(1<<(e-1)) != 0
}
