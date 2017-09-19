// +build !windows

package mountpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/internal/test/environment"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/volume/mountpoint"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/require"
)

const (
	testMountPointPlugin = "mountpointplugin"
	mountFailMessage     = "mount source path contains 'secret'"
)

var server [6]*httptest.Server
var d *daemon.Daemon
var mountPointController [6]*testMountPointController
var events []string

type testMountPointController struct {
	propertiesRes mountpoint.PropertiesResponse // propertiesRes holds the plugin response to properties requests
	attachRes     mountpoint.AttachResponse     // attachRes holds the plugin response to attach requests
	detachRes     mountpoint.DetachResponse     // detachRes holds the plugin response to detach requests
	attachCnt     int                           // attachCnt counts the number of attach requests received
	attachMounts  [][]*mountpoint.MountPoint    // attachMounts is a stack of mount point sets requested for attachment
}

func emptyMountPointController() *testMountPointController {
	return &testMountPointController{
		attachRes: mountpoint.AttachResponse{
			Success: true,
		},
		detachRes: mountpoint.DetachResponse{
			Success: true,
		},
		attachCnt:    0,
		attachMounts: [][]*mountpoint.MountPoint{},
	}
}

func mountPointControllerForVBindMountOnly() (ctrl *testMountPointController) {
	typeBind := mountpoint.TypeBind
	ctrl = emptyMountPointController()

	// matches -v /host:/container
	ctrl.propertiesRes = mountpoint.PropertiesResponse{
		Success: true,
		Patterns: []mountpoint.Pattern{
			{Type: &typeBind},
		},
	}
	return
}

func mountPointControllerForVBindMountAndAllLocalVolumes() (ctrl *testMountPointController) {
	typeBind := mountpoint.TypeBind
	typeVolume := mountpoint.TypeVolume
	ctrl = emptyMountPointController()

	// matches -v /host:/container AND all local volume mounts
	ctrl.propertiesRes = mountpoint.PropertiesResponse{
		Success: true,
		Patterns: []mountpoint.Pattern{
			{Type: &typeBind},
			{
				Type: &typeVolume,
				Volume: mountpoint.VolumePattern{
					Driver: []mountpoint.StringPattern{{Exactly: "local"}},
				},
			},
		},
	}
	return
}

func mountPointControllerForLocalVolumeBindMounts() (ctrl *testMountPointController) {
	typeVolume := mountpoint.TypeVolume
	ctrl = emptyMountPointController()

	// matches local volume bind mounts (but not -v /container mounts)
	ctrl.propertiesRes = mountpoint.PropertiesResponse{
		Success: true,
		Patterns: []mountpoint.Pattern{
			{
				Type: &typeVolume,
				Volume: mountpoint.VolumePattern{
					Driver: []mountpoint.StringPattern{{Exactly: "local"}},
					Options: []mountpoint.StringMapPattern{{
						Exists: []mountpoint.StringMapKeyValuePattern{{
							Key:   mountpoint.StringPattern{Exactly: "o"},
							Value: mountpoint.StringPattern{Contains: "bind"},
						}},
					}},
				},
			},
		},
	}
	return
}

func mountPointControllerForAnonymousBindMounts() (ctrl *testMountPointController) {
	typeVolume := mountpoint.TypeVolume
	ctrl = emptyMountPointController()

	// matches -v /container
	ctrl.propertiesRes = mountpoint.PropertiesResponse{
		Success: true,
		Patterns: []mountpoint.Pattern{
			{
				Type: &typeVolume,
				Volume: mountpoint.VolumePattern{
					Driver: []mountpoint.StringPattern{{Exactly: "local"}},
					Options: []mountpoint.StringMapPattern{{
						Not: true,
						Exists: []mountpoint.StringMapKeyValuePattern{
							{Key: mountpoint.StringPattern{Exactly: "o"}},
							{Key: mountpoint.StringPattern{Exactly: "device"}},
							{Key: mountpoint.StringPattern{Exactly: "type"}},
						},
					}},
				},
			},
		},
	}
	return
}

func mountPointControllerForAllBindMounts() (ctrl *testMountPointController) {
	typeBind := mountpoint.TypeBind
	typeVolume := mountpoint.TypeVolume
	ctrl = emptyMountPointController()

	// matches all bind mounts
	ctrl.propertiesRes = mountpoint.PropertiesResponse{
		Success: true,
		Patterns: []mountpoint.Pattern{
			{Type: &typeBind},
			{
				Type: &typeVolume,
				Volume: mountpoint.VolumePattern{
					Driver: []mountpoint.StringPattern{{Exactly: "local"}},
					Options: []mountpoint.StringMapPattern{{
						Exists: []mountpoint.StringMapKeyValuePattern{{
							Key:   mountpoint.StringPattern{Exactly: "o"},
							Value: mountpoint.StringPattern{Contains: "bind"},
						}},
					}},
				},
			},
		},
	}
	return
}

func mountPointControllerForMountsIntoContainersWithoutAllowLabel() (ctrl *testMountPointController) {
	ctrl = emptyMountPointController()

	// matches all mounts into containers without a label 'allow'
	ctrl.propertiesRes = mountpoint.PropertiesResponse{
		Success: true,
		Patterns: []mountpoint.Pattern{
			{Container: mountpoint.ContainerPattern{
				Labels: []mountpoint.StringMapPattern{{
					Not: true,
					Exists: []mountpoint.StringMapKeyValuePattern{{
						Key: mountpoint.StringPattern{Exactly: "allow"},
					}},
				}},
			}},
		},
	}
	return
}

func setupTest(c *testing.T) func() {
	environment.ProtectImages(c, testEnv)

	d = daemon.New(c, "", dockerdBinary, daemon.Config{
		Experimental: testEnv.DaemonInfo.ExperimentalBuild,
	})

	mountPointController[0] = mountPointControllerForVBindMountOnly()
	mountPointController[1] = mountPointControllerForVBindMountAndAllLocalVolumes()
	mountPointController[2] = mountPointControllerForLocalVolumeBindMounts()
	mountPointController[3] = mountPointControllerForAnonymousBindMounts()
	mountPointController[4] = mountPointControllerForAllBindMounts()
	mountPointController[5] = mountPointControllerForMountsIntoContainersWithoutAllowLabel()

	events = []string{}

	return func() {
		if d != nil {
			d.Stop(c)
			testEnv.Clean(c)
			for i := range mountPointController {
				mountPointController[i] = nil
			}
		}
	}
}

func setupSuite() {
	for i := range mountPointController {
		setupPlugin(i)
	}
}

func setupPlugin(i int) {
	mux := http.NewServeMux()
	server[i] = httptest.NewServer(mux)

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		b, err := json.Marshal(plugins.Manifest{Implements: []string{mountpoint.MountPointAPIImplements}})
		if err != nil {
			panic("could not marshal json for /Plugin.Activate: " + err.Error())
		}
		w.Write(b)
	})

	mux.HandleFunc("/MountPointPlugin.MountPointProperties", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic("could not read body for /MountPointPlugin.MountPointProperties: " + err.Error())
		}
		propertiesReq := mountpoint.PropertiesRequest{}
		err = json.Unmarshal(body, &propertiesReq)
		if err != nil {
			panic("could not unmarshal json for /MountPointPlugin.MountPointProperties: " + err.Error())
		}

		events = append(events, fmt.Sprintf("%d:properties", i))

		propertiesRes := mountPointController[i].propertiesRes
		if !propertiesRes.Success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		b, err := json.Marshal(propertiesRes)
		if err != nil {
			panic("could not marshal json for /MountPointPlugin.MountPointProperties: " + err.Error())
		}
		w.Write(b)
	})

	mux.HandleFunc("/MountPointPlugin.MountPointAttach", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic("could not read body for /MountPointPlugin.MountPointAttach: " + err.Error())
		}
		attachReq := mountpoint.AttachRequest{}
		err = json.Unmarshal(body, &attachReq)
		if err != nil {
			panic("could not unmarshal json for /MountPointPlugin.MountPointAttach: " + err.Error())
		}

		mountPointController[i].attachCnt++
		mountPointController[i].attachMounts = append(mountPointController[i].attachMounts, attachReq.Mounts)
		events = append(events, fmt.Sprintf("%d:attach", i))

		attachRes := mountPointController[i].attachRes
		if !attachRes.Success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		b, err := json.Marshal(attachRes)
		if err != nil {
			panic("could not marshal json for /MountPointPlugin.MountPointAttach: " + err.Error())
		}
		w.Write(b)
	})

	mux.HandleFunc("/MountPointPlugin.MountPointDetach", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic("could not read body for /MountPointPlugin.MountPointDetach: " + err.Error())
		}
		detachReq := mountpoint.DetachRequest{}
		err = json.Unmarshal(body, &detachReq)
		if err != nil {
			panic("could not unmarshal json for /MountPointPlugin.MountPointDetach: " + err.Error())
		}

		events = append(events, fmt.Sprintf("%d:detach", i))

		detachRes := mountPointController[i].detachRes
		if !detachRes.Success && !detachRes.Recoverable {
			w.WriteHeader(http.StatusInternalServerError)
		}
		b, err := json.Marshal(detachRes)
		if err != nil {
			panic("could not marshal json for /MountPointPlugin.MountPointDetach: " + err.Error())
		}
		w.Write(b)
	})

	err := os.MkdirAll("/etc/docker/plugins", 0755)
	if err != nil {
		panic("could not create /etc/docker/plugins: " + err.Error())
	}

	fileName := fmt.Sprintf("/etc/docker/plugins/%s%d.spec", testMountPointPlugin, i)
	err = ioutil.WriteFile(fileName, []byte(server[i].URL), 0644)
	if err != nil {
		panic("could not write " + fileName + ": " + err.Error())
	}
}

func teardownSuite() {
	for i := 0; i < len(server); i++ {
		if server[i] == nil {
			continue
		}

		server[i].Close()
	}

	err := os.RemoveAll("/etc/docker/plugins")
	if err != nil {
		panic("could not remove /etc/docker/plugins: " + err.Error())
	}
}

func TestMountPointPluginNoMounts(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c, fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	id, err := containerRun(client, "busybox", []string{"top"})
	require.Nil(c, err)

	_, err = client.ContainerInspect(context.Background(), id)
	require.Nil(c, err)
	require.Equal(c, 0, mountPointController[0].attachCnt)
}

func TestMountPointPluginError(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c, fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	mountPointController[0].attachRes = mountpoint.AttachResponse{
		Success: false,
		Err:     mountFailMessage,
	}

	_, err = containerRunV(client, []string{"/secret:/host"}, "busybox", []string{"top"})
	require.NotNil(c, err)

	require.True(c, strings.HasSuffix(err.Error(), fmt.Sprintf("Error response from daemon: middleware plugin:%s0 failed with error: %s: %s", testMountPointPlugin, mountpoint.MountPointAPIAttach, mountFailMessage)), err.Error())
	require.Equal(c, 1, mountPointController[0].attachCnt)
}

func TestMountPointPluginFilter(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c, fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	id, err := containerRunV(client, []string{"/host"}, "busybox", []string{"top"})
	require.Nil(c, err)

	_, err = client.ContainerInspect(context.Background(), id)
	require.Nil(c, err)
	require.Equal(c, 0, mountPointController[0].attachCnt)
}

func TestMountPointPluginEnsureNoDuplicatePluginRegistration(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c, fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin), fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	id, err := containerRunV(client, []string{"/:/host"}, "busybox", []string{"top"})
	require.Nil(c, err)

	_, err = client.ContainerInspect(context.Background(), id)
	require.Nil(c, err)

	// assert plugin is only called once
	require.Equal(c, 1, mountPointController[0].attachCnt)
}

func TestMountPointPluginAttachOrderBind(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	id, err := containerRunV(client, []string{"/:/host"}, "busybox", []string{"top"})
	require.Nil(c, err)

	_, err = client.ContainerInspect(context.Background(), id)
	require.Nil(c, err)

	require.Equal(c, []string{"0:properties", "1:properties", "0:attach", "1:attach"}, events)
}

func TestMountPointPluginVolumeFilter(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s2", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s3", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	id, err := containerRunV(client, []string{"/anon"}, "busybox", []string{"top"})
	require.Nil(c, err)

	_, err = client.ContainerInspect(context.Background(), id)
	require.Nil(c, err)

	require.Equal(c, []string{"0:properties", "1:properties", "2:properties", "3:properties", "1:attach", "3:attach"}, events)
}

func TestMountPointPluginLocalFilter(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s2", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	volID, err := containerVolumeCreate(client, "local", map[string]string{"type": "tmpfs", "device": "tmpfs"})
	require.Nil(c, err)

	id, err := containerRunV(client, []string{volID + ":/tmpfs"}, "busybox", []string{"top"})
	require.Nil(c, err)

	_, err = client.ContainerInspect(context.Background(), id)
	require.Nil(c, err)

	require.Equal(c, []string{"0:properties", "1:properties", "2:properties", "1:attach"}, events)
}

func TestMountPointPluginLocalBindFilter(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s2", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	volID, err := containerVolumeCreate(client, "local", map[string]string{"device": "/etc", "o": "ro,bind"})
	require.Nil(c, err)

	id, err := containerRunV(client, []string{volID + ":/tmpfs"}, "busybox", []string{"top"})
	require.Nil(c, err)

	_, err = client.ContainerInspect(context.Background(), id)
	require.Nil(c, err)

	require.Equal(c, []string{"0:properties", "1:properties", "2:properties", "1:attach", "2:attach"}, events)
}

func TestMountPointPluginChangeDirectory(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s3", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	newdir := "/var/run/" + testMountPointPlugin + "1/newdir"
	err = os.MkdirAll(newdir, 0700)
	require.Nil(c, err)
	mountPointController[1].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
				Changes: types.MountPointChanges{
					EffectiveSource: newdir,
				},
			},
		},
	}

	id, err := containerRunV(client, []string{"/anon"}, "busybox", []string{"top"})
	require.Nil(c, err)

	_, err = client.ContainerInspect(context.Background(), id)
	require.Nil(c, err)

	require.Equal(c, []string{"1:properties", "3:properties", "1:attach", "3:attach"}, events)

	require.Equal(c, 1, len(mountPointController[3].attachMounts))
	require.Equal(c, 1, len(mountPointController[3].attachMounts[0]))
	require.Equal(c, newdir, mountPointController[3].attachMounts[0][0].EffectiveSource)
}

func TestMountPointPluginFailureUnwind(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s2", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s4", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	mountPointController[1].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
			},
		},
	}
	mountPointController[2].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
			},
		},
	}
	mountPointController[4].attachRes = mountpoint.AttachResponse{
		Success: false,
		Err:     mountFailMessage,
	}

	volID, err := containerVolumeCreate(client, "local", map[string]string{"device": "/etc", "o": "ro,bind"})
	require.Nil(c, err)

	_, err = containerRunV(client, []string{volID + ":/tmpfs"}, "busybox", []string{"top"})
	require.NotNil(c, err)

	require.True(c, strings.HasSuffix(err.Error(), fmt.Sprintf("Error response from daemon: middleware plugin:%s4 failed with error: %s: %s", testMountPointPlugin, mountpoint.MountPointAPIAttach, mountFailMessage)), err.Error())

	require.Equal(c, []string{"1:properties", "2:properties", "4:properties", "1:attach", "2:attach", "4:attach", "2:detach", "1:detach"}, events)

	mountPointController[2].attachRes.Attachments[0].Attach = false
	events = []string{}

	_, err = containerRunV(client, []string{volID + ":/tmpfs"}, "busybox", []string{"top"})
	require.NotNil(c, err)

	require.True(c, strings.HasSuffix(err.Error(), fmt.Sprintf("Error response from daemon: middleware plugin:%s4 failed with error: %s: %s", testMountPointPlugin, mountpoint.MountPointAPIAttach, mountFailMessage)), err.Error())

	require.Equal(c, []string{"1:attach", "2:attach", "4:attach", "1:detach"}, events)
}

func TestMountPointPluginDetachExit(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s2", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s4", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	mountPointController[1].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
			},
		},
	}
	mountPointController[4].attachRes = mountPointController[1].attachRes

	volID, err := containerVolumeCreate(client, "local", map[string]string{"device": "/etc", "o": "ro,bind"})
	require.Nil(c, err)

	id, err := containerRunV(client, []string{volID + ":/tmpfs"}, "busybox", []string{"true"})
	require.Nil(c, err)

	_, err = containerWait(client, id)
	require.Nil(c, err)

	require.Equal(c, []string{"1:properties", "2:properties", "4:properties", "1:attach", "2:attach", "4:attach", "4:detach", "1:detach"}, events)
}

func TestMountPointPluginMultipleMounts(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s2", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	mountPointController[0].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
				Changes: types.MountPointChanges{
					EffectiveSource: "/usr",
				},
			},
			{ // tests overlong attach lists
				Attach: true,
			},
		},
	}
	mountPointController[1].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
				Changes: types.MountPointChanges{
					EffectiveSource: "/etc",
				},
			},
			{
				Attach: true,
			},
		},
	}
	mountPointController[2].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
			},
		},
	}

	volID, err := containerVolumeCreate(client, "local", map[string]string{"device": "/etc", "o": "ro,bind"})
	require.Nil(c, err)

	id, err := containerRunV(client, []string{volID + ":/host_etc", "/:/host"}, "busybox", []string{"true"})
	require.Nil(c, err)

	_, err = containerWait(client, id)
	require.Nil(c, err)

	require.Equal(c, []string{"0:properties", "1:properties", "2:properties", "0:attach", "1:attach", "2:attach", "2:detach", "1:detach", "0:detach"}, events)

	require.Equal(c, 1, len(mountPointController[0].attachMounts))
	require.Equal(c, 1, len(mountPointController[0].attachMounts[0]))
	require.Equal(c, "/", mountPointController[0].attachMounts[0][0].EffectiveSource)

	require.Equal(c, 1, len(mountPointController[1].attachMounts))
	require.Equal(c, 2, len(mountPointController[1].attachMounts[0]))
	if mountPointController[1].attachMounts[0][0].Source == "/" {
		require.Equal(c, "/usr", mountPointController[1].attachMounts[0][0].EffectiveSource)
	} else {
		require.Equal(c, "/usr", mountPointController[1].attachMounts[0][1].EffectiveSource)
	}

	require.Equal(c, 1, len(mountPointController[2].attachMounts))
	require.Equal(c, 1, len(mountPointController[2].attachMounts[0]))
	require.Equal(c, "/etc", mountPointController[2].attachMounts[0][0].EffectiveSource)
}

func TestMountPointPluginDetachCleanFailure(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	mountPointController[0].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
			},
		},
	}
	mountPointController[1].attachRes = mountPointController[0].attachRes

	mountPointController[1].detachRes = mountpoint.DetachResponse{
		Success:     false,
		Recoverable: true,
		Err:         "kaboom",
	}

	id, err := containerRunV(client, []string{"/:/host"}, "busybox", []string{"true"})
	require.Nil(c, err)

	exitCode, err := containerWait(client, id)
	require.Nil(c, err)
	require.Equal(c, int64(129), exitCode)

	require.Equal(c, []string{"0:properties", "1:properties", "0:attach", "1:attach", "1:detach", "0:detach"}, events)
}

func TestMountPointPluginStopStart(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	mountPointController[0].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
			},
		},
	}
	mountPointController[1].attachRes = mountPointController[0].attachRes

	id, err := containerRunV(client, []string{"/:/host"}, "busybox", []string{"top"})
	require.Nil(c, err)

	err = containerStop(client, id)
	require.Nil(c, err)

	err = containerStart(client, id)
	require.Nil(c, err)

	require.Equal(c, []string{"0:properties", "1:properties", "0:attach", "1:attach", "1:detach", "0:detach", "0:attach", "1:attach"}, events)
}

func TestMountPointPluginKill(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	mountPointController[0].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
			},
		},
	}
	mountPointController[1].attachRes = mountPointController[0].attachRes

	id, err := containerRunV(client, []string{"/:/host"}, "busybox", []string{"top"})
	require.Nil(c, err)

	err = containerKill(client, id)
	require.Nil(c, err)

	require.Equal(c, []string{"0:properties", "1:properties", "0:attach", "1:attach", "1:detach", "0:detach"}, events)
}

func TestMountPointPluginOOM(c *testing.T) {
	skip.IfCondition(c, testEnv.DaemonInfo.OSType != "linux")
	skip.IfCondition(c, !sysInfo.MemoryLimit)
	skip.IfCondition(c, !sysInfo.SwapLimit)
	defer setupTest(c)()

	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	mountPointController[0].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
			},
		},
	}
	mountPointController[1].attachRes = mountPointController[0].attachRes

	id, err := containerRunVMem(client, []string{"/:/host"}, 32, "busybox", []string{"sh", "-c", "x=a; while true; do x=$x$x$x$x; done"})
	require.Nil(c, err)

	exitCode, err := containerWait(client, id)
	require.Nil(c, err)

	require.Equal(c, int64(137), exitCode, "OOM exit should be 137")

	require.Equal(c, []string{"0:properties", "1:properties", "0:attach", "1:attach", "1:detach", "0:detach"}, events)
}

func TestMountPointPluginDaemonRestart(c *testing.T) {
	skip.IfCondition(c, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(c)()

	d.StartWithBusybox(c, "--live-restore",
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin),
		fmt.Sprintf("--mount-point-plugin=%s1", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	mountPointController[0].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
			},
		},
	}
	mountPointController[1].attachRes = mountPointController[0].attachRes

	id, err := containerRunV(client, []string{"/:/host"}, "busybox", []string{"top"})
	require.Nil(c, err)

	// restart without attached plugins
	d.Restart(c, "--live-restore")

	err = containerStop(client, id)
	require.Nil(c, err)

	// uninitialized plugins are initialized before detach
	require.Equal(c, []string{"0:properties", "1:properties", "0:attach", "1:attach", "1:properties", "1:detach", "0:properties", "0:detach"}, events)

	events = []string{}
	id, err = containerRunV(client, []string{"/:/host"}, "busybox", []string{"top"})
	require.Nil(c, err)
	defer containerStop(client, id)

	// no new plugin events have occurred without explicit plugin loading
	require.Equal(c, []string{}, events)
}

func TestMountPointPluginContainerLabel(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s5", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	mountPointController[5].attachRes = mountpoint.AttachResponse{
		Success: false,
		Err:     "only containers with 'allow' label are allowed to mount",
	}

	_, err = containerRunV(client, []string{"/:/host"}, "busybox", []string{"true"})
	require.NotNil(c, err)
	require.True(c, strings.HasSuffix(err.Error(), fmt.Sprintf("Error response from daemon: middleware plugin:%s5 failed with error: %s: only containers with 'allow' label are allowed to mount", testMountPointPlugin, mountpoint.MountPointAPIAttach)), err.Error())

	_, err = containerRunVLabel(client, []string{"/:/host"}, map[string]string{"allow": ""}, "busybox", []string{"true"})
	require.Nil(c, err)
}

func TestMountPointPluginEmergencyDetach(c *testing.T) {
	defer setupTest(c)()
	d.StartWithBusybox(c,
		fmt.Sprintf("--mount-point-plugin=%s0", testMountPointPlugin))
	client, err := d.NewClient()
	require.Nil(c, err)

	pwd, err := os.Getwd()
	require.Nil(c, err)
	targetFile := filepath.Join(pwd, "target")

	mountPointController[0].attachRes = mountpoint.AttachResponse{
		Success: true,
		Attachments: []mountpoint.Attachment{
			{
				Attach: true,
				EmergencyDetach: []mountpoint.EmergencyDetachAction{
					{Remove: targetFile},
					{Warning: "ran emergency detach"},
				},
			},
		},
	}

	id, err := containerRunV(client, []string{pwd + ":/host"}, "busybox",
		[]string{"sh", "-c", "touch /host/target && top"})
	require.Nil(c, err)

	file, err := os.Open(targetFile)
	require.Nil(c, err)
	file.Close()

	err = server[0].Config.Close()
	require.Nil(c, err)
	defer setupPlugin(0)

	err = containerStop(client, id)
	require.Nil(c, err)

	_, err = os.Open(targetFile)
	require.True(c, os.IsNotExist(err))
}
