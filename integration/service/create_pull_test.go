package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	swarmtypes "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/swarm"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// unauthorizedRegistry returns the address of a registry that rejects every
// request with an UNAUTHORIZED error code. The response deliberately carries no
// WWW-Authenticate challenge: the pull then fails at the manifest request with
// the registry's own error, which is the case this test is about. With a Bearer
// challenge the pull fails while fetching a token instead, and that error is
// reported differently.
func unauthorizedRegistry(t *testing.T, repo string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprintf(w,
			`{"errors":[{"code":"UNAUTHORIZED","message":"unauthorized to access repository: %s, action: pull"}]}`,
			repo,
		)
	}))
	t.Cleanup(srv.Close)

	// The registry is served over plain HTTP, which the daemon allows without
	// configuration only for loopback addresses; httptest binds to 127.0.0.1.
	return strings.TrimPrefix(srv.URL, "http://")
}

// TestServiceCreateUnauthorizedRegistry verifies how a task whose image cannot
// be pulled is handled: the task runs if the image is already present on the
// node, and is rejected with the registry error if it is not.
//
// A service created without registry credentials (no "--with-registry-auth")
// cannot have its digest resolved and cannot be pulled by the node, so an
// image that is already present locally is the only thing the task can run.
func TestServiceCreateUnauthorizedRegistry(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "the test registry is only reachable from the test host")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "the test uses a Linux busybox image")
	ctx := setupTest(t)

	// Test daemons are started with DOCKER_SERVICE_PREFER_OFFLINE_IMAGE=1,
	// which makes the executor skip the pull altogether; opt out of it, as the
	// pull is what is being tested here.
	d := swarm.NewSwarm(ctx, t, testEnv, daemon.WithEnvVars("DOCKER_SERVICE_PREFER_OFFLINE_IMAGE=0"))
	defer d.Stop(t)
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	t.Run("image present locally", func(t *testing.T) {
		const repo = "testing/present"
		img := unauthorizedRegistry(t, repo) + "/" + repo + ":latest"

		// Make the image available on the node under a name that resolves to
		// the unauthorized registry. It is never pushed there: the task has to
		// run from the local image store.
		d.LoadBusybox(ctx, t)
		_, err := apiClient.ImageTag(ctx, client.ImageTagOptions{Source: "busybox:latest", Target: img})
		assert.NilError(t, err)

		serviceID := swarm.CreateService(ctx, t, d,
			swarm.ServiceWithImage(img),
			swarm.ServiceWithCommand([]string{"top"}),
			swarm.ServiceWithReplicas(1),
		)
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, 1), swarm.ServicePoll)
	})

	t.Run("image missing locally", func(t *testing.T) {
		const repo = "testing/missing"
		img := unauthorizedRegistry(t, repo) + "/" + repo + ":latest"

		serviceID := swarm.CreateService(ctx, t, d,
			swarm.ServiceWithImage(img),
			swarm.ServiceWithCommand([]string{"top"}),
			swarm.ServiceWithReplicas(1),
		)
		// The pull failure is the actual cause of the task not starting, so it
		// is what the task status must report.
		poll.WaitOn(t, taskRejectedWithPullError(ctx, apiClient, serviceID, repo), swarm.ServicePoll)
	})
}

// taskRejectedWithPullError waits for a task of serviceID to be rejected, and
// checks that it reports the failure to pull repo rather than the misleading
// "No such image" from the container create that follows the pull. The wording
// of the pull error itself is left to the image store: the graphdriver reports
// the registry's "unauthorized" error, the containerd store reports a
// reference it could not resolve.
func taskRejectedWithPullError(ctx context.Context, apiClient client.TaskAPIClient, serviceID, repo string) func(poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		taskList, err := apiClient.TaskList(ctx, client.TaskListOptions{
			Filters: make(client.Filters).Add("service", serviceID),
		})
		if err != nil {
			return poll.Error(err)
		}
		for _, task := range taskList.Items {
			if task.Status.State != swarmtypes.TaskStateRejected {
				continue
			}
			switch {
			case strings.Contains(task.Status.Err, "No such image"):
				return poll.Error(fmt.Errorf("task %s reports the container create failure %q instead of the pull failure", task.ID, task.Status.Err))
			case !strings.Contains(task.Status.Err, repo):
				return poll.Error(fmt.Errorf("task %s rejected with %q, expected it to name %s", task.ID, task.Status.Err, repo))
			}
			return poll.Success()
		}
		return poll.Continue("waiting for a task of service %s to be rejected", serviceID)
	}
}
