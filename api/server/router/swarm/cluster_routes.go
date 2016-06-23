package swarm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/server/httputils"
	basictypes "github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	types "github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
)

func (sr *swarmRouter) initCluster(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var req types.InitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}
	nodeID, err := sr.backend.Init(req)
	if err != nil {
		logrus.Errorf("Error initializing swarm: %v", err)
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, nodeID)
}

func (sr *swarmRouter) joinCluster(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var req types.JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}
	return sr.backend.Join(req)
}

func (sr *swarmRouter) leaveCluster(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	force := httputils.BoolValue(r, "force")
	return sr.backend.Leave(force)
}

func (sr *swarmRouter) inspectCluster(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	swarm, err := sr.backend.Inspect()
	if err != nil {
		logrus.Errorf("Error getting swarm: %v", err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, swarm)
}

func (sr *swarmRouter) updateCluster(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var swarm types.Spec
	if err := json.NewDecoder(r.Body).Decode(&swarm); err != nil {
		return err
	}

	rawVersion := r.URL.Query().Get("version")
	version, err := strconv.ParseUint(rawVersion, 10, 64)
	if err != nil {
		return fmt.Errorf("Invalid swarm version '%s': %s", rawVersion, err.Error())
	}

	if err := sr.backend.Update(version, swarm); err != nil {
		logrus.Errorf("Error configuring swarm: %v", err)
		return err
	}
	return nil
}

func (sr *swarmRouter) getServices(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	filter, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	services, err := sr.backend.GetServices(basictypes.ServiceListOptions{Filter: filter})
	if err != nil {
		logrus.Errorf("Error getting services: %v", err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, services)
}

func (sr *swarmRouter) getService(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	service, err := sr.backend.GetService(vars["id"])
	if err != nil {
		logrus.Errorf("Error getting service %s: %v", vars["id"], err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, service)
}

func (sr *swarmRouter) createService(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var service types.ServiceSpec
	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		return err
	}

	id, err := sr.backend.CreateService(service)
	if err != nil {
		logrus.Errorf("Error creating service %s: %v", id, err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusCreated, &basictypes.ServiceCreateResponse{
		ID: id,
	})
}

func (sr *swarmRouter) updateService(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var service types.ServiceSpec
	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		return err
	}

	rawVersion := r.URL.Query().Get("version")
	version, err := strconv.ParseUint(rawVersion, 10, 64)
	if err != nil {
		return fmt.Errorf("Invalid service version '%s': %s", rawVersion, err.Error())
	}

	if err := sr.backend.UpdateService(vars["id"], version, service); err != nil {
		logrus.Errorf("Error updating service %s: %v", vars["id"], err)
		return err
	}
	return nil
}

func (sr *swarmRouter) removeService(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := sr.backend.RemoveService(vars["id"]); err != nil {
		logrus.Errorf("Error removing service %s: %v", vars["id"], err)
		return err
	}
	return nil
}

func (sr *swarmRouter) getNodes(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	filter, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	nodes, err := sr.backend.GetNodes(basictypes.NodeListOptions{Filter: filter})
	if err != nil {
		logrus.Errorf("Error getting nodes: %v", err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, nodes)
}

func (sr *swarmRouter) getNode(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	node, err := sr.backend.GetNode(vars["id"])
	if err != nil {
		logrus.Errorf("Error getting node %s: %v", vars["id"], err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, node)
}

func (sr *swarmRouter) updateNode(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var node types.NodeSpec
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		return err
	}

	rawVersion := r.URL.Query().Get("version")
	version, err := strconv.ParseUint(rawVersion, 10, 64)
	if err != nil {
		return fmt.Errorf("Invalid node version '%s': %s", rawVersion, err.Error())
	}

	if err := sr.backend.UpdateNode(vars["id"], version, node); err != nil {
		logrus.Errorf("Error updating node %s: %v", vars["id"], err)
		return err
	}
	return nil
}

func (sr *swarmRouter) removeNode(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := sr.backend.RemoveNode(vars["id"]); err != nil {
		logrus.Errorf("Error removing node %s: %v", vars["id"], err)
		return err
	}
	return nil
}

func (sr *swarmRouter) getTasks(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	filter, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		return err
	}

	tasks, err := sr.backend.GetTasks(basictypes.TaskListOptions{Filter: filter})
	if err != nil {
		logrus.Errorf("Error getting tasks: %v", err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, tasks)
}

func (sr *swarmRouter) getTask(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	task, err := sr.backend.GetTask(vars["id"])
	if err != nil {
		logrus.Errorf("Error getting task %s: %v", vars["id"], err)
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, task)
}
