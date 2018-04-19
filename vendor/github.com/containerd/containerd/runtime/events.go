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

package runtime

const (
	// TaskCreateEventTopic for task create
	TaskCreateEventTopic = "/tasks/create"
	// TaskStartEventTopic for task start
	TaskStartEventTopic = "/tasks/start"
	// TaskOOMEventTopic for task oom
	TaskOOMEventTopic = "/tasks/oom"
	// TaskExitEventTopic for task exit
	TaskExitEventTopic = "/tasks/exit"
	// TaskDeleteEventTopic for task delete
	TaskDeleteEventTopic = "/tasks/delete"
	// TaskExecAddedEventTopic for task exec create
	TaskExecAddedEventTopic = "/tasks/exec-added"
	// TaskExecStartedEventTopic for task exec start
	TaskExecStartedEventTopic = "/tasks/exec-started"
	// TaskPausedEventTopic for task pause
	TaskPausedEventTopic = "/tasks/paused"
	// TaskResumedEventTopic for task resume
	TaskResumedEventTopic = "/tasks/resumed"
	// TaskCheckpointedEventTopic for task checkpoint
	TaskCheckpointedEventTopic = "/tasks/checkpointed"
	// TaskUnknownTopic for unknown task events
	TaskUnknownTopic = "/tasks/?"
)
