package network

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type NetworkStats struct {
	RxBytes   uint64 `json:"rx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	RxDropped uint64 `json:"rx_dropped"`
	TxBytes   uint64 `json:"tx_bytes"`
	TxPackets uint64 `json:"tx_packets"`
	TxErrors  uint64 `json:"tx_errors"`
	TxDropped uint64 `json:"tx_dropped"`
}

// Returns the network statistics for the network interfaces represented by the NetworkRuntimeInfo.
func GetStats(networkState *NetworkState) (*NetworkStats, error) {
	// This can happen if the network runtime information is missing - possible if the container was created by an old version of libcontainer.
	if networkState.VethHost == "" {
		return &NetworkStats{}, nil
	}
	data, err := readSysfsNetworkStats(networkState.VethHost)
	if err != nil {
		return nil, err
	}

	// Ingress for host veth is from the container. Hence tx_bytes stat on the host veth is actually number of bytes received by the container.
	return &NetworkStats{
		RxBytes:   data["tx_bytes"],
		RxPackets: data["tx_packets"],
		RxErrors:  data["tx_errors"],
		RxDropped: data["tx_dropped"],
		TxBytes:   data["rx_bytes"],
		TxPackets: data["rx_packets"],
		TxErrors:  data["rx_errors"],
		TxDropped: data["rx_dropped"],
	}, nil
}

// Reads all the statistics available under /sys/class/net/<EthInterface>/statistics as a map with file name as key and data as integers.
func readSysfsNetworkStats(ethInterface string) (map[string]uint64, error) {
	out := make(map[string]uint64)

	fullPath := filepath.Join("/sys/class/net", ethInterface, "statistics/")
	err := filepath.Walk(fullPath, func(path string, _ os.FileInfo, _ error) error {
		// skip fullPath.
		if path == fullPath {
			return nil
		}
		base := filepath.Base(path)
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			return err
		}
		out[base] = value
		return nil
	})
	return out, err
}
