package swarm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/internal/test"
	"github.com/pkg/errors"
	// Import builders to get the builder function as package function
	. "github.com/docker/docker/cli/internal/test/builders"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/golden"
)

func TestSwarmUpdateErrors(t *testing.T) {
	testCases := []struct {
		name                  string
		args                  []string
		flags                 map[string]string
		swarmInspectFunc      func() (swarm.Swarm, error)
		swarmUpdateFunc       func(swarm swarm.Spec, flags swarm.UpdateFlags) error
		swarmGetUnlockKeyFunc func() (types.SwarmUnlockKeyResponse, error)
		expectedError         string
	}{
		{
			name:          "too-many-args",
			args:          []string{"foo"},
			expectedError: "accepts no argument(s)",
		},
		{
			name: "swarm-inspect-error",
			flags: map[string]string{
				flagTaskHistoryLimit: "10",
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return swarm.Swarm{}, errors.Errorf("error inspecting the swarm")
			},
			expectedError: "error inspecting the swarm",
		},
		{
			name: "swarm-update-error",
			flags: map[string]string{
				flagTaskHistoryLimit: "10",
			},
			swarmUpdateFunc: func(swarm swarm.Spec, flags swarm.UpdateFlags) error {
				return errors.Errorf("error updating the swarm")
			},
			expectedError: "error updating the swarm",
		},
		{
			name: "swarm-unlockkey-error",
			flags: map[string]string{
				flagAutolock: "true",
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return *Swarm(), nil
			},
			swarmGetUnlockKeyFunc: func() (types.SwarmUnlockKeyResponse, error) {
				return types.SwarmUnlockKeyResponse{}, errors.Errorf("error getting unlock key")
			},
			expectedError: "error getting unlock key",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newUpdateCommand(
			test.NewFakeCli(&fakeClient{
				swarmInspectFunc:      tc.swarmInspectFunc,
				swarmUpdateFunc:       tc.swarmUpdateFunc,
				swarmGetUnlockKeyFunc: tc.swarmGetUnlockKeyFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestSwarmUpdate(t *testing.T) {
	testCases := []struct {
		name                  string
		args                  []string
		flags                 map[string]string
		swarmInspectFunc      func() (swarm.Swarm, error)
		swarmUpdateFunc       func(swarm swarm.Spec, flags swarm.UpdateFlags) error
		swarmGetUnlockKeyFunc func() (types.SwarmUnlockKeyResponse, error)
	}{
		{
			name: "noargs",
		},
		{
			name: "all-flags-quiet",
			flags: map[string]string{
				flagTaskHistoryLimit:    "10",
				flagDispatcherHeartbeat: "10s",
				flagCertExpiry:          "20s",
				flagExternalCA:          "protocol=cfssl,url=https://example.com.",
				flagMaxSnapshots:        "10",
				flagSnapshotInterval:    "100",
				flagAutolock:            "true",
				flagQuiet:               "true",
			},
			swarmUpdateFunc: func(swarm swarm.Spec, flags swarm.UpdateFlags) error {
				if *swarm.Orchestration.TaskHistoryRetentionLimit != 10 {
					return errors.Errorf("historyLimit not correctly set")
				}
				heartbeatDuration, err := time.ParseDuration("10s")
				if err != nil {
					return err
				}
				if swarm.Dispatcher.HeartbeatPeriod != heartbeatDuration {
					return errors.Errorf("heartbeatPeriodLimit not correctly set")
				}
				certExpiryDuration, err := time.ParseDuration("20s")
				if err != nil {
					return err
				}
				if swarm.CAConfig.NodeCertExpiry != certExpiryDuration {
					return errors.Errorf("certExpiry not correctly set")
				}
				if len(swarm.CAConfig.ExternalCAs) != 1 {
					return errors.Errorf("externalCA not correctly set")
				}
				if *swarm.Raft.KeepOldSnapshots != 10 {
					return errors.Errorf("keepOldSnapshots not correctly set")
				}
				if swarm.Raft.SnapshotInterval != 100 {
					return errors.Errorf("snapshotInterval not correctly set")
				}
				if !swarm.EncryptionConfig.AutoLockManagers {
					return errors.Errorf("autolock not correctly set")
				}
				return nil
			},
		},
		{
			name: "autolock-unlock-key",
			flags: map[string]string{
				flagTaskHistoryLimit: "10",
				flagAutolock:         "true",
			},
			swarmUpdateFunc: func(swarm swarm.Spec, flags swarm.UpdateFlags) error {
				if *swarm.Orchestration.TaskHistoryRetentionLimit != 10 {
					return errors.Errorf("historyLimit not correctly set")
				}
				return nil
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return *Swarm(), nil
			},
			swarmGetUnlockKeyFunc: func() (types.SwarmUnlockKeyResponse, error) {
				return types.SwarmUnlockKeyResponse{
					UnlockKey: "unlock-key",
				}, nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newUpdateCommand(
			test.NewFakeCli(&fakeClient{
				swarmInspectFunc:      tc.swarmInspectFunc,
				swarmUpdateFunc:       tc.swarmUpdateFunc,
				swarmGetUnlockKeyFunc: tc.swarmGetUnlockKeyFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		cmd.SetOutput(buf)
		assert.NilError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("update-%s.golden", tc.name))
		assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
	}
}
