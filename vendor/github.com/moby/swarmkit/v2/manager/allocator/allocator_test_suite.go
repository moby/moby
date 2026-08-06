package allocator

import (
	"context"
	"net"
	"runtime/debug"
	"strconv"
	"testing"
	"time"

	"github.com/docker/go-events"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
	"github.com/moby/swarmkit/v2/manager/state"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func RunAllocatorTests(t *testing.T, np networkallocator.Provider) {
	// set artificially low retry interval for testing
	retryInterval = 5 * time.Millisecond
	suite.Run(t, &testSuite{np: np})
}

type testSuite struct {
	suite.Suite
	np networkallocator.Provider
}

func (suite *testSuite) newAllocator(store *store.MemoryStore) *Allocator {
	na, err := suite.np.NewAllocator(nil)
	suite.NoError(err)
	a := New(store, na)
	suite.NotNil(a)
	return a
}

// startAllocator starts running allocator a in a background goroutine and returns a function to stop it.
// The returned function blocks until the allocator has stopped. It must be called from the test goroutine.
func (suite *testSuite) startAllocator(a *Allocator) func() {
	done := make(chan error)
	go func() {
		done <- a.Run(context.Background())
	}()
	return func() {
		a.Stop()
		// Prevent data races with suite.T() by checking the error
		// return value synchronously, before the test function returns.
		suite.NoError(<-done)
	}
}

func (suite *testSuite) TestAllocator() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()
	a := suite.newAllocator(s)

	// Predefined node-local networkTestNoDuplicateIPs
	p := &api.Network{
		Id: "one_unIque_id",
		Spec: &api.NetworkSpec{
			Annotations: &api.Annotations{
				Name: "pred_bridge_network",
				Labels: map[string]string{
					"com.docker.swarm.predefined": "true",
				},
			},
			DriverConfig: &api.Driver{Name: "bridge"},
		},
	}

	// Node-local swarm scope network
	nln := &api.Network{
		Id: "another_unIque_id",
		Spec: &api.NetworkSpec{
			Annotations: &api.Annotations{
				Name: "swarm-macvlan",
			},
			DriverConfig: &api.Driver{Name: "macvlan"},
		},
	}

	// Try adding some objects to store before allocator is started
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			Id: "ingress-nw-id",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "default-ingress",
				},
				Ingress: true,
			},
		}
		suite.NoError(store.CreateNetwork(tx, in))

		n1 := &api.Network{
			Id: "testID1",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "test1",
				},
			},
		}
		suite.NoError(store.CreateNetwork(tx, n1))

		s1 := &api.Service{
			Id: "testServiceID1",
			Spec: &api.ServiceSpec{
				Annotations: &api.Annotations{
					Name: "service1",
				},
				Task: &api.TaskSpec{
					Networks: []*api.NetworkAttachmentConfig{
						{
							Target: "testID1",
						},
					},
				},
				Endpoint: &api.EndpointSpec{
					Mode: api.EndpointSpec_VIP,
					Ports: []*api.PortConfig{
						{
							Name:          "some_tcp",
							Protocol:      api.PortConfig_TCP,
							TargetPort:    8000,
							PublishedPort: 8001,
						},
						{
							Name:          "some_udp",
							Protocol:      api.PortConfig_UDP,
							TargetPort:    8000,
							PublishedPort: 8001,
						},
						{
							Name:       "auto_assigned_tcp",
							Protocol:   api.PortConfig_TCP,
							TargetPort: 9000,
						},
						{
							Name:       "auto_assigned_udp",
							Protocol:   api.PortConfig_UDP,
							TargetPort: 9000,
						},
					},
				},
			},
		}
		suite.NoError(store.CreateService(tx, s1))

		t1 := &api.Task{
			Id: "testTaskID1",
			Status: &api.TaskStatus{
				State: api.TaskState_NEW,
			},
			Networks: []*api.NetworkAttachment{
				{
					Network: n1,
				},
			},
		}
		suite.NoError(store.CreateTask(tx, t1))

		t2 := &api.Task{
			Id: "testTaskIDPreInit",
			Status: &api.TaskStatus{
				State: api.TaskState_NEW,
			},
			ServiceId:    "testServiceID1",
			DesiredState: api.TaskState_RUNNING,
		}
		suite.NoError(store.CreateTask(tx, t2))

		// Create the predefined node-local network with one service
		suite.NoError(store.CreateNetwork(tx, p))

		sp1 := &api.Service{
			Id: "predServiceID1",
			Spec: &api.ServiceSpec{
				Annotations: &api.Annotations{
					Name: "predService1",
				},
				Task: &api.TaskSpec{
					Networks: []*api.NetworkAttachmentConfig{
						{
							Target: p.Id,
						},
					},
				},
				Endpoint: &api.EndpointSpec{Mode: api.EndpointSpec_DNSRR},
			},
		}
		suite.NoError(store.CreateService(tx, sp1))

		tp1 := &api.Task{
			Id: "predTaskID1",
			Status: &api.TaskStatus{
				State: api.TaskState_NEW,
			},
			Networks: []*api.NetworkAttachment{
				{
					Network: p,
				},
			},
		}
		suite.NoError(store.CreateTask(tx, tp1))

		// Create the the swarm level node-local network with one service
		suite.NoError(store.CreateNetwork(tx, nln))

		sp2 := &api.Service{
			Id: "predServiceID2",
			Spec: &api.ServiceSpec{
				Annotations: &api.Annotations{
					Name: "predService2",
				},
				Task: &api.TaskSpec{
					Networks: []*api.NetworkAttachmentConfig{
						{
							Target: nln.Id,
						},
					},
				},
				Endpoint: &api.EndpointSpec{Mode: api.EndpointSpec_DNSRR},
			},
		}
		suite.NoError(store.CreateService(tx, sp2))

		tp2 := &api.Task{
			Id: "predTaskID2",
			Status: &api.TaskStatus{
				State: api.TaskState_NEW,
			},
			Networks: []*api.NetworkAttachment{
				{
					Network: nln,
				},
			},
		}
		suite.NoError(store.CreateTask(tx, tp2))

		return nil
	}))

	netWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateNetwork{}, api.EventDeleteNetwork{})
	defer cancel()
	taskWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateTask{}, api.EventDeleteTask{})
	defer cancel()
	serviceWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateService{}, api.EventDeleteService{})
	defer cancel()

	defer suite.startAllocator(a)()

	// Now verify if we get network and tasks updated properly
	watchNetwork(suite.T(), netWatch, false, isValidNetwork)
	watchTask(suite.T(), s, taskWatch, false, isValidTask) // t1
	watchTask(suite.T(), s, taskWatch, false, isValidTask) // t2
	watchService(suite.T(), serviceWatch, false, nil)

	// Verify no allocation was done for the node-local networks
	var (
		ps *api.Network
		sn *api.Network
	)
	s.View(func(tx store.ReadTx) {
		ps = store.GetNetwork(tx, p.Id)
		sn = store.GetNetwork(tx, nln.Id)

	})
	suite.NotNil(ps)
	suite.NotNil(sn)
	// Verify no allocation was done for tasks on node-local networks
	var (
		tp1 *api.Task
		tp2 *api.Task
	)
	s.View(func(tx store.ReadTx) {
		tp1 = store.GetTask(tx, "predTaskID1")
		tp2 = store.GetTask(tx, "predTaskID2")
	})
	suite.NotNil(tp1)
	suite.NotNil(tp2)
	suite.Equal(tp1.Networks[0].Network.Id, p.Id)
	suite.Equal(tp2.Networks[0].Network.Id, nln.Id)
	suite.Nil(tp1.Networks[0].Addresses, "Non nil addresses for task on node-local network")
	suite.Nil(tp2.Networks[0].Addresses, "Non nil addresses for task on node-local network")
	// Verify service ports were allocated
	s.View(func(tx store.ReadTx) {
		s1 := store.GetService(tx, "testServiceID1")
		if suite.NotNil(s1) && suite.NotNil(s1.Endpoint) && suite.Len(s1.Endpoint.Ports, 4) {
			// "some_tcp" and "some_udp"
			for _, i := range []int{0, 1} {
				suite.True(s1.Spec.GetEndpoint().GetPorts()[i].EqualVT(s1.Endpoint.Ports[i]))
			}
			// "auto_assigned_tcp" and "auto_assigned_udp"
			for _, i := range []int{2, 3} {
				suite.Equal(s1.Spec.GetEndpoint().GetPorts()[i].TargetPort, s1.Endpoint.Ports[i].TargetPort)
				suite.GreaterOrEqual(s1.Endpoint.Ports[i].PublishedPort, uint32(dynamicPortStart))
				suite.LessOrEqual(s1.Endpoint.Ports[i].PublishedPort, uint32(dynamicPortEnd))
			}
		}
	})

	// Add new networks/tasks/services after allocator is started.
	suite.NoError(s.Update(func(tx store.Tx) error {
		n2 := &api.Network{
			Id: "testID2",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "test2",
				},
			},
		}
		suite.NoError(store.CreateNetwork(tx, n2))
		return nil
	}))

	watchNetwork(suite.T(), netWatch, false, isValidNetwork)

	suite.NoError(s.Update(func(tx store.Tx) error {
		s2 := &api.Service{
			Id: "testServiceID2",
			Spec: &api.ServiceSpec{
				Annotations: &api.Annotations{
					Name: "service2",
				},
				Networks: []*api.NetworkAttachmentConfig{
					{
						Target: "testID2",
					},
				},
				Endpoint: &api.EndpointSpec{},
			},
		}
		suite.NoError(store.CreateService(tx, s2))
		return nil
	}))

	watchService(suite.T(), serviceWatch, false, nil)

	suite.NoError(s.Update(func(tx store.Tx) error {
		t2 := &api.Task{
			Id: "testTaskID2",
			Status: &api.TaskStatus{
				State: api.TaskState_NEW,
			},
			ServiceId:    "testServiceID2",
			DesiredState: api.TaskState_RUNNING,
		}
		suite.NoError(store.CreateTask(tx, t2))
		return nil
	}))

	watchTask(suite.T(), s, taskWatch, false, isValidTask)

	// Now try adding a task which depends on a network before adding the network.
	n3 := &api.Network{
		Id: "testID3",
		Spec: &api.NetworkSpec{
			Annotations: &api.Annotations{
				Name: "test3",
			},
		},
	}

	suite.NoError(s.Update(func(tx store.Tx) error {
		t3 := &api.Task{
			Id: "testTaskID3",
			Status: &api.TaskStatus{
				State: api.TaskState_NEW,
			},
			DesiredState: api.TaskState_RUNNING,
			Networks: []*api.NetworkAttachment{
				{
					Network: n3,
				},
			},
		}
		suite.NoError(store.CreateTask(tx, t3))
		return nil
	}))

	// Wait for a little bit of time before adding network just to
	// test network is not available while task allocation is
	// going through
	time.Sleep(10 * time.Millisecond)

	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.CreateNetwork(tx, n3))
		return nil
	}))

	watchNetwork(suite.T(), netWatch, false, isValidNetwork)
	watchTask(suite.T(), s, taskWatch, false, isValidTask)

	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.DeleteTask(tx, "testTaskID3"))
		return nil
	}))
	watchTask(suite.T(), s, taskWatch, false, isValidTask)

	suite.NoError(s.Update(func(tx store.Tx) error {
		t5 := &api.Task{
			Id: "testTaskID5",
			Spec: &api.TaskSpec{
				Networks: []*api.NetworkAttachmentConfig{
					{
						Target: "testID2",
					},
				},
			},
			Status: &api.TaskStatus{
				State: api.TaskState_NEW,
			},
			DesiredState: api.TaskState_RUNNING,
			ServiceId:    "testServiceID2",
		}
		suite.NoError(store.CreateTask(tx, t5))
		return nil
	}))
	watchTask(suite.T(), s, taskWatch, false, isValidTask)

	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.DeleteNetwork(tx, "testID3"))
		return nil
	}))
	watchNetwork(suite.T(), netWatch, false, isValidNetwork)

	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.DeleteService(tx, "testServiceID2"))
		return nil
	}))
	watchService(suite.T(), serviceWatch, false, nil)

	// Try to create a task with no network attachments and test
	// that it moves to ALLOCATED state.
	suite.NoError(s.Update(func(tx store.Tx) error {
		t4 := &api.Task{
			Id: "testTaskID4",
			Status: &api.TaskStatus{
				State: api.TaskState_NEW,
			},
			DesiredState: api.TaskState_RUNNING,
		}
		suite.NoError(store.CreateTask(tx, t4))
		return nil
	}))
	watchTask(suite.T(), s, taskWatch, false, isValidTask)

	suite.NoError(s.Update(func(tx store.Tx) error {
		n2 := store.GetNetwork(tx, "testID2")
		require.NotEqual(suite.T(), nil, n2)
		suite.NoError(store.UpdateNetwork(tx, n2))
		return nil
	}))
	watchNetwork(suite.T(), netWatch, false, isValidNetwork)
	watchNetwork(suite.T(), netWatch, true, nil)

	// Try updating service which is already allocated with no endpointSpec
	suite.NoError(s.Update(func(tx store.Tx) error {
		s := store.GetService(tx, "testServiceID1")
		s.Spec.Endpoint = nil

		suite.NoError(store.UpdateService(tx, s))
		return nil
	}))
	watchService(suite.T(), serviceWatch, false, nil)

	// Try updating task which is already allocated
	suite.NoError(s.Update(func(tx store.Tx) error {
		t2 := store.GetTask(tx, "testTaskID2")
		require.NotEqual(suite.T(), nil, t2)
		suite.NoError(store.UpdateTask(tx, t2))
		return nil
	}))
	watchTask(suite.T(), s, taskWatch, false, isValidTask)
	watchTask(suite.T(), s, taskWatch, true, nil)

	// Try adding networks with conflicting network resources and
	// add task which attaches to a network which gets allocated
	// later and verify if task reconciles and moves to ALLOCATED.
	n4 := &api.Network{
		Id: "testID4",
		Spec: &api.NetworkSpec{
			Annotations: &api.Annotations{
				Name: "test4",
			},
			DriverConfig: &api.Driver{
				Name: "overlay",
				Options: map[string]string{
					"com.docker.network.driver.overlay.vxlanid_list": "328",
				},
			},
		},
	}

	n5 := n4.Copy()
	n5.Id = "testID5"
	n5.Spec.Annotations.Name = "test5"
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.CreateNetwork(tx, n4))
		return nil
	}))
	watchNetwork(suite.T(), netWatch, false, isValidNetwork)

	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.CreateNetwork(tx, n5))
		return nil
	}))
	watchNetwork(suite.T(), netWatch, true, nil)

	suite.NoError(s.Update(func(tx store.Tx) error {
		t6 := &api.Task{
			Id: "testTaskID6",
			Status: &api.TaskStatus{
				State: api.TaskState_NEW,
			},
			DesiredState: api.TaskState_RUNNING,
			Networks: []*api.NetworkAttachment{
				{
					Network: n5,
				},
			},
		}
		suite.NoError(store.CreateTask(tx, t6))
		return nil
	}))
	watchTask(suite.T(), s, taskWatch, true, nil)

	// Now remove the conflicting network.
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.DeleteNetwork(tx, n4.Id))
		return nil
	}))
	watchNetwork(suite.T(), netWatch, false, isValidNetwork)
	watchTask(suite.T(), s, taskWatch, false, isValidTask)

	// Try adding services with conflicting port configs and add
	// task which is part of the service whose allocation hasn't
	// happened and when that happens later and verify if task
	// reconciles and moves to ALLOCATED.
	s3 := &api.Service{
		Id: "testServiceID3",
		Spec: &api.ServiceSpec{
			Annotations: &api.Annotations{
				Name: "service3",
			},
			Endpoint: &api.EndpointSpec{
				Ports: []*api.PortConfig{
					{
						Name:          "http",
						TargetPort:    80,
						PublishedPort: 8080,
					},
					{
						PublishMode: api.PortConfig_HOST,
						Name:        "http",
						TargetPort:  80,
					},
				},
			},
		},
	}

	s4 := s3.Copy()
	s4.Id = "testServiceID4"
	s4.Spec.Annotations.Name = "service4"
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.CreateService(tx, s3))
		return nil
	}))
	watchService(suite.T(), serviceWatch, false, nil)
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.CreateService(tx, s4))
		return nil
	}))
	watchService(suite.T(), serviceWatch, true, nil)

	suite.NoError(s.Update(func(tx store.Tx) error {
		t7 := &api.Task{
			Id: "testTaskID7",
			Status: &api.TaskStatus{
				State: api.TaskState_NEW,
			},
			ServiceId:    "testServiceID4",
			DesiredState: api.TaskState_RUNNING,
		}
		suite.NoError(store.CreateTask(tx, t7))
		return nil
	}))
	watchTask(suite.T(), s, taskWatch, true, nil)

	// Now remove the conflicting service.
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.DeleteService(tx, s3.Id))
		return nil
	}))
	watchService(suite.T(), serviceWatch, false, nil)
	watchTask(suite.T(), s, taskWatch, false, isValidTask)
}

func (suite *testSuite) TestNoDuplicateIPs() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()

	// Try adding some objects to store before allocator is started
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			Id: "ingress-nw-id",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "default-ingress",
				},
				Ingress: true,
			},
			Ipam: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "10.0.0.0/24",
						Gateway: "10.0.0.1",
					},
				},
			},
			DriverState: &api.Driver{},
		}
		suite.NoError(store.CreateNetwork(tx, in))
		n1 := &api.Network{
			Id: "testID1",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "test1",
				},
			},
			Ipam: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "10.1.0.0/24",
						Gateway: "10.1.0.1",
					},
				},
			},
			DriverState: &api.Driver{},
		}
		suite.NoError(store.CreateNetwork(tx, n1))

		s1 := &api.Service{
			Id: "testServiceID1",
			Spec: &api.ServiceSpec{
				Annotations: &api.Annotations{
					Name: "service1",
				},
				Task: &api.TaskSpec{
					Networks: []*api.NetworkAttachmentConfig{
						{
							Target: "testID1",
						},
					},
				},
				Endpoint: &api.EndpointSpec{
					Mode: api.EndpointSpec_VIP,
					Ports: []*api.PortConfig{
						{
							Name:          "portName",
							Protocol:      api.PortConfig_TCP,
							TargetPort:    8000,
							PublishedPort: 8001,
						},
					},
				},
			},
		}
		suite.NoError(store.CreateService(tx, s1))

		return nil
	}))

	taskWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateTask{}, api.EventDeleteTask{})
	defer cancel()

	assignedIPs := make(map[string]string)
	hasUniqueIP := func(_ assert.TestingT, _ *store.MemoryStore, task *api.Task) bool {
		if len(task.Networks) == 0 {
			panic("missing networks")
		}
		if len(task.Networks[0].Addresses) == 0 {
			panic("missing network address")
		}

		assignedIP := task.Networks[0].Addresses[0]
		oldTaskID, present := assignedIPs[assignedIP]
		if present && task.Id != oldTaskID {
			suite.T().Fatalf("task %s assigned duplicate IP %s, previously assigned to task %s", task.Id, assignedIP, oldTaskID)
		}
		assignedIPs[assignedIP] = task.Id
		return true
	}

	const reps = 100
	for i := range reps {
		suite.NoError(s.Update(func(tx store.Tx) error {
			t2 := &api.Task{
				// The allocator iterates over the tasks in
				// lexical order, so number tasks in descending
				// order. Note that the problem this test was
				// meant to trigger also showed up with tasks
				// numbered in ascending order, but it took
				// until the 52nd task.
				Id: "testTaskID" + strconv.Itoa(reps-i),
				Status: &api.TaskStatus{
					State: api.TaskState_NEW,
				},
				ServiceId:    "testServiceID1",
				DesiredState: api.TaskState_RUNNING,
			}
			suite.NoError(store.CreateTask(tx, t2))

			return nil
		}))
		a := suite.newAllocator(s)
		stop := suite.startAllocator(a)

		// Confirm task gets a unique IP
		watchTask(suite.T(), s, taskWatch, false, hasUniqueIP)
		stop()
	}
}

func (suite *testSuite) TestAllocatorRestoreForDuplicateIPs() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()
	// Create 3 services with 1 task each
	numsvcstsks := 3
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			Id: "ingress-nw-id",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "default-ingress",
				},
				Ingress: true,
			},
			Ipam: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "10.0.0.0/24",
						Gateway: "10.0.0.1",
					},
				},
			},
		}
		suite.NoError(store.CreateNetwork(tx, in))

		for i := range numsvcstsks {
			svc := &api.Service{
				Id: "testServiceID" + strconv.Itoa(i),
				Spec: &api.ServiceSpec{
					Annotations: &api.Annotations{
						Name: "service" + strconv.Itoa(i),
					},
					Endpoint: &api.EndpointSpec{
						Mode: api.EndpointSpec_VIP,

						Ports: []*api.PortConfig{
							{
								Name:          "",
								Protocol:      api.PortConfig_TCP,
								TargetPort:    8000,
								PublishedPort: uint32(8001 + i),
							},
						},
					},
				},
				Endpoint: &api.Endpoint{
					Ports: []*api.PortConfig{
						{
							Name:          "",
							Protocol:      api.PortConfig_TCP,
							TargetPort:    8000,
							PublishedPort: uint32(8001 + i),
						},
					},
					VirtualIps: []*api.Endpoint_VirtualIP{
						{
							NetworkId: "ingress-nw-id",
							Addr:      "10.0.0." + strconv.Itoa(2+i) + "/24",
						},
					},
				},
			}
			suite.NoError(store.CreateService(tx, svc))
		}
		return nil
	}))

	for i := range numsvcstsks {
		suite.NoError(s.Update(func(tx store.Tx) error {
			tsk := &api.Task{
				Id: "testTaskID" + strconv.Itoa(i),
				Status: &api.TaskStatus{
					State: api.TaskState_NEW,
				},
				ServiceId:    "testServiceID" + strconv.Itoa(i),
				DesiredState: api.TaskState_RUNNING,
			}
			suite.NoError(store.CreateTask(tx, tsk))
			return nil
		}))
	}

	assignedVIPs := make(map[string]bool)
	assignedIPs := make(map[string]bool)
	hasNoIPOverlapServices := func(fakeT assert.TestingT, service *api.Service) bool {
		assert.NotEqual(fakeT, len(service.Endpoint.VirtualIps), 0)
		assert.NotEqual(fakeT, len(service.Endpoint.VirtualIps[0].Addr), 0)

		assignedVIP := service.Endpoint.VirtualIps[0].Addr
		if assignedVIPs[assignedVIP] {
			suite.T().Fatalf("service %s assigned duplicate IP %s", service.Id, assignedVIP)
		}
		assignedVIPs[assignedVIP] = true
		if assignedIPs[assignedVIP] {
			suite.T().Fatalf("a task and service %s have the same IP %s", service.Id, assignedVIP)
		}
		return true
	}

	hasNoIPOverlapTasks := func(fakeT assert.TestingT, _ *store.MemoryStore, task *api.Task) bool {
		assert.NotEqual(fakeT, len(task.Networks), 0)
		assert.NotEqual(fakeT, len(task.Networks[0].Addresses), 0)

		assignedIP := task.Networks[0].Addresses[0]
		if assignedIPs[assignedIP] {
			suite.T().Fatalf("task %s assigned duplicate IP %s", task.Id, assignedIP)
		}
		assignedIPs[assignedIP] = true
		if assignedVIPs[assignedIP] {
			suite.T().Fatalf("a service and task %s have the same IP %s", task.Id, assignedIP)
		}
		return true
	}

	a := suite.newAllocator(s)
	defer suite.startAllocator(a)()

	taskWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateTask{}, api.EventDeleteTask{})
	defer cancel()

	serviceWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateService{}, api.EventDeleteService{})
	defer cancel()

	// Confirm tasks have no IPs that overlap with the services VIPs on restart
	for range numsvcstsks {
		watchTask(suite.T(), s, taskWatch, false, hasNoIPOverlapTasks)
		watchService(suite.T(), serviceWatch, false, hasNoIPOverlapServices)
	}
}

// TestAllocatorRestartNoEndpointSpec covers the leader election case when the service Spec
// does not contain the EndpointSpec.
// The expected behavior is that the VIP(s) are still correctly populated inside
// the IPAM and that no configuration on the service is changed.
func (suite *testSuite) TestAllocatorRestartNoEndpointSpec() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()
	// Create 3 services with 1 task each
	numsvcstsks := 3
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			Id: "overlay1",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "net1",
				},
			},
			Ipam: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "10.0.0.0/24",
						Gateway: "10.0.0.1",
					},
				},
			},
			DriverState: &api.Driver{},
		}
		suite.NoError(store.CreateNetwork(tx, in))

		for i := range numsvcstsks {
			svc := &api.Service{
				Id: "testServiceID" + strconv.Itoa(i),
				Spec: &api.ServiceSpec{
					Annotations: &api.Annotations{
						Name: "service" + strconv.Itoa(i),
					},
					// Endpoint: &api.EndpointSpec{
					// 	Mode: api.ResolutionModeVirtualIP,
					// },
					Task: &api.TaskSpec{
						Networks: []*api.NetworkAttachmentConfig{
							{
								Target: "overlay1",
							},
						},
					},
				},
				Endpoint: &api.Endpoint{
					Spec: &api.EndpointSpec{
						Mode: api.EndpointSpec_VIP,
					},
					VirtualIps: []*api.Endpoint_VirtualIP{
						{
							NetworkId: "overlay1",
							Addr:      "10.0.0." + strconv.Itoa(2+2*i) + "/24",
						},
					},
				},
			}
			suite.NoError(store.CreateService(tx, svc))
		}
		return nil
	}))

	for i := range numsvcstsks {
		suite.NoError(s.Update(func(tx store.Tx) error {
			tsk := &api.Task{
				Id: "testTaskID" + strconv.Itoa(i),
				Status: &api.TaskStatus{
					State: api.TaskState_NEW,
				},
				ServiceId:    "testServiceID" + strconv.Itoa(i),
				DesiredState: api.TaskState_RUNNING,
				Networks: []*api.NetworkAttachment{
					{
						Network: &api.Network{
							Id: "overlay1",
						},
					},
				},
			}
			suite.NoError(store.CreateTask(tx, tsk))
			return nil
		}))
	}

	expectedIPs := map[string]string{
		"testServiceID0": "10.0.0.2/24",
		"testServiceID1": "10.0.0.4/24",
		"testServiceID2": "10.0.0.6/24",
		"testTaskID0":    "10.0.0.3/24",
		"testTaskID1":    "10.0.0.5/24",
		"testTaskID2":    "10.0.0.7/24",
	}
	assignedIPs := make(map[string]bool)
	hasNoIPOverlapServices := func(fakeT assert.TestingT, service *api.Service) bool {
		assert.NotEqual(fakeT, len(service.Endpoint.VirtualIps), 0)
		assert.NotEqual(fakeT, len(service.Endpoint.VirtualIps[0].Addr), 0)
		assignedVIP := service.Endpoint.VirtualIps[0].Addr
		if assignedIPs[assignedVIP] {
			suite.T().Fatalf("service %s assigned duplicate IP %s", service.Id, assignedVIP)
		}
		assignedIPs[assignedVIP] = true
		ip, ok := expectedIPs[service.Id]
		suite.True(ok)
		suite.Equal(ip, assignedVIP)
		delete(expectedIPs, service.Id)
		return true
	}

	hasNoIPOverlapTasks := func(fakeT assert.TestingT, _ *store.MemoryStore, task *api.Task) bool {
		assert.NotEqual(fakeT, len(task.Networks), 0)
		assert.NotEqual(fakeT, len(task.Networks[0].Addresses), 0)
		assignedIP := task.Networks[0].Addresses[0]
		if assignedIPs[assignedIP] {
			suite.T().Fatalf("task %s assigned duplicate IP %s", task.Id, assignedIP)
		}
		assignedIPs[assignedIP] = true
		ip, ok := expectedIPs[task.Id]
		suite.True(ok)
		suite.Equal(ip, assignedIP)
		delete(expectedIPs, task.Id)
		return true
	}

	a := suite.newAllocator(s)
	defer suite.startAllocator(a)()

	taskWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateTask{}, api.EventDeleteTask{})
	defer cancel()

	serviceWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateService{}, api.EventDeleteService{})
	defer cancel()

	// Confirm tasks have no IPs that overlap with the services VIPs on restart
	for range numsvcstsks {
		watchTask(suite.T(), s, taskWatch, false, hasNoIPOverlapTasks)
		watchService(suite.T(), serviceWatch, false, hasNoIPOverlapServices)
	}
	suite.Len(expectedIPs, 0)
}

// TestAllocatorRestoreForUnallocatedNetwork tests allocator restart
// scenarios where there is a combination of allocated and unallocated
// networks and tests whether the restore logic ensures the networks
// services and tasks that were preallocated are allocated correctly
// followed by the allocation of unallocated networks prior to the
// restart.
func (suite *testSuite) TestAllocatorRestoreForUnallocatedNetwork() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()
	// Create 3 services with 1 task each
	numsvcstsks := 3
	var n1 *api.Network
	var n2 *api.Network
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			Id: "ingress-nw-id",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "default-ingress",
				},
				Ingress: true,
			},
			Ipam: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "10.0.0.0/24",
						Gateway: "10.0.0.1",
					},
				},
			},
		}
		suite.NoError(store.CreateNetwork(tx, in))

		n1 = &api.Network{
			Id: "testID1",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "test1",
				},
			},
			Ipam: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "10.1.0.0/24",
						Gateway: "10.1.0.1",
					},
				},
			},
			DriverState: &api.Driver{},
		}
		suite.NoError(store.CreateNetwork(tx, n1))

		n2 = &api.Network{
			// Intentionally named testID0 so that in restore this network
			// is looked into first
			Id: "testID0",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "test2",
				},
			},
		}
		suite.NoError(store.CreateNetwork(tx, n2))

		for i := range numsvcstsks {
			svc := &api.Service{
				Id: "testServiceID" + strconv.Itoa(i),
				Spec: &api.ServiceSpec{
					Annotations: &api.Annotations{
						Name: "service" + strconv.Itoa(i),
					},
					Task: &api.TaskSpec{
						Networks: []*api.NetworkAttachmentConfig{
							{
								Target: "testID1",
							},
						},
					},
					Endpoint: &api.EndpointSpec{
						Mode: api.EndpointSpec_VIP,
						Ports: []*api.PortConfig{
							{
								Name:          "",
								Protocol:      api.PortConfig_TCP,
								TargetPort:    8000,
								PublishedPort: uint32(8001 + i),
							},
						},
					},
				},
				Endpoint: &api.Endpoint{
					Ports: []*api.PortConfig{
						{
							Name:          "",
							Protocol:      api.PortConfig_TCP,
							TargetPort:    8000,
							PublishedPort: uint32(8001 + i),
						},
					},
					VirtualIps: []*api.Endpoint_VirtualIP{
						{
							NetworkId: "ingress-nw-id",
							Addr:      "10.0.0." + strconv.Itoa(2+i) + "/24",
						},
						{
							NetworkId: "testID1",
							Addr:      "10.1.0." + strconv.Itoa(2+i) + "/24",
						},
					},
				},
			}
			suite.NoError(store.CreateService(tx, svc))
		}
		return nil
	}))

	for i := range numsvcstsks {
		suite.NoError(s.Update(func(tx store.Tx) error {
			tsk := &api.Task{
				Id: "testTaskID" + strconv.Itoa(i),
				Status: &api.TaskStatus{
					State: api.TaskState_NEW,
				},
				Spec: &api.TaskSpec{
					Networks: []*api.NetworkAttachmentConfig{
						{
							Target: "testID1",
						},
					},
				},
				ServiceId:    "testServiceID" + strconv.Itoa(i),
				DesiredState: api.TaskState_RUNNING,
			}
			suite.NoError(store.CreateTask(tx, tsk))
			return nil
		}))
	}

	assignedIPs := make(map[string]bool)
	expectedIPs := map[string]string{
		"testServiceID0": "10.1.0.2/24",
		"testServiceID1": "10.1.0.3/24",
		"testServiceID2": "10.1.0.4/24",
		"testTaskID0":    "10.1.0.5/24",
		"testTaskID1":    "10.1.0.6/24",
		"testTaskID2":    "10.1.0.7/24",
	}
	hasNoIPOverlapServices := func(fakeT assert.TestingT, service *api.Service) bool {
		assert.NotEqual(fakeT, len(service.Endpoint.VirtualIps), 0)
		assert.NotEqual(fakeT, len(service.Endpoint.VirtualIps[0].Addr), 0)
		assignedVIP := service.Endpoint.VirtualIps[1].Addr
		if assignedIPs[assignedVIP] {
			suite.T().Fatalf("service %s assigned duplicate IP %s", service.Id, assignedVIP)
		}
		assignedIPs[assignedVIP] = true
		ip, ok := expectedIPs[service.Id]
		suite.True(ok)
		suite.Equal(ip, assignedVIP)
		delete(expectedIPs, service.Id)
		return true
	}

	hasNoIPOverlapTasks := func(fakeT assert.TestingT, _ *store.MemoryStore, task *api.Task) bool {
		assert.NotEqual(fakeT, len(task.Networks), 0)
		assert.NotEqual(fakeT, len(task.Networks[0].Addresses), 0)
		assignedIP := task.Networks[1].Addresses[0]
		if assignedIPs[assignedIP] {
			suite.T().Fatalf("task %s assigned duplicate IP %s", task.Id, assignedIP)
		}
		assignedIPs[assignedIP] = true
		ip, ok := expectedIPs[task.Id]
		suite.True(ok)
		suite.Equal(ip, assignedIP)
		delete(expectedIPs, task.Id)
		return true
	}

	a := suite.newAllocator(s)
	defer suite.startAllocator(a)()

	taskWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateTask{}, api.EventDeleteTask{})
	defer cancel()

	serviceWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateService{}, api.EventDeleteService{})
	defer cancel()

	// Confirm tasks have no IPs that overlap with the services VIPs on restart
	for range numsvcstsks {
		watchTask(suite.T(), s, taskWatch, false, hasNoIPOverlapTasks)
		watchService(suite.T(), serviceWatch, false, hasNoIPOverlapServices)
	}
}

func (suite *testSuite) TestNodeAllocator() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()

	a := suite.newAllocator(s)

	var node1FromStore *api.Node
	node1 := &api.Node{
		Id: "nodeID1",
	}

	// Try adding some objects to store before allocator is started
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			Id: "ingress",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "ingress",
				},
				Ingress: true,
			},
		}
		suite.NoError(store.CreateNetwork(tx, in))

		n1 := &api.Network{
			Id: "overlayID1",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "overlayID1",
				},
			},
		}
		suite.NoError(store.CreateNetwork(tx, n1))

		// this network will never be used for any task
		nUnused := &api.Network{
			Id: "overlayIDUnused",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "overlayIDUnused",
				},
			},
		}
		suite.NoError(store.CreateNetwork(tx, nUnused))

		suite.NoError(store.CreateNode(tx, node1))

		return nil
	}))

	nodeWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateNode{}, api.EventDeleteNode{})
	defer cancel()
	netWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateNetwork{}, api.EventDeleteNetwork{})
	defer cancel()
	taskWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateTask{})
	defer cancel()

	defer suite.startAllocator(a)()

	suite.NoError(s.Update(func(tx store.Tx) error {
		// create a task assigned to this node that has a network attachment on
		// n1
		t1 := &api.Task{
			Id:           "task1",
			NodeId:       node1.Id,
			DesiredState: api.TaskState_RUNNING,
			Spec: &api.TaskSpec{
				Networks: []*api.NetworkAttachmentConfig{
					{
						Target: "overlayID1",
					},
				},
			},
		}

		return store.CreateTask(tx, t1)
	}))

	// validate that the task is created
	watchTask(suite.T(), s, taskWatch, false, isValidTask)

	// Validate node has 2 LB IP address (1 for each network).
	watchNetwork(suite.T(), netWatch, false, isValidNetwork)                                      // ingress
	watchNetwork(suite.T(), netWatch, false, isValidNetwork)                                      // overlayID1
	watchNetwork(suite.T(), netWatch, false, isValidNetwork)                                      // overlayIDUnused
	watchNode(suite.T(), nodeWatch, false, isValidNode, node1, []string{"ingress", "overlayID1"}) // node1

	// Add a node and validate it gets a LB ip only on ingress, as it has no
	// tasks assigned.
	node2 := &api.Node{
		Id: "nodeID2",
	}
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.CreateNode(tx, node2))
		return nil
	}))
	watchNode(suite.T(), nodeWatch, false, isValidNode, node2, []string{"ingress"}) // node2

	// Add a network and validate that nothing has changed in the nodes
	n2 := &api.Network{
		Id: "overlayID2",
		Spec: &api.NetworkSpec{
			Annotations: &api.Annotations{
				Name: "overlayID2",
			},
		},
	}
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.CreateNetwork(tx, n2))
		return nil
	}))
	watchNetwork(suite.T(), netWatch, false, isValidNetwork) // overlayID2
	// nothing should change, no updates
	watchNode(suite.T(), nodeWatch, true, isValidNode, node1, []string{"ingress", "overlayID1"}) // node1
	watchNode(suite.T(), nodeWatch, true, isValidNode, node2, []string{"ingress"})               // node2

	// add a task and validate that the node gets the network for the task
	suite.NoError(s.Update(func(tx store.Tx) error {
		// create a task assigned to this node that has a network attachment on
		// n1
		t2 := &api.Task{
			Id:           "task2",
			NodeId:       node2.Id,
			DesiredState: api.TaskState_RUNNING,
			Spec: &api.TaskSpec{
				Networks: []*api.NetworkAttachmentConfig{
					{
						Target: "overlayID2",
					},
				},
			},
		}

		return store.CreateTask(tx, t2)
	}))
	// validate that the task is created
	watchTask(suite.T(), s, taskWatch, false, isValidTask)

	// validate that node2 gets a new attachment and node1 stays the same
	watchNode(suite.T(), nodeWatch, false, isValidNode, node2, []string{"ingress", "overlayID2"}) // node2
	watchNode(suite.T(), nodeWatch, true, isValidNode, node1, []string{"ingress", "overlayID1"})  // node1

	// add another task with the same network to a node and validate that it
	// still only has 1 attachment for that network
	suite.NoError(s.Update(func(tx store.Tx) error {
		// create a task assigned to this node that has a network attachment on
		// n1
		t3 := &api.Task{
			Id:           "task3",
			NodeId:       node1.Id,
			DesiredState: api.TaskState_RUNNING,
			Spec: &api.TaskSpec{
				Networks: []*api.NetworkAttachmentConfig{
					{
						Target: "overlayID1",
					},
				},
			},
		}

		return store.CreateTask(tx, t3)
	}))

	// validate that the task is created
	watchTask(suite.T(), s, taskWatch, false, isValidTask)

	// validate that nothing changes
	watchNode(suite.T(), nodeWatch, true, isValidNode, node1, []string{"ingress", "overlayID1"}) // node1
	watchNode(suite.T(), nodeWatch, true, isValidNode, node2, []string{"ingress", "overlayID2"}) // node2

	// now remove that task we just created, and validate that the node still
	// has an attachment for the other task
	// Remove a node and validate remaining node has 2 LB IP addresses
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.DeleteTask(tx, "task1"))
		return nil
	}))

	// validate that nothing changes
	watchNode(suite.T(), nodeWatch, true, isValidNode, node1, []string{"ingress", "overlayID1"}) // node1
	watchNode(suite.T(), nodeWatch, true, isValidNode, node2, []string{"ingress", "overlayID2"}) // node2

	// now remove another task. this time the attachment on the node should be
	// removed as well
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.DeleteTask(tx, "task2"))
		return nil
	}))

	watchNode(suite.T(), nodeWatch, false, isValidNode, node2, []string{"ingress"})              // node2
	watchNode(suite.T(), nodeWatch, true, isValidNode, node1, []string{"ingress", "overlayID1"}) // node1

	// Remove a node and validate remaining node has 2 LB IP addresses
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.DeleteNode(tx, node2.Id))
		return nil
	}))
	watchNode(suite.T(), nodeWatch, false, nil, nil, nil) // node2
	s.View(func(tx store.ReadTx) {
		node1FromStore = store.GetNode(tx, node1.Id)
	})

	isValidNode(suite.T(), node1, node1FromStore, []string{"ingress", "overlayID1"})

	// Validate that a LB IP address is not allocated for node-local networks
	p := &api.Network{
		Id: "bridge",
		Spec: &api.NetworkSpec{
			Annotations: &api.Annotations{
				Name: "pred_bridge_network",
				Labels: map[string]string{
					"com.docker.swarm.predefined": "true",
				},
			},
			DriverConfig: &api.Driver{Name: "bridge"},
		},
	}
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.CreateNetwork(tx, p))
		return nil
	}))
	watchNetwork(suite.T(), netWatch, false, isValidNetwork) // bridge

	s.View(func(tx store.ReadTx) {
		node1FromStore = store.GetNode(tx, node1.Id)
	})

	isValidNode(suite.T(), node1, node1FromStore, []string{"ingress", "overlayID1"})
}

// TestNodeAttachmentOnLeadershipChange tests that a Node which is only partly
// allocated during a leadership change is correctly allocated afterward
func (suite *testSuite) TestNodeAttachmentOnLeadershipChange() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()

	a := suite.newAllocator(s)

	net1 := &api.Network{
		Id: "ingress",
		Spec: &api.NetworkSpec{
			Annotations: &api.Annotations{
				Name: "ingress",
			},
			Ingress: true,
		},
	}

	net2 := &api.Network{
		Id: "net2",
		Spec: &api.NetworkSpec{
			Annotations: &api.Annotations{
				Name: "net2",
			},
		},
	}

	node1 := &api.Node{
		Id: "node1",
	}

	task1 := &api.Task{
		Id:           "task1",
		NodeId:       node1.Id,
		DesiredState: api.TaskState_RUNNING,
		Spec:         &api.TaskSpec{},
	}

	// this task is not yet assigned. we will assign it to node1 after running
	// the allocator a 2nd time. we should create it now so that its network
	// attachments are allocated.
	task2 := &api.Task{
		Id:           "task2",
		DesiredState: api.TaskState_RUNNING,
		Spec: &api.TaskSpec{
			Networks: []*api.NetworkAttachmentConfig{
				{
					Target: "net2",
				},
			},
		},
	}

	// before starting the allocator, populate with these
	suite.NoError(s.Update(func(tx store.Tx) error {
		require.NoError(suite.T(), store.CreateNetwork(tx, net1))
		require.NoError(suite.T(), store.CreateNetwork(tx, net2))
		require.NoError(suite.T(), store.CreateNode(tx, node1))
		require.NoError(suite.T(), store.CreateTask(tx, task1))
		require.NoError(suite.T(), store.CreateTask(tx, task2))
		return nil
	}))

	// now start the allocator, let it allocate all of these objects, and then
	// stop it. it's easier to do this than to manually assign all of the
	// values

	nodeWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateNode{}, api.EventDeleteNode{})
	defer cancel()
	netWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateNetwork{}, api.EventDeleteNetwork{})
	defer cancel()
	taskWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateTask{})
	defer cancel()

	stop := suite.startAllocator(a)

	// validate that everything gets allocated
	watchNetwork(suite.T(), netWatch, false, isValidNetwork)
	watchNetwork(suite.T(), netWatch, false, isValidNetwork)
	watchNode(suite.T(), nodeWatch, false, isValidNode, node1, []string{"ingress"})
	watchTask(suite.T(), s, taskWatch, false, isValidTask)

	// once everything is created, go ahead and stop the allocator
	stop()

	// now update task2 to assign it to node1
	s.Update(func(tx store.Tx) error {
		task := store.GetTask(tx, task2.Id)
		require.NotNil(suite.T(), task)
		// make sure it has 1 network attachment
		suite.Len(task.Networks, 1)
		task.NodeId = node1.Id
		require.NoError(suite.T(), store.UpdateTask(tx, task))
		return nil
	})

	// and now we'll start a new allocator.
	a2 := suite.newAllocator(s)
	defer suite.startAllocator(a2)()

	// now we should see the node get allocated
	watchNode(suite.T(), nodeWatch, false, isValidNode, node1, []string{"ingress"})
	watchNode(suite.T(), nodeWatch, false, isValidNode, node1, []string{"ingress", "net2"})
}

func (suite *testSuite) TestAllocateServiceConflictingUserDefinedPorts() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()

	const svcID = "testID1"
	// Try adding some objects to store before allocator is started
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			Id: "ingress-nw-id",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "default-ingress",
				},
				Ingress: true,
			},
			Ipam: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "10.0.0.0/24",
						Gateway: "10.0.0.1",
					},
				},
			},
			DriverState: &api.Driver{},
		}
		suite.NoError(store.CreateNetwork(tx, in))

		s1 := &api.Service{
			Id: svcID,
			Spec: &api.ServiceSpec{
				Annotations: &api.Annotations{
					Name: "service1",
				},
				Endpoint: &api.EndpointSpec{
					Ports: []*api.PortConfig{
						{
							Name:          "some_tcp",
							TargetPort:    1234,
							PublishedPort: 1234,
						},
						{
							Name:          "some_other_tcp",
							TargetPort:    1234,
							PublishedPort: 1234,
						},
					},
				},
			},
		}
		suite.NoError(store.CreateService(tx, s1))

		return nil
	}))

	serviceWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateService{}, api.EventDeleteService{})
	defer cancel()

	a := suite.newAllocator(s)
	defer suite.startAllocator(a)()

	// Port spec is invalid; service should not be updated
	watchService(suite.T(), serviceWatch, true, func(_ assert.TestingT, service *api.Service) bool {
		suite.T().Errorf("unexpected service update: %v", service)
		return true
	})

	// Update the service to remove the conflicting port
	suite.NoError(s.Update(func(tx store.Tx) error {
		s1 := store.GetService(tx, svcID)
		if suite.NotNil(s1) {
			s1.Spec.GetEndpoint().GetPorts()[1].TargetPort = 1235
			s1.Spec.GetEndpoint().GetPorts()[1].PublishedPort = 1235
			suite.NoError(store.UpdateService(tx, s1))
		}
		return nil
	}))
	watchService(suite.T(), serviceWatch, false, func(t assert.TestingT, service *api.Service) bool {
		if assert.Equal(t, svcID, service.Id) && assert.NotNil(t, service.Endpoint) && assert.Len(t, service.Endpoint.Ports, 2) {
			return assert.Equal(t, uint32(1235), service.Endpoint.Ports[1].PublishedPort)
		}
		return false
	})
}

func (suite *testSuite) TestDeallocateServiceAllocate() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()

	newSvc := func(id string) *api.Service {
		return &api.Service{
			Id: id,
			Spec: &api.ServiceSpec{
				Annotations: &api.Annotations{
					Name: "service1",
				},
				Endpoint: &api.EndpointSpec{
					Ports: []*api.PortConfig{
						{
							Name:          "some_tcp",
							TargetPort:    1234,
							PublishedPort: 1234,
						},
					},
				},
			},
		}
	}

	// Try adding some objects to store before allocator is started
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			Id: "ingress-nw-id",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "default-ingress",
				},
				Ingress: true,
			},
			Ipam: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "10.0.0.0/24",
						Gateway: "10.0.0.1",
					},
				},
			},
			DriverState: &api.Driver{},
		}
		suite.NoError(store.CreateNetwork(tx, in))
		suite.NoError(store.CreateService(tx, newSvc("testID1")))
		return nil
	}))

	serviceWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateService{}, api.EventDeleteService{})
	defer cancel()

	a := suite.newAllocator(s)
	defer suite.startAllocator(a)()

	isTestService := func(id string) func(t assert.TestingT, service *api.Service) bool {
		return func(t assert.TestingT, service *api.Service) bool {
			return assert.Equal(t, id, service.Id) &&
				assert.Len(t, service.Endpoint.Ports, 1) &&
				assert.Equal(t, uint32(1234), service.Endpoint.Ports[0].PublishedPort) &&
				assert.Len(t, service.Endpoint.VirtualIps, 1)
		}
	}
	// Confirm service is allocated
	watchService(suite.T(), serviceWatch, false, isTestService("testID1"))

	// Deallocate the service and allocate a new one with the same port spec
	suite.NoError(s.Update(func(tx store.Tx) error {
		suite.NoError(store.DeleteService(tx, "testID1"))
		suite.NoError(store.CreateService(tx, newSvc("testID2")))
		return nil
	}))
	// Confirm new service is allocated
	watchService(suite.T(), serviceWatch, false, isTestService("testID2"))
}

func (suite *testSuite) TestServiceAddRemovePorts() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()

	const svcID = "testID1"
	// Try adding some objects to store before allocator is started
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			Id: "ingress-nw-id",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "default-ingress",
				},
				Ingress: true,
			},
			Ipam: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "10.0.0.0/24",
						Gateway: "10.0.0.1",
					},
				},
			},
			DriverState: &api.Driver{},
		}
		suite.NoError(store.CreateNetwork(tx, in))

		s1 := &api.Service{
			Id: svcID,
			Spec: &api.ServiceSpec{
				Annotations: &api.Annotations{
					Name: "service1",
				},
				Endpoint: &api.EndpointSpec{
					Ports: []*api.PortConfig{
						{
							Name:          "some_tcp",
							TargetPort:    1234,
							PublishedPort: 1234,
						},
					},
				},
			},
		}
		suite.NoError(store.CreateService(tx, s1))

		return nil
	}))

	serviceWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateService{}, api.EventDeleteService{})
	defer cancel()

	a := suite.newAllocator(s)
	defer suite.startAllocator(a)()

	var probedVIP string
	probeTestService := func(expectPorts ...uint32) func(t assert.TestingT, service *api.Service) bool {
		return func(t assert.TestingT, service *api.Service) bool {
			expectedVIPCount := 0
			if len(expectPorts) > 0 {
				expectedVIPCount = 1
			}
			if len(service.Endpoint.VirtualIps) > 0 {
				probedVIP = service.Endpoint.VirtualIps[0].Addr
			} else {
				probedVIP = ""
			}
			if assert.Equal(t, svcID, service.Id) && assert.Len(t, service.Endpoint.Ports, len(expectPorts)) {
				var published []uint32
				for _, port := range service.Endpoint.Ports {
					published = append(published, port.PublishedPort)
				}
				return assert.Equal(t, expectPorts, published) && assert.Len(t, service.Endpoint.VirtualIps, expectedVIPCount)
			}

			return false
		}
	}
	// Confirm service is allocated
	watchService(suite.T(), serviceWatch, false, probeTestService(1234))
	allocatedVIP := probedVIP

	// Unpublish port
	suite.NoError(s.Update(func(tx store.Tx) error {
		s1 := store.GetService(tx, svcID)
		if suite.NotNil(s1) {
			s1.Spec.Endpoint.Ports = nil
			suite.NoError(store.UpdateService(tx, s1))
		}
		return nil
	}))
	// Wait for unpublishing to take effect
	watchService(suite.T(), serviceWatch, false, probeTestService())

	// Publish port again and ensure VIP is not the same that was deallocated.
	// Since IP allocation is serial we should receive the next available IP.
	suite.NoError(s.Update(func(tx store.Tx) error {
		s1 := store.GetService(tx, svcID)
		if suite.NotNil(s1) {
			s1.Spec.Endpoint.Ports = append(s1.Spec.GetEndpoint().GetPorts(), &api.PortConfig{Name: "some_tcp",
				TargetPort:    1234,
				PublishedPort: 1234,
			})
			suite.NoError(store.UpdateService(tx, s1))
		}
		return nil
	}))
	watchService(suite.T(), serviceWatch, false, probeTestService(1234))
	suite.NotEqual(allocatedVIP, probedVIP)
}

func (suite *testSuite) TestServiceUpdatePort() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()

	const svcID = "testID1"
	// Try adding some objects to store before allocator is started
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			Id: "ingress-nw-id",
			Spec: &api.NetworkSpec{
				Annotations: &api.Annotations{
					Name: "default-ingress",
				},
				Ingress: true,
			},
			Ipam: &api.IPAMOptions{
				Driver: &api.Driver{},
				Configs: []*api.IPAMConfig{
					{
						Subnet:  "10.0.0.0/24",
						Gateway: "10.0.0.1",
					},
				},
			},
			DriverState: &api.Driver{},
		}
		suite.NoError(store.CreateNetwork(tx, in))

		s1 := &api.Service{
			Id: svcID,
			Spec: &api.ServiceSpec{
				Annotations: &api.Annotations{
					Name: "service1",
				},
				Endpoint: &api.EndpointSpec{
					Ports: []*api.PortConfig{
						{
							Name:          "some_tcp",
							TargetPort:    1234,
							PublishedPort: 1234,
						},
						{
							Name:       "some_other_tcp",
							TargetPort: 1235,
						},
					},
				},
			},
		}
		suite.NoError(store.CreateService(tx, s1))

		return nil
	}))

	serviceWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateService{}, api.EventDeleteService{})
	defer cancel()

	a := suite.newAllocator(s)
	defer suite.startAllocator(a)()

	watchService(suite.T(), serviceWatch, false, func(t assert.TestingT, service *api.Service) bool {
		return assert.Equal(t, svcID, service.Id) && assert.Len(t, service.Endpoint.Ports, 2)
	})

	suite.NoError(s.Update(func(tx store.Tx) error {
		s1 := store.GetService(tx, svcID)
		if suite.NotNil(s1) {
			s1.Spec.GetEndpoint().GetPorts()[1].PublishedPort = 1235
			suite.NoError(store.UpdateService(tx, s1))
		}
		return nil
	}))
	watchService(suite.T(), serviceWatch, false, func(t assert.TestingT, service *api.Service) bool {
		if assert.Equal(t, svcID, service.Id) && assert.Len(t, service.Endpoint.Ports, 2) {
			return assert.Equal(t, uint32(1235), service.Endpoint.Ports[1].PublishedPort)
		}
		return false
	})
}

func (suite *testSuite) TestServicePortAllocationIsRepeatable() {
	alloc := func() []*api.PortConfig {
		s := store.NewMemoryStore(nil)
		suite.NotNil(s)
		defer s.Close()

		const svcID = "testID1"
		// Try adding some objects to store before allocator is started
		suite.NoError(s.Update(func(tx store.Tx) error {
			// populate ingress network
			in := &api.Network{
				Id: "ingress-nw-id",
				Spec: &api.NetworkSpec{
					Annotations: &api.Annotations{
						Name: "default-ingress",
					},
					Ingress: true,
				},
				Ipam: &api.IPAMOptions{
					Driver: &api.Driver{},
					Configs: []*api.IPAMConfig{
						{
							Subnet:  "10.0.0.0/24",
							Gateway: "10.0.0.1",
						},
					},
				},
				DriverState: &api.Driver{},
			}
			suite.NoError(store.CreateNetwork(tx, in))

			s1 := &api.Service{
				Id: svcID,
				Spec: &api.ServiceSpec{
					Annotations: &api.Annotations{
						Name: "service1",
					},
					Endpoint: &api.EndpointSpec{
						Ports: []*api.PortConfig{
							{
								Name:          "some_tcp",
								TargetPort:    1234,
								PublishedPort: 1234,
							},
							{
								Name:       "some_other_tcp",
								TargetPort: 1235,
							},
						},
					},
				},
			}
			suite.NoError(store.CreateService(tx, s1))

			return nil
		}))

		serviceWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateService{}, api.EventDeleteService{})
		defer cancel()

		a := suite.newAllocator(s)
		defer suite.startAllocator(a)()

		var probedPorts []*api.PortConfig
		probeTestService := func(t assert.TestingT, service *api.Service) bool {
			if assert.Equal(t, svcID, service.Id) && assert.Len(t, service.Endpoint.Ports, 2) {
				probedPorts = service.Endpoint.Ports
				return true
			}
			return false
		}
		watchService(suite.T(), serviceWatch, false, probeTestService)
		return probedPorts
	}

	suite.Equal(alloc(), alloc())
}

func isValidNode(t assert.TestingT, originalNode, updatedNode *api.Node, networks []string) bool {

	if !assert.Equal(t, originalNode.Id, updatedNode.Id) {
		return false
	}

	if !assert.Equal(t, len(updatedNode.Attachments), len(networks)) {
		return false
	}

	for _, na := range updatedNode.Attachments {
		if !assert.Equal(t, len(na.Addresses), 1) {
			return false
		}
	}

	return true
}

func isValidNetwork(t assert.TestingT, n *api.Network) bool {
	if _, ok := n.GetSpec().GetAnnotations().GetLabels()["com.docker.swarm.predefined"]; ok {
		return true
	}
	return assert.NotEqual(t, n.Ipam.Configs, nil) &&
		assert.Equal(t, len(n.Ipam.Configs), 1) &&
		assert.Equal(t, n.Ipam.Configs[0].Range, "") &&
		assert.Equal(t, len(n.Ipam.Configs[0].Reserved), 0) &&
		isValidSubnet(t, n.Ipam.Configs[0].Subnet) &&
		assert.NotEqual(t, net.ParseIP(n.Ipam.Configs[0].Gateway), nil)
}

func isValidTask(t assert.TestingT, s *store.MemoryStore, task *api.Task) bool {
	return isValidNetworkAttachment(t, task) &&
		isValidEndpoint(t, s, task) &&
		assert.Equal(t, task.Status.GetState(), api.TaskState_PENDING)
}

func isValidNetworkAttachment(t assert.TestingT, task *api.Task) bool {
	if len(task.Networks) != 0 {
		return assert.Equal(t, len(task.Networks[0].Addresses), 1) &&
			isValidSubnet(t, task.Networks[0].Addresses[0])
	}

	return true
}

func isValidEndpoint(t assert.TestingT, s *store.MemoryStore, task *api.Task) bool {
	if task.ServiceId != "" {
		var service *api.Service
		s.View(func(tx store.ReadTx) {
			service = store.GetService(tx, task.ServiceId)
		})

		if service == nil {
			return true
		}

		return assert.Equal(t, service.Endpoint, task.Endpoint)

	}

	return true
}

func isValidSubnet(t assert.TestingT, subnet string) bool {
	_, _, err := net.ParseCIDR(subnet)
	return assert.NoError(t, err)
}

type mockTester struct{}

func (m mockTester) Errorf(_ string, _ ...any) {
}

// Returns a timeout given whether we should expect a timeout:  In the case where we do expect a timeout,
// the timeout should be short, because it's not very useful to wait long amounts of time just in case
// an unexpected event comes in - a short timeout should catch an incorrect event at least often enough
// to make the test flaky and alert us to the problem. But in the cases where we don't expect a timeout,
// the timeout should be on the order of several seconds, so the test doesn't fail just because it's run
// on a relatively slow system, or there's a load spike.
func getWatchTimeout(expectTimeout bool) time.Duration {
	if expectTimeout {
		return 350 * time.Millisecond
	}
	return 5 * time.Second
}

func watchNode(t *testing.T, watch chan events.Event, expectTimeout bool,
	fn func(t assert.TestingT, originalNode, updatedNode *api.Node, networks []string) bool,
	originalNode *api.Node,
	networks []string) {
	for {

		var node *api.Node
		select {
		case event := <-watch:
			if n, ok := event.(api.EventUpdateNode); ok {
				node = n.Node.Copy()
				if fn == nil || fn(mockTester{}, originalNode, node, networks) {
					return
				}
			}

			if n, ok := event.(api.EventDeleteNode); ok {
				node = n.Node.Copy()
				if fn == nil || fn(mockTester{}, originalNode, node, networks) {
					return
				}
			}

		case <-time.After(getWatchTimeout(expectTimeout)):
			if !expectTimeout {
				fn(t, originalNode, node, networks)
				t.Fatal("timed out before watchNode found expected node state", string(debug.Stack()))
			}

			return
		}
	}
}

func watchNetwork(t *testing.T, watch chan events.Event, expectTimeout bool, fn func(t assert.TestingT, n *api.Network) bool) {
	for {
		var network *api.Network
		select {
		case event := <-watch:
			if n, ok := event.(api.EventUpdateNetwork); ok {
				network = n.Network.Copy()
				if fn == nil || fn(mockTester{}, network) {
					return
				}
			}

			if n, ok := event.(api.EventDeleteNetwork); ok {
				network = n.Network.Copy()
				if fn == nil || fn(mockTester{}, network) {
					return
				}
			}

		case <-time.After(getWatchTimeout(expectTimeout)):
			if !expectTimeout {
				fn(t, network)
				t.Fatal("timed out before watchNetwork found expected network state", string(debug.Stack()))
			}

			return
		}
	}
}

func watchService(t *testing.T, watch chan events.Event, expectTimeout bool, fn func(t assert.TestingT, n *api.Service) bool) {
	for {
		var service *api.Service
		select {
		case event := <-watch:
			if s, ok := event.(api.EventUpdateService); ok {
				service = s.Service.Copy()
				if fn == nil || fn(mockTester{}, service) {
					return
				}
			}

			if s, ok := event.(api.EventDeleteService); ok {
				service = s.Service.Copy()
				if fn == nil || fn(mockTester{}, service) {
					return
				}
			}

		case <-time.After(getWatchTimeout(expectTimeout)):
			if !expectTimeout {
				fn(t, service)
				t.Fatalf("timed out before watchService found expected service state\n stack = %s", string(debug.Stack()))
			}

			return
		}
	}
}

func watchTask(t *testing.T, s *store.MemoryStore, watch chan events.Event, expectTimeout bool, fn func(t assert.TestingT, s *store.MemoryStore, n *api.Task) bool) {
	for {
		var task *api.Task
		select {
		case event := <-watch:
			if t, ok := event.(api.EventUpdateTask); ok {
				task = t.Task.Copy()
				if fn == nil || fn(mockTester{}, s, task) {
					return
				}
			}

			if t, ok := event.(api.EventDeleteTask); ok {
				task = t.Task.Copy()
				if fn == nil || fn(mockTester{}, s, task) {
					return
				}
			}

		case <-time.After(getWatchTimeout(expectTimeout)):
			if !expectTimeout {
				fn(t, s, task)
				t.Fatalf("timed out before watchTask found expected task state %s", debug.Stack())
			}

			return
		}
	}
}
