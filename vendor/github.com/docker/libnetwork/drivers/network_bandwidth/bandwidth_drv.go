package bandwidth_drv

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/discoverapi"
	"github.com/docker/libnetwork/driverapi"
	nettypes "github.com/docker/libnetwork/types"
)

const networkType = "bandwidth_drv"

type driver struct {
	network string
	sync.Mutex
}

/***********************/
/* sivaramaprasad ch */
/* (c) Copyright 2016 Hewlett Packard Enterprise Development LP */
/*  Interface {eth0}Rule{,rulecount,.,nw{egress}} */
/*
 * IN MEMORY it should be
 *            -  contaner1 rule
 * Nic[eth0] -  -  cont2rule
 *            -  cont3rule
 *
 * Nic[eth1] -  cont4rule
 *          -  cont5rule
 *           -  cont6rule
 *
 * only one rule per container
 *  hence this "nw map[string]NetworkBandwidth"
 */

//Global Per Nic Rule
type Nic struct {
	interfaceName string
}

var Major_Id uint32 = 16 //starts from 0x10
var HostNic map[Nic]Rules

//this type for nw map
//type Container string
//supposed to be declaring nw like this below nw map[Container]NetworkBandwidth
type Rules struct {
	interfaceSpeed   int32                       // Physical nic speed in mbps mega bits per sec
	totalAvailablebw int64                       //Total Available bandwidth in bits per second
	totalRemainingbw int64                       // Total Remaining bandwidth
	majorId          uint32                      //majorId starts 0x10
	ruleCount        uint32                      //used as minorId 0x1
	nw               map[string]NetworkBandwidth //map of Container name
	//allocate nw map only after adding element to HostNic Map

	lock *sync.RWMutex
}

//declared nw in the Rule Struct
//var nw map[Container]NetworkBandwidth

type NetworkBandwidth struct {
	//name       string //Container Name
	egressMin  int32 //Should be in int32 similar to TC
	egressMax  int32
	ingressMin int32
	ingressMax int32
	path       string //container net_cls path
	speedType  string
	//change the types
	srcIp   string
	dstIp   string
	srcPort string
	dstPort string

	majorClassId uint32
	minorClassId uint32
	classId      uint32
}

func (r *Rules) CreateBandwidth(create types.BandwidthCreateRequest, cgroupPath string, HostPort string) error {

	var tmp_Nic Nic
	var tmp_Rule Rules
	var tmp_nwb map[string]NetworkBandwidth //Rules.nw//NetworkBandwidth
	var ruleCount uint32
	var SrcPort, DstPort string
	var SrcIp, DstIp string
	var err error

	interfaceName := create.InterfaceName
	if interfaceName == "" {
		interfaceName, err = getDefaultInterfaceName()
		if err != nil {
			return err
		}
	}

	tmp_Nic = Nic{interfaceName}

	if HostPort != "" {
		/*srcPort,_ := strconv.ParseUint(HostPort,10,32)
		  rule.nw[rule.ruleCount].srcPort = uint(srcPort)
		*/
		/* Apply filter rules on both egress and ingress */
		SrcPort = HostPort
		DstPort = HostPort
	} else {
		SrcPort = ""
		DstPort = ""
	}

	//Adding Bandwidth rule for the first time for a given interface
	if tmp_rule, ok := HostNic[Nic{interfaceName}]; ok != true {

		// Returned speed of the interface will be in Mbps(Mega bits per sec)
		actualSpeed, err := getInterfaceSpeed(interfaceName)
		if err != nil {
			return err
		}
		//Since actualSpeed in mega bits per second,convert to bits per sec
		allocatedSpeed := int64(actualSpeed) * 1000 * 1000
		remainingSpeed := int64(actualSpeed) * 1000 * 1000

		ruleCount = 1 // combined majorId and ruleCount in net_cls.classid would be in hex 0x0100001

		classid_major, classid_minor, classid, err := tmp_rule.verifyAndSetCgroupClassid(create.Container, Major_Id, ruleCount, cgroupPath)

		if err != nil {
			return err
		}
		//Allocate NetworkBandiwth for a container
		tmp_nwb = make(map[string]NetworkBandwidth)

		tmp_nwb[create.Container] = NetworkBandwidth{create.EgressMin,
			create.EgressMax,
			create.IngressMin,
			create.IngressMax,
			cgroupPath,
			create.SpeedTypeIn,
			SrcIp,
			DstIp,
			SrcPort,
			DstPort,
			classid_major,
			classid_minor,
			classid}

		tmp_Rule = Rules{actualSpeed, allocatedSpeed, remainingSpeed, Major_Id, ruleCount, tmp_nwb, new(sync.RWMutex)}

		err = tmp_Rule.verifyAndAddTC(create, interfaceName, Major_Id, ruleCount)
		if err != nil {
			return err
		}
		// increase from 0x10
		Major_Id = Major_Id + 1

	} else {
		// serving second and later rule requests
		tmp_Rule = HostNic[Nic{interfaceName}]
		tmp_Rule.lock.Lock()
		defer tmp_Rule.lock.Unlock()

		if _, ok := tmp_rule.nw[create.Container]; ok == true {
			return fmt.Errorf("Bandwidth rule exists already for the container, remove it before updating")
		}

		ruleCount = tmp_Rule.ruleCount + 1
		major_id := tmp_Rule.majorId
		classid_major, classid_minor, classid, err := tmp_rule.verifyAndSetCgroupClassid(create.Container, major_id, ruleCount, cgroupPath)
		if err != nil {
			return err
		}

		tmp_rule.nw[create.Container] = NetworkBandwidth{create.EgressMin,
			create.EgressMax,
			create.IngressMin,
			create.IngressMax,
			cgroupPath,
			create.SpeedTypeIn,
			SrcIp,
			DstIp,
			SrcPort,
			DstPort,
			classid_major,
			classid_minor,
			classid}

		if remainingSpeed := tmp_Rule.totalRemainingbw; remainingSpeed <= 0 {
			return fmt.Errorf("Not Enough Bandwidth available to allocate")
		}

		err = tmp_Rule.verifyAndAddTC(create, interfaceName, major_id, ruleCount)
		if err != nil {
			return err
		}

	}
	tmp_Rule.ruleCount = ruleCount //if successful Increment ruleCount in Rules as well
	HostNic[tmp_Nic] = tmp_Rule
	fmt.Println("HostNic: ", HostNic)

	return nil
}

func (r *Rules) RemoveBandwidth(container string, interfaceName string) error {
	var err error

	if interfaceName == "" {
		interfaceName, err = getDefaultInterfaceName()
		if err != nil {
			return err
		}
	}

	if tmp_rule, ok := HostNic[Nic{interfaceName}]; ok != true {
		return fmt.Errorf("Interface: %s not added to Docker network bandwidth management", interfaceName)
	} else if _, ok := tmp_rule.nw[container]; ok == true {
		tmp_rule.lock.Lock()
		defer tmp_rule.lock.Unlock()
		//delete filter and class
		if err = tmp_rule.deleteFilter(container, interfaceName); err != nil {
			return err
		}
		if err = tmp_rule.deleteClass(container, interfaceName); err != nil {
			return err
		}
		nw := tmp_rule.nw[container]
		//Reclaim the speed
		if nw.speedType == "kbit" {
			tmp_rule.totalRemainingbw = tmp_rule.totalRemainingbw - int64(nw.egressMin)*1000
		} else if nw.speedType == "mbit" {
			tmp_rule.totalRemainingbw = tmp_rule.totalRemainingbw - int64(nw.egressMin)*1000*1000
		} else if nw.speedType == "gbit" {
			tmp_rule.totalRemainingbw = tmp_rule.totalRemainingbw - int64(nw.egressMin)*1000*1000*1000
		}

		f, err := os.OpenFile(nw.path+"/net_cls.classid", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			err = fmt.Errorf("Failed to reset net_cls.classid file : %s", err)
		}

		str := strconv.Itoa(int(0))
		io.WriteString(f, str)
		f.Close()

		delete(tmp_rule.nw, container)

		return err
	} else {
		return fmt.Errorf("No Bandwidth management rule present for this container: %s", container)
	}

}

func (r Rules) verifyAndSetCgroupClassid(name string, major_id uint32, ruleCount uint32, cgroupPath string) (uint32, uint32, uint32, error) {

	var classid_major, classid_minor, classid uint32

	classid_major = 0x0000000
	classid_minor = 0x0000000
	classid = 0
	if r.ruleCount > 998 {
		return classid_major, classid_minor, classid, nettypes.ForbiddenErrorf("Cannot add bandwidth rule beyond ruleCount:%d", r.ruleCount)
	}
	classid_major = classid_major + major_id
	classid_minor = classid_minor + ruleCount

	tmp_classid_major := classid_major << 16
	classid = tmp_classid_major | classid_minor

	f, err := os.OpenFile(cgroupPath+"/net_cls.classid", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		err = fmt.Errorf("Failed to open net_cls.classid file err: %s", err)
		return 0, 0, 0, err
	}

	str := strconv.Itoa(int(classid))
	io.WriteString(f, str)

	f.Close()
	//fmt.Println(" successfully set net_cls.classid  in verifyAndSetCgroupClassid() ", r)
	return classid_major, classid_minor, classid, nil
}

func (r Rules) verifyAndAddTC(create types.BandwidthCreateRequest, interfaceName string, majorId uint32, ruleCount uint32) error {

	tc_binary, lookErr := exec.LookPath("tc")
	if lookErr != nil {
		lookErr := fmt.Errorf("Failed to find binary TC, please install traffic classifier:%s", lookErr)
		return lookErr
	}

	//if This is the first rule, then add default qdisc type before adding any filter
	// otherwords it just removes old qdisc,class,filters and initializes interface with required qdisc
	//tc qdisc del dev em49 root
	tc_interface := interfaceName
	//getting NetworkBandwidth of container from Rules struct
	nw := r.nw[create.Container]

	//Check the speed caliculations in bits per sec
	var tmp_speed int64
	if nw.speedType == "kbit" {
		tmp_speed = int64(nw.egressMin) * 1000
	} else if nw.speedType == "mbit" {
		tmp_speed = int64(nw.egressMin) * 1000 * 1000
	} else if nw.speedType == "gbit" {
		tmp_speed = int64(nw.egressMin) * 1000 * 1000 * 1000
	}

	if tmp_speed < r.totalAvailablebw && r.totalRemainingbw-tmp_speed > 0 {
		r.totalRemainingbw = r.totalRemainingbw - tmp_speed
	} else {
		fmt.Println("Warning!! Trying to allocate n/w bandwidth more than available physical interface speed")
	}

	mjr_handle := fmt.Sprintf("%x:", majorId)
	//minor id is ruleCount here
	cgroup_filter_priority := fmt.Sprintf("%x", ruleCount)
	filter_priority := fmt.Sprintf("%x", ruleCount)
	minor_handle := fmt.Sprintf("%x:", ruleCount)
	cls_id := fmt.Sprintf("%s%x", mjr_handle, ruleCount)

	set_rate := fmt.Sprintf("%d%s", nw.egressMin, nw.speedType)
	set_ceil := fmt.Sprintf("%d%s", nw.egressMin, nw.speedType)

	if ruleCount == 1 {
		cmd := exec.Command(tc_binary, "qdisc", "del", "dev", tc_interface, "root")

		var tc_out bytes.Buffer
		var tc_err bytes.Buffer
		cmd.Stdout = &tc_out
		cmd.Stderr = &tc_err
		err := cmd.Run()
		if err != nil {
			fmt.Errorf("failed to delete qdisc using TC but still continuing", err, tc_err.String())
			//This is okay you can continue if err is
			//RTNETLINK answers: No such file or directory
		}

		//Add default qdisc root
		//tc qdisc add dev em49 parent root handle 1: htb default 99
		{
			cmd := exec.Command(tc_binary, "qdisc", "add", "dev", tc_interface, "parent", "root", "handle", mjr_handle, "htb", "default", "999")
			var tc_out bytes.Buffer
			var tc_err bytes.Buffer
			cmd.Stdout = &tc_out
			cmd.Stderr = &tc_err
			err := cmd.Run()
			if err != nil {
				err := fmt.Errorf("failed to add qdisc using TC,%s", err, tc_err.String())
				return err
			}

		}

		//fmt.Println("major id:, minor id: classid:  ruleCount: hostPort", nw.majorClassId, nw.minorClassId, nw.classId, ruleCount, nw.srcPort)

	}

	// add class for a container and filter
	//ex: tc class add dev em49 parent 1: classid 1:1 htb rate 100mbit burst 4000 ceil 150mbit prio 0
	{
		cmd := exec.Command(tc_binary, "class", "add", "dev", tc_interface, "parent", mjr_handle, "classid", cls_id, "htb",
			"rate", set_rate, "burst", "4000", "ceil", set_ceil, "prio", "0")

		var tc_out bytes.Buffer
		var tc_err bytes.Buffer
		cmd.Stdout = &tc_out
		cmd.Stderr = &tc_err
		err := cmd.Run()
		if err != nil {
			err := fmt.Errorf("failed to add  class using TC,%s", err, tc_err.String())
			return err
		}

	}
	//Add cgroup Filter
	if nw.srcPort == "" && nw.dstPort == "" {
		fmt.Println(" if  nw.srcPort ==  && nw.dstPort == ")
		//tc filter add dev em49 protocol all parent 10: prio 10 handle 1: cgroup
		{
			cmd := exec.Command(tc_binary, "filter", "add", "dev", tc_interface, "protocol", "all", "parent", mjr_handle,
				"prio", cgroup_filter_priority, "handle", minor_handle, "cgroup")

			var tc_out bytes.Buffer
			var tc_err bytes.Buffer
			cmd.Stdout = &tc_out
			cmd.Stderr = &tc_err
			err := cmd.Run()
			if err != nil {
				err := fmt.Errorf("failed to add cgroup filter using TC,%s", err, tc_err.String())
				return err
			}

		}
	} else {

		fmt.Println(" inside else nw.srcPort ==  && nw.dstPort == ")
		srcPort := nw.srcPort
		//tc filter add dev em49 protocol ip parent 1: prio 1 u32 match ip sport 45455  0xffff flowid 1:1
		{
			cmd := exec.Command(tc_binary, "filter", "add", "dev", tc_interface, "protocol", "ip", "parent", mjr_handle,
				"prio", filter_priority, "u32", "match", "ip", "sport", srcPort, "0xffff", "flowid", cls_id)

			var tc_out bytes.Buffer
			var tc_err bytes.Buffer
			cmd.Stdout = &tc_out
			cmd.Stderr = &tc_err
			err := cmd.Run()
			if err != nil {
				err := fmt.Errorf("failed to add filter using TC,%s", err, tc_err.String())
				return err
			}

		}
		dstPort := nw.dstPort
		//tc filter add dev em49 protocol ip parent 1: prio 1 u32 match ip dport 45455  0xffff flowid 1:1
		{
			cmd := exec.Command(tc_binary, "filter", "add", "dev", tc_interface, "protocol", "ip", "parent", mjr_handle,
				"prio", filter_priority, "u32", "match", "ip", "dport", dstPort, "0xffff", "flowid", cls_id)

			var tc_out bytes.Buffer
			var tc_err bytes.Buffer
			cmd.Stdout = &tc_out
			cmd.Stderr = &tc_err
			err := cmd.Run()
			if err != nil {
				err := fmt.Errorf("failed to add filter using TC,%s", err, tc_err.String())
				return err
			}
		}

	}

	return nil

}

func (r Rules) deleteClass(Container string, interfaceName string) error {

	tc_binary, lookErr := exec.LookPath("tc")
	if lookErr != nil {
		lookErr := fmt.Errorf("Failed to find binary TC, please install traffic classifier:%s", lookErr)
		return lookErr
	}

	tc_interface := interfaceName
	//getting NetworkBandwidth of container from Rules struct
	nw := r.nw[Container]
	mjr_handle := fmt.Sprintf("%x:", nw.majorClassId)
	//minor_handle := fmt.Sprintf("%x:",nw.minorClassId);
	cls_id := fmt.Sprintf("%s%x", mjr_handle, nw.minorClassId)

	set_rate := fmt.Sprintf("%d%s", nw.egressMin, nw.speedType)
	set_ceil := fmt.Sprintf("%d%s", nw.egressMin, nw.speedType)

	// delete a class
	//ex: tc class add dev em49 parent 1: classid 1:1 htb rate 100mbit burst 4000 ceil 150mbit prio 0
	//    tc class delete dev em49 parent 10: classid 10:2 htb rate 10mbit burst 4000 ceil 15mbit prio 0
	{
		cmd := exec.Command(tc_binary, "class", "delete", "dev", tc_interface, "parent", mjr_handle, "classid", cls_id, "htb",
			"rate", set_rate, "burst", "4000", "ceil", set_ceil, "prio", "0")

		var tc_out bytes.Buffer
		var tc_err bytes.Buffer
		cmd.Stdout = &tc_out
		cmd.Stderr = &tc_err
		err := cmd.Run()
		if err != nil {
			err := fmt.Errorf("failed to delete class using TC,%s", err, tc_err.String())
			return err
		}

	}
	//TOdo: add the set_rate/speed to total remaining speed

	return nil
}

func (r Rules) deleteFilter(Container string, interfaceName string) error {

	tc_binary, lookErr := exec.LookPath("tc")
	if lookErr != nil {
		lookErr := fmt.Errorf("Failed to find binary TC, please install traffic classifier:%s", lookErr)
		return lookErr
	}

	tc_interface := interfaceName
	//getting NetworkBandwidth of container from Rules struct
	nw := r.nw[Container]
	mjr_handle := fmt.Sprintf("%x:", nw.majorClassId)
	minor_handle := fmt.Sprintf("%x", nw.minorClassId)
	cls_id := fmt.Sprintf("%s%x", mjr_handle, nw.minorClassId)

	if nw.srcPort == "" && nw.dstPort == "" {
		//tc filter delete dev em49 protocol all parent 10: prio 1
		{
			cmd := exec.Command(tc_binary, "filter", "delete", "dev", tc_interface, "protocol", "all", "parent", mjr_handle,
				"prio", minor_handle)

			var tc_out bytes.Buffer
			var tc_err bytes.Buffer
			cmd.Stdout = &tc_out
			cmd.Stderr = &tc_err
			err := cmd.Run()
			if err != nil {
				err := fmt.Errorf("failed to delete cgroup filter using TC,%s", err, tc_err.String())
				return err
			}

		}
		return nil
	}

	dstPort := nw.dstPort
	//tc filter delete dev em49 protocol ip parent 1: prio 1 u32 match ip dport 45455  0xffff flowid 1:1
	// This will delete both sport and dport filters
	{
		cmd := exec.Command(tc_binary, "filter", "delete", "dev", tc_interface, "protocol", "ip", "parent", mjr_handle,
			"prio", minor_handle, "u32", "match", "ip", "dport", dstPort, "0xffff", "flowid", cls_id)

		var tc_out bytes.Buffer
		var tc_err bytes.Buffer
		cmd.Stdout = &tc_out
		cmd.Stderr = &tc_err
		err := cmd.Run()
		if err != nil {
			err := fmt.Errorf("failed to delete dPort filter using TC,%s", err, tc_err.String())
			return err
		}
	}

	/*
	           // If you set priority for src and dst port differently, then enable this delete
		   srcPort := nw.srcPort
		   //tc filter delete dev em49 protocol ip parent 1: prio 1 u32 match ip dport 45455  0xffff flowid 1:1
		   {
		           cmd := exec.Command(tc_binary, "filter", "delete", "dev", tc_interface, "protocol", "ip", "parent", mjr_handle,
		                   "prio", minor_handle, "u32", "match", "ip", "dport", srcPort, "0xffff", "flowid", cls_id)

		           var tc_out bytes.Buffer
		           var tc_err bytes.Buffer
		           cmd.Stdout = &tc_out
		           cmd.Stderr = &tc_err
		           err := cmd.Run()
		           if err != nil {
		                   err := fmt.Errorf("failed to delete srcPort filter using TC,%s", err, tc_err.String())
		                   return err
		           }
		   }
	*/

	return nil
}

//TODO:
func (r *Rules) DisplayContainerBwStat(container string, interfaceName string) error {
	// Read lock and unlock
	return nil

}

func getDefaultInterfaceName() (string, error) {

	ip_bin, lookErr := exec.LookPath("ip")
	if lookErr != nil {
		lookErr = fmt.Errorf("Failed to find binary ip:%s", lookErr)
		return "", lookErr
	}

	cmd := exec.Command(ip_bin, "route", "ls")
	var ip_out bytes.Buffer
	var ip_err bytes.Buffer
	cmd.Stdout = &ip_out
	cmd.Stderr = &ip_err

	err := cmd.Run()
	if err != nil {
		lookErr = fmt.Errorf("failed to run ip route command ", err, ip_err.String())
		return "", lookErr
	}

	lines := strings.Split(strings.TrimSpace(ip_out.String()), "\n")
	for _, l := range lines {
		if strings.Contains(l, "default") {
			sp := strings.Split(l, "dev")
			def_interface := strings.TrimSpace(sp[1])
			if def_interface != "" {
				return def_interface, nil
			}
		}
	}

	if !strings.Contains(ip_out.String(), "default") {
		return "", fmt.Errorf("No default Gateway route found on Host")
	}
	return "", nil
}

func getInterfaceSpeed(interfaceName string) (int32, error) {

	net_path := "/sys/class/net"
	interface_speed_path := net_path + "/" + interfaceName + "/" + "speed"

	if _, err := os.Stat(interface_speed_path); err != nil {
		if os.IsNotExist(err) {
			//fmt.Println("path doesn't exist",interface_speed_path)
			return 0, fmt.Errorf("path doesn't exist", interface_speed_path)
		}
	}
	//fmt.Println("default interface path to speed ",interface_speed_path)
	cat_bin, lookErr := exec.LookPath("cat")
	if lookErr != nil {
		lookErr = fmt.Errorf("Failed to find binary ip:%s", lookErr)
		return 0, lookErr
	}

	cmd := exec.Command(cat_bin, interface_speed_path)
	var cat_out bytes.Buffer
	var cat_err bytes.Buffer
	cmd.Stdout = &cat_out
	cmd.Stderr = &cat_err
	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("failed to get interface speed", err, cat_err.String())
	}
	var speed int64
	t := strings.TrimSpace(cat_out.String())
	if t != "" {
		speed, err = strconv.ParseInt(t, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("failed to get interface speed :", err)
		}
		return int32(speed), err
	} else {
		return 0, fmt.Errorf("Not a valid device to fetch speed")
	}
}

// Init registers a new instance of host driver
func Init(dc driverapi.DriverCallback, config map[string]interface{}) error {
	c := driverapi.Capability{
		DataScope: datastore.LocalScope,
	}
	HostNic = make(map[Nic]Rules)

	return dc.RegisterDriver(networkType, &driver{}, c)
}

func (d *driver) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	return nil, nettypes.NotImplementedErrorf("not implemented")
}

func (d *driver) NetworkFree(id string) error {
	return nettypes.NotImplementedErrorf("not implemented")
}

func (d *driver) EventNotify(etype driverapi.EventType, nid, tableName, key string, value []byte) {
}

func (d *driver) CreateNetwork(id string, option map[string]interface{}, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	d.Lock()
	defer d.Unlock()

	if d.network != "" {
		return nettypes.ForbiddenErrorf("shiva only one instance of \"%s\" network is allowed", networkType)
	}

	d.network = id

	return nil
}

func (d *driver) DeleteNetwork(nid string) error {
	return nettypes.ForbiddenErrorf("network of type \"%s\" cannot be deleted", networkType)
}

func (d *driver) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo, epOptions map[string]interface{}) error {
	return nil
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	return nil
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return make(map[string]interface{}, 0), nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	return nil
}

func (d *driver) ProgramExternalConnectivity(nid, eid string, options map[string]interface{}) error {
	return nil
}

func (d *driver) RevokeExternalConnectivity(nid, eid string) error {
	return nil
}

func (d *driver) Type() string {
	return networkType
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *driver) DiscoverNew(dType discoverapi.DiscoveryType, data interface{}) error {
	return nil
}

// DiscoverDelete is a notification for a discovery delete event, such as a node leaving a cluster
func (d *driver) DiscoverDelete(dType discoverapi.DiscoveryType, data interface{}) error {
	return nil
}
