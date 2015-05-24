package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"testing"

	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/types"
)

const (
	bridgeNetType = "bridge"
	bridgeName    = "docker0"
)

func getEmptyGenericOption() map[string]interface{} {
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = options.Generic{}
	return genericOption
}

func i2s(i interface{}) string {
	s, ok := i.(string)
	if !ok {
		panic(fmt.Sprintf("Failed i2s for %v", i))
	}
	return s
}

func i2e(i interface{}) *endpointResource {
	s, ok := i.(*endpointResource)
	if !ok {
		panic(fmt.Sprintf("Failed i2e for %v", i))
	}
	return s
}

func i2c(i interface{}) *libnetwork.ContainerData {
	s, ok := i.(*libnetwork.ContainerData)
	if !ok {
		panic(fmt.Sprintf("Failed i2c for %v", i))
	}
	return s
}

func i2eL(i interface{}) []*endpointResource {
	s, ok := i.([]*endpointResource)
	if !ok {
		panic(fmt.Sprintf("Failed i2eL for %v", i))
	}
	return s
}

func i2n(i interface{}) *networkResource {
	s, ok := i.(*networkResource)
	if !ok {
		panic(fmt.Sprintf("Failed i2n for %v", i))
	}
	return s
}

func i2nL(i interface{}) []*networkResource {
	s, ok := i.([]*networkResource)
	if !ok {
		panic(fmt.Sprintf("Failed i2nL for %v", i))
	}
	return s
}

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

func TestJoinOptionParser(t *testing.T) {
	hn := "host1"
	dn := "docker.com"
	hp := "/etc/hosts"
	rc := "/etc/resolv.conf"
	dnss := []string{"8.8.8.8", "172.28.34.5"}
	ehs := []endpointExtraHost{endpointExtraHost{Name: "extra1", Address: "172.28.9.1"}, endpointExtraHost{Name: "extra2", Address: "172.28.9.2"}}
	pus := []endpointParentUpdate{endpointParentUpdate{EndpointID: "abc123def456", Name: "serv1", Address: "172.28.30.123"}}

	ej := endpointJoin{
		HostName:          hn,
		DomainName:        dn,
		HostsPath:         hp,
		ResolvConfPath:    rc,
		DNS:               dnss,
		ExtraHosts:        ehs,
		ParentUpdates:     pus,
		UseDefaultSandbox: true,
	}

	if len(ej.parseOptions()) != 10 {
		t.Fatalf("Failed to generate all libnetwork.EndpointJoinOption methods libnetwork.EndpointJoinOption method")
	}

}

func TestJson(t *testing.T) {
	nc := networkCreate{NetworkType: bridgeNetType}
	b, err := json.Marshal(nc)
	if err != nil {
		t.Fatal(err)
	}

	var ncp networkCreate
	err = json.Unmarshal(b, &ncp)
	if err != nil {
		t.Fatal(err)
	}

	if nc.NetworkType != ncp.NetworkType {
		t.Fatalf("Incorrect networkCreate after json encoding/deconding: %v", ncp)
	}

	jl := endpointJoin{ContainerID: "abcdef456789"}
	b, err = json.Marshal(jl)
	if err != nil {
		t.Fatal(err)
	}

	var jld endpointJoin
	err = json.Unmarshal(b, &jld)
	if err != nil {
		t.Fatal(err)
	}

	if jl.ContainerID != jld.ContainerID {
		t.Fatalf("Incorrect endpointJoin after json encoding/deconding: %v", jld)
	}
}

func TestCreateDeleteNetwork(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}
	err = c.ConfigureNetworkDriver(bridgeNetType, nil)
	if err != nil {
		t.Fatal(err)
	}

	badBody, err := json.Marshal("bad body")
	if err != nil {
		t.Fatal(err)
	}

	vars := make(map[string]string)
	_, errRsp := procCreateNetwork(c, nil, badBody)
	if errRsp == &createdResponse {
		t.Fatalf("Expected to fail but succeeded")
	}
	if errRsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected StatusBadRequest status code, got: %v", errRsp)
	}

	incompleteBody, err := json.Marshal(networkCreate{})
	if err != nil {
		t.Fatal(err)
	}

	_, errRsp = procCreateNetwork(c, vars, incompleteBody)
	if errRsp == &createdResponse {
		t.Fatalf("Expected to fail but succeeded")
	}
	if errRsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected StatusBadRequest status code, got: %v", errRsp)
	}

	ops := make(map[string]interface{})
	ops[netlabel.GenericData] = options.Generic{}
	nc := networkCreate{Name: "network_1", NetworkType: bridgeNetType, Options: ops}
	goodBody, err := json.Marshal(nc)
	if err != nil {
		t.Fatal(err)
	}

	_, errRsp = procCreateNetwork(c, vars, goodBody)
	if errRsp != &createdResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	vars[urlNwName] = ""
	_, errRsp = procDeleteNetwork(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected to fail but succeeded")
	}

	vars[urlNwName] = "abc"
	_, errRsp = procDeleteNetwork(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected to fail but succeeded")
	}

	vars[urlNwName] = "network_1"
	_, errRsp = procDeleteNetwork(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
}

func TestGetNetworksAndEndpoints(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}
	err = c.ConfigureNetworkDriver(bridgeNetType, nil)
	if err != nil {
		t.Fatal(err)
	}

	nc := networkCreate{Name: "sh", NetworkType: bridgeNetType}
	body, err := json.Marshal(nc)
	if err != nil {
		t.Fatal(err)
	}

	vars := make(map[string]string)
	inid, errRsp := procCreateNetwork(c, vars, body)
	if errRsp != &createdResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	nid, ok := inid.(string)
	if !ok {
		t.FailNow()
	}

	ec1 := endpointCreate{
		Name: "ep1",
		ExposedPorts: []types.TransportPort{
			types.TransportPort{Proto: types.TCP, Port: uint16(5000)},
			types.TransportPort{Proto: types.UDP, Port: uint16(400)},
			types.TransportPort{Proto: types.TCP, Port: uint16(600)},
		},
		PortMapping: []types.PortBinding{
			types.PortBinding{Proto: types.TCP, Port: uint16(230), HostPort: uint16(23000)},
			types.PortBinding{Proto: types.UDP, Port: uint16(200), HostPort: uint16(22000)},
			types.PortBinding{Proto: types.TCP, Port: uint16(120), HostPort: uint16(12000)},
		},
	}
	b1, err := json.Marshal(ec1)
	if err != nil {
		t.Fatal(err)
	}
	ec2 := endpointCreate{Name: "ep2"}
	b2, err := json.Marshal(ec2)
	if err != nil {
		t.Fatal(err)
	}

	vars[urlNwName] = "sh"
	vars[urlEpName] = "ep1"
	ieid1, errRsp := procCreateEndpoint(c, vars, b1)
	if errRsp != &createdResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	eid1 := i2s(ieid1)
	vars[urlEpName] = "ep2"
	ieid2, errRsp := procCreateEndpoint(c, vars, b2)
	if errRsp != &createdResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	eid2 := i2s(ieid2)

	vars[urlNwName] = ""
	vars[urlEpName] = "ep1"
	_, errRsp = procGetEndpoint(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure but succeeded: %v", errRsp)
	}
	if errRsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected to fail with http.StatusBadRequest, but got: %d", errRsp.StatusCode)
	}

	vars = make(map[string]string)
	vars[urlNwName] = "sh"
	vars[urlEpID] = ""
	_, errRsp = procGetEndpoint(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure but succeeded: %v", errRsp)
	}
	if errRsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected to fail with http.StatusBadRequest, but got: %d", errRsp.StatusCode)
	}

	vars = make(map[string]string)
	vars[urlNwID] = ""
	vars[urlEpID] = eid1
	_, errRsp = procGetEndpoint(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure but succeeded: %v", errRsp)
	}
	if errRsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected to fail with http.StatusBadRequest, but got: %d", errRsp.StatusCode)
	}

	// nw by name and ep by id
	vars[urlNwName] = "sh"
	i1, errRsp := procGetEndpoint(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	// nw by name and ep by name
	delete(vars, urlEpID)
	vars[urlEpName] = "ep1"
	i2, errRsp := procGetEndpoint(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	// nw by id and ep by name
	delete(vars, urlNwName)
	vars[urlNwID] = nid
	i3, errRsp := procGetEndpoint(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	// nw by id and ep by id
	delete(vars, urlEpName)
	vars[urlEpID] = eid1
	i4, errRsp := procGetEndpoint(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	id1 := i2e(i1).ID
	if id1 != i2e(i2).ID || id1 != i2e(i3).ID || id1 != i2e(i4).ID {
		t.Fatalf("Endpoints retireved via different query parameters differ: %v, %v, %v, %v", i1, i2, i3, i4)
	}

	vars[urlNwName] = ""
	_, errRsp = procGetEndpoints(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	delete(vars, urlNwName)
	vars[urlNwID] = "fakeID"
	_, errRsp = procGetEndpoints(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	vars[urlNwID] = nid
	_, errRsp = procGetEndpoints(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	vars[urlNwName] = "sh"
	iepList, errRsp := procGetEndpoints(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	epList := i2eL(iepList)
	if len(epList) != 2 {
		t.Fatalf("Did not return the expected number (2) of endpoint resources: %d", len(epList))
	}
	if "sh" != epList[0].Network || "sh" != epList[1].Network {
		t.Fatalf("Did not find expected network name in endpoint resources")
	}

	vars = make(map[string]string)
	vars[urlNwName] = ""
	_, errRsp = procGetNetwork(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Exepected failure, got: %v", errRsp)
	}
	vars[urlNwName] = "shhhhh"
	_, errRsp = procGetNetwork(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Exepected failure, got: %v", errRsp)
	}
	vars[urlNwName] = "sh"
	inr1, errRsp := procGetNetwork(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	nr1 := i2n(inr1)

	delete(vars, urlNwName)
	vars[urlNwID] = "cacca"
	_, errRsp = procGetNetwork(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	vars[urlNwID] = nid
	inr2, errRsp := procGetNetwork(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("procgetNetworkByName() != procgetNetworkById(), %v vs %v", inr1, inr2)
	}
	nr2 := i2n(inr2)
	if nr1.Name != nr2.Name || nr1.Type != nr2.Type || nr1.ID != nr2.ID || len(nr1.Endpoints) != len(nr2.Endpoints) {
		t.Fatalf("Get by name and Get failure: %v", errRsp)
	}

	if len(nr1.Endpoints) != 2 {
		t.Fatalf("Did not find the expected number (2) of endpoint resources in the network resource: %d", len(nr1.Endpoints))
	}
	for _, er := range nr1.Endpoints {
		if er.ID != eid1 && er.ID != eid2 {
			t.Fatalf("Did not find the expected endpoint resources in the network resource: %v", nr1.Endpoints)
		}
	}

	iList, errRsp := procGetNetworks(c, nil, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	netList := i2nL(iList)
	if len(netList) != 1 {
		t.Fatalf("Did not return the expected number of network resources")
	}
	if nid != netList[0].ID {
		t.Fatalf("Did not find expected network %s: %v", nid, netList)
	}

	_, errRsp = procDeleteNetwork(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Exepected failure, got: %v", errRsp)
	}

	vars[urlEpName] = "ep1"
	_, errRsp = procDeleteEndpoint(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	delete(vars, urlEpName)
	iepList, errRsp = procGetEndpoints(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	epList = i2eL(iepList)
	if len(epList) != 1 {
		t.Fatalf("Did not return the expected number (1) of endpoint resources: %d", len(epList))
	}

	vars[urlEpName] = "ep2"
	_, errRsp = procDeleteEndpoint(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	iepList, errRsp = procGetEndpoints(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	epList = i2eL(iepList)
	if len(epList) != 0 {
		t.Fatalf("Did not return the expected number (0) of endpoint resources: %d", len(epList))
	}

	_, errRsp = procDeleteNetwork(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	iList, errRsp = procGetNetworks(c, nil, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	netList = i2nL(iList)
	if len(netList) != 0 {
		t.Fatalf("Did not return the expected number of network resources")
	}
}

func TestDetectGetNetworksInvalidQueryComposition(t *testing.T) {
	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}

	vars := map[string]string{urlNwName: "x", urlNwPID: "y"}
	_, errRsp := procGetNetworks(c, vars, nil)
	if errRsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected %d. Got: %v", http.StatusBadRequest, errRsp)
	}
}

func TestDetectGetEndpointsInvalidQueryComposition(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}
	err = c.ConfigureNetworkDriver(bridgeNetType, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.NewNetwork(bridgeNetType, "network", nil)
	if err != nil {
		t.Fatal(err)
	}

	vars := map[string]string{urlNwName: "network", urlEpName: "x", urlEpPID: "y"}
	_, errRsp := procGetEndpoints(c, vars, nil)
	if errRsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected %d. Got: %v", http.StatusBadRequest, errRsp)
	}
}

func TestFindNetworkUtil(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}
	err = c.ConfigureNetworkDriver(bridgeNetType, nil)
	if err != nil {
		t.Fatal(err)
	}

	nw, err := c.NewNetwork(bridgeNetType, "network", nil)
	if err != nil {
		t.Fatal(err)
	}
	nid := nw.ID()

	defer checkPanic(t)
	findNetwork(c, "", -1)

	_, errRsp := findNetwork(c, "", byName)
	if errRsp == &successResponse {
		t.Fatalf("Expected to fail but succeeded")
	}
	if errRsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected %d, but got: %d", http.StatusBadRequest, errRsp.StatusCode)
	}

	n, errRsp := findNetwork(c, nid, byID)
	if errRsp != &successResponse {
		t.Fatalf("Unexpected failure: %v", errRsp)
	}
	if n == nil {
		t.Fatalf("Unexpected nil libnetwork.Network")
	}
	if nid != n.ID() {
		t.Fatalf("Incorrect libnetwork.Network resource. It has different id: %v", n)
	}
	if "network" != n.Name() {
		t.Fatalf("Incorrect libnetwork.Network resource. It has different name: %v", n)
	}

	n, errRsp = findNetwork(c, "network", byName)
	if errRsp != &successResponse {
		t.Fatalf("Unexpected failure: %v", errRsp)
	}
	if n == nil {
		t.Fatalf("Unexpected nil libnetwork.Network")
	}
	if nid != n.ID() {
		t.Fatalf("Incorrect libnetwork.Network resource. It has different id: %v", n)
	}
	if "network" != n.Name() {
		t.Fatalf("Incorrect libnetwork.Network resource. It has different name: %v", n)
	}

	n.Delete()

	_, errRsp = findNetwork(c, nid, byID)
	if errRsp == &successResponse {
		t.Fatalf("Expected to fail but succeeded")
	}
	if errRsp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected %d, but got: %d", http.StatusNotFound, errRsp.StatusCode)
	}

	_, errRsp = findNetwork(c, "network", byName)
	if errRsp == &successResponse {
		t.Fatalf("Expected to fail but succeeded")
	}
	if errRsp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected %d, but got: %d", http.StatusNotFound, errRsp.StatusCode)
	}
}

func TestCreateDeleteEndpoints(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}
	err = c.ConfigureNetworkDriver(bridgeNetType, nil)
	if err != nil {
		t.Fatal(err)
	}

	nc := networkCreate{Name: "firstNet", NetworkType: bridgeNetType}
	body, err := json.Marshal(nc)
	if err != nil {
		t.Fatal(err)
	}

	vars := make(map[string]string)
	i, errRsp := procCreateNetwork(c, vars, body)
	if errRsp != &createdResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	nid := i2s(i)

	vbad, err := json.Marshal("bad endppoint create data")
	if err != nil {
		t.Fatal(err)
	}

	vars[urlNwName] = "firstNet"
	_, errRsp = procCreateEndpoint(c, vars, vbad)
	if errRsp == &createdResponse {
		t.Fatalf("Expected to fail but succeeded")
	}

	b, err := json.Marshal(endpointCreate{Name: ""})
	if err != nil {
		t.Fatal(err)
	}

	vars[urlNwName] = "secondNet"
	_, errRsp = procCreateEndpoint(c, vars, b)
	if errRsp == &createdResponse {
		t.Fatalf("Expected to fail but succeeded")
	}

	vars[urlNwName] = "firstNet"
	_, errRsp = procCreateEndpoint(c, vars, b)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure but succeeded: %v", errRsp)
	}

	b, err = json.Marshal(endpointCreate{Name: "firstEp"})
	if err != nil {
		t.Fatal(err)
	}

	i, errRsp = procCreateEndpoint(c, vars, b)
	if errRsp != &createdResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
	eid := i2s(i)

	_, errRsp = findEndpoint(c, "myNet", "firstEp", byName, byName)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure but succeeded: %v", errRsp)
	}

	ep0, errRsp := findEndpoint(c, nid, "firstEp", byID, byName)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	ep1, errRsp := findEndpoint(c, "firstNet", "firstEp", byName, byName)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	ep2, errRsp := findEndpoint(c, nid, eid, byID, byID)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	ep3, errRsp := findEndpoint(c, "firstNet", eid, byName, byID)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	if ep0.ID() != ep1.ID() || ep0.ID() != ep2.ID() || ep0.ID() != ep3.ID() {
		t.Fatalf("Diffenrent queries returned different endpoints: \nep0: %v\nep1: %v\nep2: %v\nep3: %v", ep0, ep1, ep2, ep3)
	}

	vars = make(map[string]string)
	vars[urlNwName] = ""
	vars[urlEpName] = "ep1"
	_, errRsp = procDeleteEndpoint(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	vars[urlNwName] = "firstNet"
	vars[urlEpName] = ""
	_, errRsp = procDeleteEndpoint(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	vars[urlEpName] = "ep2"
	_, errRsp = procDeleteEndpoint(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	vars[urlEpName] = "firstEp"
	_, errRsp = procDeleteEndpoint(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	_, errRsp = findEndpoint(c, "firstNet", "firstEp", byName, byName)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}
}

func TestJoinLeave(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}
	err = c.ConfigureNetworkDriver(bridgeNetType, nil)
	if err != nil {
		t.Fatal(err)
	}

	nb, err := json.Marshal(networkCreate{Name: "network", NetworkType: bridgeNetType})
	if err != nil {
		t.Fatal(err)
	}
	vars := make(map[string]string)
	_, errRsp := procCreateNetwork(c, vars, nb)
	if errRsp != &createdResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	eb, err := json.Marshal(endpointCreate{Name: "endpoint"})
	if err != nil {
		t.Fatal(err)
	}
	vars[urlNwName] = "network"
	_, errRsp = procCreateEndpoint(c, vars, eb)
	if errRsp != &createdResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	vbad, err := json.Marshal("bad data")
	if err != nil {
		t.Fatal(err)
	}
	_, errRsp = procJoinEndpoint(c, vars, vbad)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	vars[urlEpName] = "endpoint"
	bad, err := json.Marshal(endpointJoin{})
	if err != nil {
		t.Fatal(err)
	}
	_, errRsp = procJoinEndpoint(c, vars, bad)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	cid := "abcdefghi"
	jl := endpointJoin{ContainerID: cid}
	jlb, err := json.Marshal(jl)
	if err != nil {
		t.Fatal(err)
	}

	vars = make(map[string]string)
	vars[urlNwName] = ""
	vars[urlEpName] = ""
	_, errRsp = procJoinEndpoint(c, vars, jlb)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	vars[urlNwName] = "network"
	vars[urlEpName] = ""
	_, errRsp = procJoinEndpoint(c, vars, jlb)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	vars[urlEpName] = "epoint"
	_, errRsp = procJoinEndpoint(c, vars, jlb)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	vars[urlEpName] = "endpoint"
	cdi, errRsp := procJoinEndpoint(c, vars, jlb)
	if errRsp != &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	cd := i2c(cdi)
	if cd.SandboxKey == "" {
		t.Fatalf("Empty sandbox key")
	}
	_, errRsp = procDeleteEndpoint(c, vars, nil)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	vars[urlNwName] = "network2"
	_, errRsp = procLeaveEndpoint(c, vars, vbad)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}
	_, errRsp = procLeaveEndpoint(c, vars, bad)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}
	_, errRsp = procLeaveEndpoint(c, vars, jlb)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}
	vars = make(map[string]string)
	vars[urlNwName] = ""
	vars[urlEpName] = ""
	_, errRsp = procLeaveEndpoint(c, vars, jlb)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}
	vars[urlNwName] = "network"
	vars[urlEpName] = ""
	_, errRsp = procLeaveEndpoint(c, vars, jlb)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}
	vars[urlEpName] = "2epoint"
	_, errRsp = procLeaveEndpoint(c, vars, jlb)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}
	vars[urlEpName] = "epoint"
	vars[urlCnID] = "who"
	_, errRsp = procLeaveEndpoint(c, vars, jlb)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	delete(vars, urlCnID)
	vars[urlEpName] = "endpoint"
	_, errRsp = procLeaveEndpoint(c, vars, jlb)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	vars[urlCnID] = cid
	_, errRsp = procLeaveEndpoint(c, vars, jlb)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	_, errRsp = procLeaveEndpoint(c, vars, jlb)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, got: %v", errRsp)
	}

	_, errRsp = procDeleteEndpoint(c, vars, nil)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}
}

func TestFindEndpointUtil(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}
	err = c.ConfigureNetworkDriver(bridgeNetType, nil)
	if err != nil {
		t.Fatal(err)
	}

	nw, err := c.NewNetwork(bridgeNetType, "second", nil)
	if err != nil {
		t.Fatal(err)
	}
	nid := nw.ID()

	ep, err := nw.CreateEndpoint("secondEp", nil)
	if err != nil {
		t.Fatal(err)
	}
	eid := ep.ID()

	defer checkPanic(t)
	findEndpoint(c, nid, "", byID, -1)

	_, errRsp := findEndpoint(c, nid, "", byID, byName)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, but got: %v", errRsp)
	}
	if errRsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected %d, but got: %d", http.StatusBadRequest, errRsp.StatusCode)
	}

	ep0, errRsp := findEndpoint(c, nid, "secondEp", byID, byName)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	ep1, errRsp := findEndpoint(c, "second", "secondEp", byName, byName)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	ep2, errRsp := findEndpoint(c, nid, eid, byID, byID)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	ep3, errRsp := findEndpoint(c, "second", eid, byName, byID)
	if errRsp != &successResponse {
		t.Fatalf("Unexepected failure: %v", errRsp)
	}

	if ep0 != ep1 || ep0 != ep2 || ep0 != ep3 {
		t.Fatalf("Diffenrent queries returned different endpoints")
	}

	ep.Delete()

	_, errRsp = findEndpoint(c, nid, "secondEp", byID, byName)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, but got: %v", errRsp)
	}
	if errRsp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected %d, but got: %d", http.StatusNotFound, errRsp.StatusCode)
	}

	_, errRsp = findEndpoint(c, "second", "secondEp", byName, byName)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, but got: %v", errRsp)
	}
	if errRsp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected %d, but got: %d", http.StatusNotFound, errRsp.StatusCode)
	}

	_, errRsp = findEndpoint(c, nid, eid, byID, byID)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, but got: %v", errRsp)
	}
	if errRsp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected %d, but got: %d", http.StatusNotFound, errRsp.StatusCode)
	}

	_, errRsp = findEndpoint(c, "second", eid, byName, byID)
	if errRsp == &successResponse {
		t.Fatalf("Expected failure, but got: %v", errRsp)
	}
	if errRsp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected %d, but got: %d", http.StatusNotFound, errRsp.StatusCode)
	}
}

func checkPanic(t *testing.T) {
	if r := recover(); r != nil {
		if _, ok := r.(runtime.Error); ok {
			panic(r)
		}
	} else {
		t.Fatalf("Expected to panic, but suceeded")
	}
}

func TestDetectNetworkTargetPanic(t *testing.T) {
	defer checkPanic(t)
	vars := make(map[string]string)
	detectNetworkTarget(vars)
}

func TestDetectEndpointTargetPanic(t *testing.T) {
	defer checkPanic(t)
	vars := make(map[string]string)
	detectEndpointTarget(vars)
}

func TestResponseStatus(t *testing.T) {
	list := []int{
		http.StatusBadGateway,
		http.StatusBadRequest,
		http.StatusConflict,
		http.StatusContinue,
		http.StatusExpectationFailed,
		http.StatusForbidden,
		http.StatusFound,
		http.StatusGatewayTimeout,
		http.StatusGone,
		http.StatusHTTPVersionNotSupported,
		http.StatusInternalServerError,
		http.StatusLengthRequired,
		http.StatusMethodNotAllowed,
		http.StatusMovedPermanently,
		http.StatusMultipleChoices,
		http.StatusNoContent,
		http.StatusNonAuthoritativeInfo,
		http.StatusNotAcceptable,
		http.StatusNotFound,
		http.StatusNotModified,
		http.StatusPartialContent,
		http.StatusPaymentRequired,
		http.StatusPreconditionFailed,
		http.StatusProxyAuthRequired,
		http.StatusRequestEntityTooLarge,
		http.StatusRequestTimeout,
		http.StatusRequestURITooLong,
		http.StatusRequestedRangeNotSatisfiable,
		http.StatusResetContent,
		http.StatusServiceUnavailable,
		http.StatusSwitchingProtocols,
		http.StatusTemporaryRedirect,
		http.StatusUnauthorized,
		http.StatusUnsupportedMediaType,
		http.StatusUseProxy,
	}
	for _, c := range list {
		r := responseStatus{StatusCode: c}
		if r.isOK() {
			t.Fatalf("isOK() returned true for code% d", c)
		}
	}

	r := responseStatus{StatusCode: http.StatusOK}
	if !r.isOK() {
		t.Fatalf("isOK() failed")
	}

	r = responseStatus{StatusCode: http.StatusCreated}
	if !r.isOK() {
		t.Fatalf("isOK() failed")
	}
}

// Local structs for end to end testing of api.go
type localReader struct {
	data  []byte
	beBad bool
}

func newLocalReader(data []byte) *localReader {
	lr := &localReader{data: make([]byte, len(data))}
	copy(lr.data, data)
	return lr
}

func (l *localReader) Read(p []byte) (n int, err error) {
	if l.beBad {
		return 0, errors.New("I am a bad reader")
	}
	if p == nil {
		return -1, fmt.Errorf("nil buffer passed")
	}
	if l.data == nil || len(l.data) == 0 {
		return 0, io.EOF
	}
	copy(p[:], l.data[:])
	return len(l.data), io.EOF
}

type localResponseWriter struct {
	body       []byte
	statusCode int
}

func newWriter() *localResponseWriter {
	return &localResponseWriter{}
}

func (f *localResponseWriter) Header() http.Header {
	return make(map[string][]string, 0)
}

func (f *localResponseWriter) Write(data []byte) (int, error) {
	if data == nil {
		return -1, fmt.Errorf("nil data passed")
	}

	f.body = make([]byte, len(data))
	copy(f.body, data)

	return len(f.body), nil
}

func (f *localResponseWriter) WriteHeader(c int) {
	f.statusCode = c
}

func TestwriteJSON(t *testing.T) {
	testCode := 55
	testData, err := json.Marshal("test data")
	if err != nil {
		t.Fatal(err)
	}

	rsp := newWriter()
	writeJSON(rsp, testCode, testData)
	if rsp.statusCode != testCode {
		t.Fatalf("writeJSON() failed to set the status code. Expected %d. Got %d", testCode, rsp.statusCode)
	}
	if !bytes.Equal(testData, rsp.body) {
		t.Fatalf("writeJSON() failed to set the body. Expected %s. Got %s", testData, rsp.body)
	}

}

func TestHttpHandlerUninit(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}

	h := &httpHandler{c: c}
	h.initRouter()
	if h.r == nil {
		t.Fatalf("initRouter() did not initialize the router")
	}

	rsp := newWriter()
	req, err := http.NewRequest("GET", "/v1.19/networks", nil)
	if err != nil {
		t.Fatal(err)
	}

	handleRequest := NewHTTPHandler(nil)
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusServiceUnavailable {
		t.Fatalf("Expected (%d). Got (%d): %s", http.StatusServiceUnavailable, rsp.statusCode, rsp.body)
	}

	handleRequest = NewHTTPHandler(c)

	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Expected (%d). Got: (%d): %s", http.StatusOK, rsp.statusCode, rsp.body)
	}

	var list []*networkResource
	err = json.Unmarshal(rsp.body, &list)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("Expected empty list. Got %v", list)
	}

	n, err := c.NewNetwork(bridgeNetType, "didietro", nil)
	if err != nil {
		t.Fatal(err)
	}
	nwr := buildNetworkResource(n)
	expected, err := json.Marshal([]*networkResource{nwr})
	if err != nil {
		t.Fatal(err)
	}

	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Unexpectded failure: (%d): %s", rsp.statusCode, rsp.body)
	}
	if len(rsp.body) == 0 {
		t.Fatalf("Empty list of networks")
	}
	if bytes.Equal(rsp.body, expected) {
		t.Fatalf("Incorrect list of networks in response's body")
	}
}

func TestHttpHandlerBadBody(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	rsp := newWriter()

	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}
	handleRequest := NewHTTPHandler(c)

	req, err := http.NewRequest("POST", "/v1.19/networks", &localReader{beBad: true})
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusBadRequest {
		t.Fatalf("Unexpected status code. Expected (%d). Got (%d): %s.", http.StatusBadRequest, rsp.statusCode, string(rsp.body))
	}

	body := []byte{}
	lr := newLocalReader(body)
	req, err = http.NewRequest("POST", "/v1.19/networks", lr)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusBadRequest {
		t.Fatalf("Unexpected status code. Expected (%d). Got (%d): %s.", http.StatusBadRequest, rsp.statusCode, string(rsp.body))
	}
}

func TestEndToEnd(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	rsp := newWriter()

	c, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}
	handleRequest := NewHTTPHandler(c)

	// Create network
	nc := networkCreate{Name: "network-fiftyfive", NetworkType: bridgeNetType}
	body, err := json.Marshal(nc)
	if err != nil {
		t.Fatal(err)
	}
	lr := newLocalReader(body)
	req, err := http.NewRequest("POST", "/v1.19/networks", lr)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusCreated {
		t.Fatalf("Unexpectded status code. Expected (%d). Got (%d): %s.", http.StatusCreated, rsp.statusCode, string(rsp.body))
	}
	if len(rsp.body) == 0 {
		t.Fatalf("Empty response body")
	}

	var nid string
	err = json.Unmarshal(rsp.body, &nid)
	if err != nil {
		t.Fatal(err)
	}

	// Query networks collection
	req, err = http.NewRequest("GET", "/v1.19/networks", nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Expected StatusOK. Got (%d): %s", rsp.statusCode, rsp.body)
	}

	b0 := make([]byte, len(rsp.body))
	copy(b0, rsp.body)

	req, err = http.NewRequest("GET", "/v1.19/networks?name=network-fiftyfive", nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Expected StatusOK. Got (%d): %s", rsp.statusCode, rsp.body)
	}

	if !bytes.Equal(b0, rsp.body) {
		t.Fatalf("Expected same body from GET /networks and GET /networks?name=<nw> when only network <nw> exist.")
	}

	// Query network by name
	req, err = http.NewRequest("GET", "/v1.19/networks?name=culo", nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Expected StatusOK. Got (%d): %s", rsp.statusCode, rsp.body)
	}

	var list []*networkResource
	err = json.Unmarshal(rsp.body, &list)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("Expected empty list. Got %v", list)
	}

	req, err = http.NewRequest("GET", "/v1.19/networks?name=network-fiftyfive", nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Unexpectded failure: (%d): %s", rsp.statusCode, rsp.body)
	}

	err = json.Unmarshal(rsp.body, &list)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatalf("Expected non empty list")
	}
	if list[0].Name != "network-fiftyfive" || nid != list[0].ID {
		t.Fatalf("Incongruent resource found: %v", list[0])
	}

	// Query network by partial id
	chars := []byte(nid)
	partial := string(chars[0 : len(chars)/2])
	req, err = http.NewRequest("GET", "/v1.19/networks?partial-id="+partial, nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Unexpectded failure: (%d): %s", rsp.statusCode, rsp.body)
	}

	err = json.Unmarshal(rsp.body, &list)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatalf("Expected non empty list")
	}
	if list[0].Name != "network-fiftyfive" || nid != list[0].ID {
		t.Fatalf("Incongruent resource found: %v", list[0])
	}

	// Get network by id
	req, err = http.NewRequest("GET", "/v1.19/networks/"+nid, nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Unexpectded failure: (%d): %s", rsp.statusCode, rsp.body)
	}

	var nwr networkResource
	err = json.Unmarshal(rsp.body, &nwr)
	if err != nil {
		t.Fatal(err)
	}
	if nwr.Name != "network-fiftyfive" || nid != nwr.ID {
		t.Fatalf("Incongruent resource found: %v", nwr)
	}

	// Create endpoint
	eb, err := json.Marshal(endpointCreate{Name: "ep-TwentyTwo"})
	if err != nil {
		t.Fatal(err)
	}

	lr = newLocalReader(eb)
	req, err = http.NewRequest("POST", "/v1.19/networks/"+nid+"/endpoints", lr)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusCreated {
		t.Fatalf("Unexpectded status code. Expected (%d). Got (%d): %s.", http.StatusCreated, rsp.statusCode, string(rsp.body))
	}
	if len(rsp.body) == 0 {
		t.Fatalf("Empty response body")
	}

	var eid string
	err = json.Unmarshal(rsp.body, &eid)
	if err != nil {
		t.Fatal(err)
	}

	// Query endpoint(s)
	req, err = http.NewRequest("GET", "/v1.19/networks/"+nid+"/endpoints", nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Expected StatusOK. Got (%d): %s", rsp.statusCode, rsp.body)
	}

	req, err = http.NewRequest("GET", "/v1.19/networks/"+nid+"/endpoints?name=bla", nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Unexpectded failure: (%d): %s", rsp.statusCode, rsp.body)
	}
	var epList []*endpointResource
	err = json.Unmarshal(rsp.body, &epList)
	if err != nil {
		t.Fatal(err)
	}
	if len(epList) != 0 {
		t.Fatalf("Expected empty list. Got %v", epList)
	}

	// Query endpoint by name
	req, err = http.NewRequest("GET", "/v1.19/networks/"+nid+"/endpoints?name=ep-TwentyTwo", nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Unexpectded failure: (%d): %s", rsp.statusCode, rsp.body)
	}

	err = json.Unmarshal(rsp.body, &epList)
	if err != nil {
		t.Fatal(err)
	}
	if len(epList) == 0 {
		t.Fatalf("Empty response body")
	}
	if epList[0].Name != "ep-TwentyTwo" || eid != epList[0].ID {
		t.Fatalf("Incongruent resource found: %v", epList[0])
	}

	// Query endpoint by partial id
	chars = []byte(eid)
	partial = string(chars[0 : len(chars)/2])
	req, err = http.NewRequest("GET", "/v1.19/networks/"+nid+"/endpoints?partial-id="+partial, nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Unexpectded failure: (%d): %s", rsp.statusCode, rsp.body)
	}

	err = json.Unmarshal(rsp.body, &epList)
	if err != nil {
		t.Fatal(err)
	}
	if len(epList) == 0 {
		t.Fatalf("Empty response body")
	}
	if epList[0].Name != "ep-TwentyTwo" || eid != epList[0].ID {
		t.Fatalf("Incongruent resource found: %v", epList[0])
	}

	// Get endpoint by id
	req, err = http.NewRequest("GET", "/v1.19/networks/"+nid+"/endpoints/"+eid, nil)
	if err != nil {
		t.Fatal(err)
	}
	handleRequest(rsp, req)
	if rsp.statusCode != http.StatusOK {
		t.Fatalf("Unexpectded failure: (%d): %s", rsp.statusCode, rsp.body)
	}

	var epr endpointResource
	err = json.Unmarshal(rsp.body, &epr)
	if err != nil {
		t.Fatal(err)
	}
	if epr.Name != "ep-TwentyTwo" || epr.ID != eid {
		t.Fatalf("Incongruent resource found: %v", epr)
	}
}

type bre struct{}

func (b *bre) Error() string {
	return "I am a bad request error"
}
func (b *bre) BadRequest() {}

type nfe struct{}

func (n *nfe) Error() string {
	return "I am a not found error"
}
func (n *nfe) NotFound() {}

type forb struct{}

func (f *forb) Error() string {
	return "I am a bad request error"
}
func (f *forb) Forbidden() {}

type notimpl struct{}

func (nip *notimpl) Error() string {
	return "I am a not implemented error"
}
func (nip *notimpl) NotImplemented() {}

type inter struct{}

func (it *inter) Error() string {
	return "I am a internal error"
}
func (it *inter) Internal() {}

type tout struct{}

func (to *tout) Error() string {
	return "I am a timeout error"
}
func (to *tout) Timeout() {}

type noserv struct{}

func (nos *noserv) Error() string {
	return "I am a no service error"
}
func (nos *noserv) NoService() {}

type notclassified struct{}

func (noc *notclassified) Error() string {
	return "I am a non classified error"
}

func TestErrorConversion(t *testing.T) {
	if convertNetworkError(new(bre)).StatusCode != http.StatusBadRequest {
		t.Fatalf("Failed to recognize BadRequest error")
	}

	if convertNetworkError(new(nfe)).StatusCode != http.StatusNotFound {
		t.Fatalf("Failed to recognize NotFound error")
	}

	if convertNetworkError(new(forb)).StatusCode != http.StatusForbidden {
		t.Fatalf("Failed to recognize Forbidden error")
	}

	if convertNetworkError(new(notimpl)).StatusCode != http.StatusNotImplemented {
		t.Fatalf("Failed to recognize NotImplemented error")
	}

	if convertNetworkError(new(inter)).StatusCode != http.StatusInternalServerError {
		t.Fatalf("Failed to recognize Internal error")
	}

	if convertNetworkError(new(tout)).StatusCode != http.StatusRequestTimeout {
		t.Fatalf("Failed to recognize Timeout error")
	}

	if convertNetworkError(new(noserv)).StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("Failed to recognize No Service error")
	}

	if convertNetworkError(new(notclassified)).StatusCode != http.StatusInternalServerError {
		t.Fatalf("Failed to recognize not classified error as Internal error")
	}
}
