package dbclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/log"
)

var servicePort string

const totalWrittenKeys string = "totalKeys"

type resultTuple struct {
	id     string
	result int
}

func httpGetFatalError(ip, port, path string) {
	body, err := httpGet(ip, port, path)
	if err != nil || !strings.Contains(string(body), "OK") {
		log.G(context.TODO()).Fatalf("[%s] error %s %s", path, err, body)
	}
}

func httpGet(ip, port, path string) ([]byte, error) {
	resp, err := http.Get("http://" + ip + ":" + port + path)
	if err != nil {
		log.G(context.TODO()).Errorf("httpGet error:%s", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, err
}

func joinCluster(ip, port string, members []string, doneCh chan resultTuple) {
	httpGetFatalError(ip, port, "/join?members="+strings.Join(members, ","))

	if doneCh != nil {
		doneCh <- resultTuple{id: ip, result: 0}
	}
}

func joinNetwork(ip, port, network string, doneCh chan resultTuple) {
	httpGetFatalError(ip, port, "/joinnetwork?nid="+network)

	if doneCh != nil {
		doneCh <- resultTuple{id: ip, result: 0}
	}
}

func leaveNetwork(ip, port, network string, doneCh chan resultTuple) {
	httpGetFatalError(ip, port, "/leavenetwork?nid="+network)

	if doneCh != nil {
		doneCh <- resultTuple{id: ip, result: 0}
	}
}

func writeTableKey(ip, port, networkName, tableName, key string) {
	createPath := "/createentry?unsafe&nid=" + networkName + "&tname=" + tableName + "&value=v&key="
	httpGetFatalError(ip, port, createPath+key)
}

func deleteTableKey(ip, port, networkName, tableName, key string) {
	deletePath := "/deleteentry?nid=" + networkName + "&tname=" + tableName + "&key="
	httpGetFatalError(ip, port, deletePath+key)
}

func clusterPeersNumber(ip, port string, doneCh chan resultTuple) {
	body, err := httpGet(ip, port, "/clusterpeers")
	if err != nil {
		log.G(context.TODO()).Errorf("clusterPeers %s there was an error: %s", ip, err)
		doneCh <- resultTuple{id: ip, result: -1}
		return
	}
	peersRegexp := regexp.MustCompile(`total entries: ([0-9]+)`)
	peersNum, _ := strconv.Atoi(peersRegexp.FindStringSubmatch(string(body))[1])

	doneCh <- resultTuple{id: ip, result: peersNum}
}

func networkPeersNumber(ip, port, networkName string, doneCh chan resultTuple) {
	body, err := httpGet(ip, port, "/networkpeers?nid="+networkName)
	if err != nil {
		log.G(context.TODO()).Errorf("networkPeersNumber %s there was an error: %s", ip, err)
		doneCh <- resultTuple{id: ip, result: -1}
		return
	}
	peersRegexp := regexp.MustCompile(`total entries: ([0-9]+)`)
	peersNum, _ := strconv.Atoi(peersRegexp.FindStringSubmatch(string(body))[1])

	doneCh <- resultTuple{id: ip, result: peersNum}
}

func dbTableEntriesNumber(ip, port, networkName, tableName string, doneCh chan resultTuple) {
	body, err := httpGet(ip, port, "/gettable?nid="+networkName+"&tname="+tableName)
	if err != nil {
		log.G(context.TODO()).Errorf("tableEntriesNumber %s there was an error: %s", ip, err)
		doneCh <- resultTuple{id: ip, result: -1}
		return
	}
	elementsRegexp := regexp.MustCompile(`total entries: ([0-9]+)`)
	entriesNum, _ := strconv.Atoi(elementsRegexp.FindStringSubmatch(string(body))[1])
	doneCh <- resultTuple{id: ip, result: entriesNum}
}

func dbQueueLength(ip, port, networkName string, doneCh chan resultTuple) {
	body, err := httpGet(ip, port, "/networkstats?nid="+networkName)
	if err != nil {
		log.G(context.TODO()).Errorf("queueLength %s there was an error: %s", ip, err)
		doneCh <- resultTuple{id: ip, result: -1}
		return
	}
	elementsRegexp := regexp.MustCompile(`qlen: ([0-9]+)`)
	entriesNum, _ := strconv.Atoi(elementsRegexp.FindStringSubmatch(string(body))[1])
	doneCh <- resultTuple{id: ip, result: entriesNum}
}

func clientWatchTable(ip, port, networkName, tableName string, doneCh chan resultTuple) {
	httpGetFatalError(ip, port, "/watchtable?nid="+networkName+"&tname="+tableName)
	if doneCh != nil {
		doneCh <- resultTuple{id: ip, result: 0}
	}
}

func clientTableEntriesNumber(ip, port, networkName, tableName string, doneCh chan resultTuple) {
	body, err := httpGet(ip, port, "/watchedtableentries?nid="+networkName+"&tname="+tableName)
	if err != nil {
		log.G(context.TODO()).Errorf("clientTableEntriesNumber %s there was an error: %s", ip, err)
		doneCh <- resultTuple{id: ip, result: -1}
		return
	}
	elementsRegexp := regexp.MustCompile(`total elements: ([0-9]+)`)
	entriesNum, _ := strconv.Atoi(elementsRegexp.FindStringSubmatch(string(body))[1])
	doneCh <- resultTuple{id: ip, result: entriesNum}
}

func writeKeysNumber(ip, port, networkName, tableName, key string, number int, doneCh chan resultTuple) {
	x := 0
	for ; x < number; x++ {
		k := key + strconv.Itoa(x)
		// write key
		writeTableKey(ip, port, networkName, tableName, k)
	}
	doneCh <- resultTuple{id: ip, result: x}
}

func deleteKeysNumber(ip, port, networkName, tableName, key string, number int, doneCh chan resultTuple) {
	x := 0
	for ; x < number; x++ {
		k := key + strconv.Itoa(x)
		// write key
		deleteTableKey(ip, port, networkName, tableName, k)
	}
	doneCh <- resultTuple{id: ip, result: x}
}

func writeUniqueKeys(ctx context.Context, ip, port, networkName, tableName, key string, doneCh chan resultTuple) {
	for x := 0; ; x++ {
		select {
		case <-ctx.Done():
			doneCh <- resultTuple{id: ip, result: x}
			return
		default:
			k := key + strconv.Itoa(x)
			// write key
			writeTableKey(ip, port, networkName, tableName, k)
			// give time to send out key writes
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func writeDeleteUniqueKeys(ctx context.Context, ip, port, networkName, tableName, key string, doneCh chan resultTuple) {
	for x := 0; ; x++ {
		select {
		case <-ctx.Done():
			doneCh <- resultTuple{id: ip, result: x}
			return
		default:
			k := key + strconv.Itoa(x)
			// write key
			writeTableKey(ip, port, networkName, tableName, k)
			// give time to send out key writes
			time.Sleep(100 * time.Millisecond)
			// delete key
			deleteTableKey(ip, port, networkName, tableName, k)
		}
	}
}

func writeDeleteLeaveJoin(ctx context.Context, ip, port, networkName, tableName, key string, doneCh chan resultTuple) {
	for x := 0; ; x++ {
		select {
		case <-ctx.Done():
			doneCh <- resultTuple{id: ip, result: x}
			return
		default:
			k := key + strconv.Itoa(x)
			// write key
			writeTableKey(ip, port, networkName, tableName, k)
			time.Sleep(100 * time.Millisecond)
			// delete key
			deleteTableKey(ip, port, networkName, tableName, k)
			// give some time
			time.Sleep(100 * time.Millisecond)
			// leave network
			leaveNetwork(ip, port, networkName, nil)
			// join network
			joinNetwork(ip, port, networkName, nil)
		}
	}
}

func ready(ip, port string, doneCh chan resultTuple) {
	for {
		body, err := httpGet(ip, port, "/ready")
		if err != nil || !strings.Contains(string(body), "OK") {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		// success
		break
	}
	// notify the completion
	doneCh <- resultTuple{id: ip, result: 0}
}

func checkTable(ctx context.Context, ips []string, port, networkName, tableName string, expectedEntries int, fn func(string, string, string, string, chan resultTuple)) (opTime time.Duration) {
	startTime := time.Now().UnixNano()
	var successTime int64

	// Loop for 2 minutes to guarantee that the result is stable
	for {
		select {
		case <-ctx.Done():
			// Validate test success, if the time is set means that all the tables are empty
			if successTime != 0 {
				opTime = time.Duration(successTime-startTime) / time.Millisecond
				log.G(ctx).Infof("Check table passed, the cluster converged in %d msec", opTime)
				return
			}
			log.G(ctx).Fatal("Test failed, there is still entries in the tables of the nodes")
		default:
			log.G(ctx).Infof("Checking table %s expected %d", tableName, expectedEntries)
			doneCh := make(chan resultTuple, len(ips))
			for _, ip := range ips {
				go fn(ip, servicePort, networkName, tableName, doneCh)
			}

			nodesWithCorrectEntriesNum := 0
			for i := len(ips); i > 0; i-- {
				tableEntries := <-doneCh
				log.G(ctx).Infof("Node %s has %d entries", tableEntries.id, tableEntries.result)
				if tableEntries.result == expectedEntries {
					nodesWithCorrectEntriesNum++
				}
			}
			close(doneCh)
			if nodesWithCorrectEntriesNum == len(ips) {
				if successTime == 0 {
					successTime = time.Now().UnixNano()
					log.G(ctx).Infof("Success after %d msec", time.Duration(successTime-startTime)/time.Millisecond)
				}
			} else {
				successTime = 0
			}
			time.Sleep(10 * time.Second)
		}
	}
}

func waitWriters(parallelWriters int, mustWrite bool, doneCh chan resultTuple) map[string]int {
	var totalKeys int
	resultTable := make(map[string]int)
	for i := 0; i < parallelWriters; i++ {
		log.G(context.TODO()).Infof("Waiting for %d workers", parallelWriters-i)
		workerReturn := <-doneCh
		totalKeys += workerReturn.result
		if mustWrite && workerReturn.result == 0 {
			log.G(context.TODO()).Fatalf("The worker %s did not write any key %d == 0", workerReturn.id, workerReturn.result)
		}
		if !mustWrite && workerReturn.result != 0 {
			log.G(context.TODO()).Fatalf("The worker %s was supposed to return 0 instead %d != 0", workerReturn.id, workerReturn.result)
		}
		if mustWrite {
			resultTable[workerReturn.id] = workerReturn.result
			log.G(context.TODO()).Infof("The worker %s wrote %d keys", workerReturn.id, workerReturn.result)
		}
	}
	resultTable[totalWrittenKeys] = totalKeys
	return resultTable
}

// ready
func doReady(ips []string) {
	doneCh := make(chan resultTuple, len(ips))
	// check all the nodes
	for _, ip := range ips {
		go ready(ip, servicePort, doneCh)
	}
	// wait for the readiness of all nodes
	for i := len(ips); i > 0; i-- {
		<-doneCh
	}
	close(doneCh)
}

// join
func doJoin(ips []string) {
	doneCh := make(chan resultTuple, len(ips))
	// check all the nodes
	for i, ip := range ips {
		members := append([]string(nil), ips[:i]...)
		members = append(members, ips[i+1:]...)
		go joinCluster(ip, servicePort, members, doneCh)
	}
	// wait for the readiness of all nodes
	for i := len(ips); i > 0; i-- {
		<-doneCh
	}
	close(doneCh)
}

// cluster-peers expectedNumberPeers maxRetry
func doClusterPeers(ips []string, args []string) {
	doneCh := make(chan resultTuple, len(ips))
	expectedPeers, _ := strconv.Atoi(args[0])
	maxRetry, _ := strconv.Atoi(args[1])
	for retry := 0; retry < maxRetry; retry++ {
		// check all the nodes
		for _, ip := range ips {
			go clusterPeersNumber(ip, servicePort, doneCh)
		}
		var failed bool
		// wait for the readiness of all nodes
		for i := len(ips); i > 0; i-- {
			node := <-doneCh
			if node.result != expectedPeers {
				failed = true
				if retry == maxRetry-1 {
					log.G(context.TODO()).Fatalf("Expected peers from %s mismatch %d != %d", node.id, expectedPeers, node.result)
				} else {
					log.G(context.TODO()).Warnf("Expected peers from %s mismatch %d != %d", node.id, expectedPeers, node.result)
				}
				time.Sleep(1 * time.Second)
			}
		}
		// check if needs retry
		if !failed {
			break
		}
	}
	close(doneCh)
}

// join-network networkName
func doJoinNetwork(ips []string, args []string) {
	doneCh := make(chan resultTuple, len(ips))
	// check all the nodes
	for _, ip := range ips {
		go joinNetwork(ip, servicePort, args[0], doneCh)
	}
	// wait for the readiness of all nodes
	for i := len(ips); i > 0; i-- {
		<-doneCh
	}
	close(doneCh)
}

// leave-network networkName
func doLeaveNetwork(ips []string, args []string) {
	doneCh := make(chan resultTuple, len(ips))
	// check all the nodes
	for _, ip := range ips {
		go leaveNetwork(ip, servicePort, args[0], doneCh)
	}
	// wait for the readiness of all nodes
	for i := len(ips); i > 0; i-- {
		<-doneCh
	}
	close(doneCh)
}

// network-peers networkName expectedNumberPeers maxRetry
func doNetworkPeers(ips []string, args []string) {
	doneCh := make(chan resultTuple, len(ips))
	networkName := args[0]
	expectedPeers, _ := strconv.Atoi(args[1])
	maxRetry, _ := strconv.Atoi(args[2])
	for retry := 0; retry < maxRetry; retry++ {
		// check all the nodes
		for _, ip := range ips {
			go networkPeersNumber(ip, servicePort, networkName, doneCh)
		}
		var failed bool
		// wait for the readiness of all nodes
		for i := len(ips); i > 0; i-- {
			node := <-doneCh
			if node.result != expectedPeers {
				failed = true
				if retry == maxRetry-1 {
					log.G(context.TODO()).Fatalf("Expected peers from %s mismatch %d != %d", node.id, expectedPeers, node.result)
				} else {
					log.G(context.TODO()).Warnf("Expected peers from %s mismatch %d != %d", node.id, expectedPeers, node.result)
				}
				time.Sleep(1 * time.Second)
			}
		}
		// check if needs retry
		if !failed {
			break
		}
	}
	close(doneCh)
}

// network-stats-queue networkName <gt/lt> queueSize
func doNetworkStatsQueue(ips []string, args []string) {
	doneCh := make(chan resultTuple, len(ips))
	networkName := args[0]
	comparison := args[1]
	size, _ := strconv.Atoi(args[2])

	// check all the nodes
	for _, ip := range ips {
		go dbQueueLength(ip, servicePort, networkName, doneCh)
	}

	var avgQueueSize int
	// wait for the readiness of all nodes
	for i := len(ips); i > 0; i-- {
		node := <-doneCh
		switch comparison {
		case "lt":
			if node.result > size {
				log.G(context.TODO()).Fatalf("Expected queue size from %s to be %d < %d", node.id, node.result, size)
			}
		case "gt":
			if node.result < size {
				log.G(context.TODO()).Fatalf("Expected queue size from %s to be %d > %d", node.id, node.result, size)
			}
		default:
			log.G(context.TODO()).Fatal("unknown comparison operator")
		}
		avgQueueSize += node.result
	}
	close(doneCh)
	avgQueueSize /= len(ips)
	fmt.Fprintf(os.Stderr, "doNetworkStatsQueue succeeded with avg queue:%d", avgQueueSize)
}

// write-keys networkName tableName parallelWriters numberOfKeysEach
func doWriteKeys(ips []string, args []string) {
	networkName := args[0]
	tableName := args[1]
	parallelWriters, _ := strconv.Atoi(args[2])
	numberOfKeys, _ := strconv.Atoi(args[3])

	doneCh := make(chan resultTuple, parallelWriters)
	// Enable watch of tables from clients
	for i := 0; i < parallelWriters; i++ {
		go clientWatchTable(ips[i], servicePort, networkName, tableName, doneCh)
	}
	waitWriters(parallelWriters, false, doneCh)

	// Start parallel writers that will create and delete unique keys
	defer close(doneCh)
	for i := 0; i < parallelWriters; i++ {
		key := "key-" + strconv.Itoa(i) + "-"
		log.G(context.TODO()).Infof("Spawn worker: %d on IP:%s", i, ips[i])
		go writeKeysNumber(ips[i], servicePort, networkName, tableName, key, numberOfKeys, doneCh)
	}

	// Sync with all the writers
	keyMap := waitWriters(parallelWriters, true, doneCh)
	log.G(context.TODO()).Infof("Written a total of %d keys on the cluster", keyMap[totalWrittenKeys])

	// check table entries for 2 minutes
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	opTime := checkTable(ctx, ips, servicePort, networkName, tableName, keyMap[totalWrittenKeys], dbTableEntriesNumber)
	cancel()
	fmt.Fprintf(os.Stderr, "doWriteKeys succeeded in %d msec", opTime)
}

// delete-keys networkName tableName parallelWriters numberOfKeysEach
func doDeleteKeys(ips []string, args []string) {
	networkName := args[0]
	tableName := args[1]
	parallelWriters, _ := strconv.Atoi(args[2])
	numberOfKeys, _ := strconv.Atoi(args[3])

	doneCh := make(chan resultTuple, parallelWriters)
	// Enable watch of tables from clients
	for i := 0; i < parallelWriters; i++ {
		go clientWatchTable(ips[i], servicePort, networkName, tableName, doneCh)
	}
	waitWriters(parallelWriters, false, doneCh)

	// Start parallel writers that will create and delete unique keys
	defer close(doneCh)
	for i := 0; i < parallelWriters; i++ {
		key := "key-" + strconv.Itoa(i) + "-"
		log.G(context.TODO()).Infof("Spawn worker: %d on IP:%s", i, ips[i])
		go deleteKeysNumber(ips[i], servicePort, networkName, tableName, key, numberOfKeys, doneCh)
	}

	// Sync with all the writers
	keyMap := waitWriters(parallelWriters, true, doneCh)
	log.G(context.TODO()).Infof("Written a total of %d keys on the cluster", keyMap[totalWrittenKeys])

	// check table entries for 2 minutes
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	opTime := checkTable(ctx, ips, servicePort, networkName, tableName, 0, dbTableEntriesNumber)
	cancel()
	fmt.Fprintf(os.Stderr, "doDeletekeys succeeded in %d msec", opTime)
}

// write-delete-unique-keys networkName tableName numParallelWriters writeTimeSec
func doWriteDeleteUniqueKeys(ips []string, args []string) {
	networkName := args[0]
	tableName := args[1]
	parallelWriters, _ := strconv.Atoi(args[2])
	writeTimeSec, _ := strconv.Atoi(args[3])

	doneCh := make(chan resultTuple, parallelWriters)
	// Enable watch of tables from clients
	for i := 0; i < parallelWriters; i++ {
		go clientWatchTable(ips[i], servicePort, networkName, tableName, doneCh)
	}
	waitWriters(parallelWriters, false, doneCh)

	// Start parallel writers that will create and delete unique keys
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(writeTimeSec)*time.Second)
	for i := 0; i < parallelWriters; i++ {
		key := "key-" + strconv.Itoa(i) + "-"
		log.G(ctx).Infof("Spawn worker: %d on IP:%s", i, ips[i])
		go writeDeleteUniqueKeys(ctx, ips[i], servicePort, networkName, tableName, key, doneCh)
	}

	// Sync with all the writers
	keyMap := waitWriters(parallelWriters, true, doneCh)
	cancel()
	log.G(ctx).Infof("Written a total of %d keys on the cluster", keyMap[totalWrittenKeys])

	// check table entries for 2 minutes
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	opDBTime := checkTable(ctx, ips, servicePort, networkName, tableName, 0, dbTableEntriesNumber)
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	opClientTime := checkTable(ctx, ips, servicePort, networkName, tableName, 0, clientTableEntriesNumber)
	cancel()
	fmt.Fprintf(os.Stderr, "doWriteDeleteUniqueKeys succeeded in %d msec and client %d msec", opDBTime, opClientTime)
}

// write-unique-keys networkName tableName numParallelWriters writeTimeSec
func doWriteUniqueKeys(ips []string, args []string) {
	networkName := args[0]
	tableName := args[1]
	parallelWriters, _ := strconv.Atoi(args[2])
	writeTimeSec, _ := strconv.Atoi(args[3])

	doneCh := make(chan resultTuple, parallelWriters)
	// Enable watch of tables from clients
	for i := 0; i < parallelWriters; i++ {
		go clientWatchTable(ips[i], servicePort, networkName, tableName, doneCh)
	}
	waitWriters(parallelWriters, false, doneCh)

	// Start parallel writers that will create and delete unique keys
	defer close(doneCh)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(writeTimeSec)*time.Second)
	for i := 0; i < parallelWriters; i++ {
		key := "key-" + strconv.Itoa(i) + "-"
		log.G(ctx).Infof("Spawn worker: %d on IP:%s", i, ips[i])
		go writeUniqueKeys(ctx, ips[i], servicePort, networkName, tableName, key, doneCh)
	}

	// Sync with all the writers
	keyMap := waitWriters(parallelWriters, true, doneCh)
	cancel()
	log.G(ctx).Infof("Written a total of %d keys on the cluster", keyMap[totalWrittenKeys])

	// check table entries for 2 minutes
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	opTime := checkTable(ctx, ips, servicePort, networkName, tableName, keyMap[totalWrittenKeys], dbTableEntriesNumber)
	cancel()
	fmt.Fprintf(os.Stderr, "doWriteUniqueKeys succeeded in %d msec", opTime)
}

// write-delete-leave-join networkName tableName numParallelWriters writeTimeSec
func doWriteDeleteLeaveJoin(ips []string, args []string) {
	networkName := args[0]
	tableName := args[1]
	parallelWriters, _ := strconv.Atoi(args[2])
	writeTimeSec, _ := strconv.Atoi(args[3])

	// Start parallel writers that will create and delete unique keys
	doneCh := make(chan resultTuple, parallelWriters)
	defer close(doneCh)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(writeTimeSec)*time.Second)
	for i := 0; i < parallelWriters; i++ {
		key := "key-" + strconv.Itoa(i) + "-"
		log.G(ctx).Infof("Spawn worker: %d on IP:%s", i, ips[i])
		go writeDeleteLeaveJoin(ctx, ips[i], servicePort, networkName, tableName, key, doneCh)
	}

	// Sync with all the writers
	keyMap := waitWriters(parallelWriters, true, doneCh)
	cancel()
	log.G(ctx).Infof("Written a total of %d keys on the cluster", keyMap["totalKeys"])

	// check table entries for 2 minutes
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	opTime := checkTable(ctx, ips, servicePort, networkName, tableName, 0, dbTableEntriesNumber)
	cancel()
	fmt.Fprintf(os.Stderr, "doWriteDeleteLeaveJoin succeeded in %d msec", opTime)
}

// write-delete-wait-leave-join networkName tableName numParallelWriters writeTimeSec
func doWriteDeleteWaitLeaveJoin(ips []string, args []string) {
	networkName := args[0]
	tableName := args[1]
	parallelWriters, _ := strconv.Atoi(args[2])
	writeTimeSec, _ := strconv.Atoi(args[3])

	// Start parallel writers that will create and delete unique keys
	doneCh := make(chan resultTuple, parallelWriters)
	defer close(doneCh)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(writeTimeSec)*time.Second)
	for i := 0; i < parallelWriters; i++ {
		key := "key-" + strconv.Itoa(i) + "-"
		log.G(ctx).Infof("Spawn worker: %d on IP:%s", i, ips[i])
		go writeDeleteUniqueKeys(ctx, ips[i], servicePort, networkName, tableName, key, doneCh)
	}

	// Sync with all the writers
	keyMap := waitWriters(parallelWriters, true, doneCh)
	cancel()
	log.G(ctx).Infof("Written a total of %d keys on the cluster", keyMap[totalWrittenKeys])

	// The writers will leave the network
	for i := 0; i < parallelWriters; i++ {
		log.G(ctx).Infof("worker leaveNetwork: %d on IP:%s", i, ips[i])
		go leaveNetwork(ips[i], servicePort, networkName, doneCh)
	}
	waitWriters(parallelWriters, false, doneCh)

	// Give some time
	time.Sleep(100 * time.Millisecond)

	// The writers will join the network
	for i := 0; i < parallelWriters; i++ {
		log.G(ctx).Infof("worker joinNetwork: %d on IP:%s", i, ips[i])
		go joinNetwork(ips[i], servicePort, networkName, doneCh)
	}
	waitWriters(parallelWriters, false, doneCh)

	// check table entries for 2 minutes
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	opTime := checkTable(ctx, ips, servicePort, networkName, tableName, 0, dbTableEntriesNumber)
	cancel()
	fmt.Fprintf(os.Stderr, "doWriteDeleteWaitLeaveJoin succeeded in %d msec", opTime)
}

// write-wait-leave networkName tableName numParallelWriters writeTimeSec
func doWriteWaitLeave(ips []string, args []string) {
	networkName := args[0]
	tableName := args[1]
	parallelWriters, _ := strconv.Atoi(args[2])
	writeTimeSec, _ := strconv.Atoi(args[3])

	// Start parallel writers that will create and delete unique keys
	doneCh := make(chan resultTuple, parallelWriters)
	defer close(doneCh)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(writeTimeSec)*time.Second)
	for i := 0; i < parallelWriters; i++ {
		key := "key-" + strconv.Itoa(i) + "-"
		log.G(ctx).Infof("Spawn worker: %d on IP:%s", i, ips[i])
		go writeUniqueKeys(ctx, ips[i], servicePort, networkName, tableName, key, doneCh)
	}

	// Sync with all the writers
	keyMap := waitWriters(parallelWriters, true, doneCh)
	cancel()
	log.G(ctx).Infof("Written a total of %d keys on the cluster", keyMap[totalWrittenKeys])

	// The writers will leave the network
	for i := 0; i < parallelWriters; i++ {
		log.G(ctx).Infof("worker leaveNetwork: %d on IP:%s", i, ips[i])
		go leaveNetwork(ips[i], servicePort, networkName, doneCh)
	}
	waitWriters(parallelWriters, false, doneCh)

	// check table entries for 2 minutes
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	opTime := checkTable(ctx, ips, servicePort, networkName, tableName, 0, dbTableEntriesNumber)
	cancel()
	fmt.Fprintf(os.Stderr, "doWriteLeaveJoin succeeded in %d msec", opTime)
}

// write-wait-leave-join networkName tableName numParallelWriters writeTimeSec numParallelLeaver
func doWriteWaitLeaveJoin(ips []string, args []string) {
	networkName := args[0]
	tableName := args[1]
	parallelWriters, _ := strconv.Atoi(args[2])
	writeTimeSec, _ := strconv.Atoi(args[3])
	parallelLeaver, _ := strconv.Atoi(args[4])

	// Start parallel writers that will create and delete unique keys
	doneCh := make(chan resultTuple, parallelWriters)
	defer close(doneCh)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(writeTimeSec)*time.Second)
	for i := 0; i < parallelWriters; i++ {
		key := "key-" + strconv.Itoa(i) + "-"
		log.G(ctx).Infof("Spawn worker: %d on IP:%s", i, ips[i])
		go writeUniqueKeys(ctx, ips[i], servicePort, networkName, tableName, key, doneCh)
	}

	// Sync with all the writers
	keyMap := waitWriters(parallelWriters, true, doneCh)
	cancel()
	log.G(ctx).Infof("Written a total of %d keys on the cluster", keyMap[totalWrittenKeys])

	keysExpected := keyMap[totalWrittenKeys]
	// The Leavers will leave the network
	for i := 0; i < parallelLeaver; i++ {
		log.G(ctx).Infof("worker leaveNetwork: %d on IP:%s", i, ips[i])
		go leaveNetwork(ips[i], servicePort, networkName, doneCh)
		// Once a node leave all the keys written previously will be deleted, so the expected keys will consider that as removed
		keysExpected -= keyMap[ips[i]]
	}
	waitWriters(parallelLeaver, false, doneCh)

	// Give some time
	time.Sleep(100 * time.Millisecond)

	// The writers will join the network
	for i := 0; i < parallelLeaver; i++ {
		log.G(ctx).Infof("worker joinNetwork: %d on IP:%s", i, ips[i])
		go joinNetwork(ips[i], servicePort, networkName, doneCh)
	}
	waitWriters(parallelLeaver, false, doneCh)

	// check table entries for 2 minutes
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	opTime := checkTable(ctx, ips, servicePort, networkName, tableName, keysExpected, dbTableEntriesNumber)
	cancel()
	fmt.Fprintf(os.Stderr, "doWriteWaitLeaveJoin succeeded in %d msec", opTime)
}

var cmdArgChec = map[string]int{
	"debug":                    0,
	"fail":                     0,
	"ready":                    2,
	"join":                     2,
	"leave":                    2,
	"join-network":             3,
	"leave-network":            3,
	"cluster-peers":            5,
	"network-peers":            5,
	"write-delete-unique-keys": 7,
}

// Client is a client
func Client(args []string) {
	log.G(context.TODO()).Infof("[CLIENT] Starting with arguments %v", args)
	command := args[0]

	if len(args) < cmdArgChec[command] {
		log.G(context.TODO()).Fatalf("Command %s requires %d arguments, passed %d, aborting...", command, cmdArgChec[command], len(args))
	}

	switch command {
	case "debug":
		time.Sleep(1 * time.Hour)
		os.Exit(0)
	case "fail":
		log.G(context.TODO()).Fatalf("Test error condition with message: error error error")
	}

	serviceName := args[1]
	ips, _ := net.LookupHost("tasks." + serviceName)
	log.G(context.TODO()).Infof("got the ips %v", ips)
	if len(ips) == 0 {
		log.G(context.TODO()).Fatalf("Cannot resolve any IP for the service tasks.%s", serviceName)
	}
	servicePort = args[2]
	commandArgs := args[3:]
	log.G(context.TODO()).Infof("Executing %s with args:%v", command, commandArgs)
	switch command {
	case "ready":
		doReady(ips)
	case "join":
		doJoin(ips)
	case "leave":

	case "cluster-peers":
		// cluster-peers maxRetry
		doClusterPeers(ips, commandArgs)

	case "join-network":
		// join-network networkName
		doJoinNetwork(ips, commandArgs)
	case "leave-network":
		// leave-network networkName
		doLeaveNetwork(ips, commandArgs)
	case "network-peers":
		// network-peers networkName expectedNumberPeers maxRetry
		doNetworkPeers(ips, commandArgs)
		//	case "network-stats-entries":
		//		// network-stats-entries networkName maxRetry
		//		doNetworkPeers(ips, commandArgs)
	case "network-stats-queue":
		// network-stats-queue networkName <lt/gt> queueSize
		doNetworkStatsQueue(ips, commandArgs)

	case "write-keys":
		// write-keys networkName tableName parallelWriters numberOfKeysEach
		doWriteKeys(ips, commandArgs)
	case "delete-keys":
		// delete-keys networkName tableName parallelWriters numberOfKeysEach
		doDeleteKeys(ips, commandArgs)
	case "write-unique-keys":
		// write-delete-unique-keys networkName tableName numParallelWriters writeTimeSec
		doWriteUniqueKeys(ips, commandArgs)
	case "write-delete-unique-keys":
		// write-delete-unique-keys networkName tableName numParallelWriters writeTimeSec
		doWriteDeleteUniqueKeys(ips, commandArgs)
	case "write-delete-leave-join":
		// write-delete-leave-join networkName tableName numParallelWriters writeTimeSec
		doWriteDeleteLeaveJoin(ips, commandArgs)
	case "write-delete-wait-leave-join":
		// write-delete-wait-leave-join networkName tableName numParallelWriters writeTimeSec
		doWriteDeleteWaitLeaveJoin(ips, commandArgs)
	case "write-wait-leave":
		// write-wait-leave networkName tableName numParallelWriters writeTimeSec
		doWriteWaitLeave(ips, commandArgs)
	case "write-wait-leave-join":
		// write-wait-leave networkName tableName numParallelWriters writeTimeSec
		doWriteWaitLeaveJoin(ips, commandArgs)
	default:
		log.G(context.TODO()).Fatalf("Command %s not recognized", command)
	}
}
