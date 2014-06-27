package network

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type NetworkStats struct {
	RxBytes   uint64 `json:"rx_bytes,omitempty"`
	RxPackets uint64 `json:"rx_packets,omitempty"`
	RxErrors  uint64 `json:"rx_errors,omitempty"`
	RxDropped uint64 `json:"rx_dropped,omitempty"`
	TxBytes   uint64 `json:"tx_bytes,omitempty"`
	TxPackets uint64 `json:"tx_packets,omitempty"`
	TxErrors  uint64 `json:"tx_errors,omitempty"`
	TxDropped uint64 `json:"tx_dropped,omitempty"`
}

// Returns the network statistics for the network interfaces represented by the NetworkRuntimeInfo.
func GetStats(networkState *NetworkState) (NetworkStats, error) {
	// This can happen if the network runtime information is missing - possible if the container was created by an old version of libcontainer.
	if networkState.VethHost == "" {
		return NetworkStats{}, nil
	}
	data, err := readSysfsNetworkStats(networkState.VethHost)
	if err != nil {
		return NetworkStats{}, err
	}

	return NetworkStats{
		RxBytes:   data["rx_bytes"],
		RxPackets: data["rx_packets"],
		RxErrors:  data["rx_errors"],
		RxDropped: data["rx_dropped"],
		TxBytes:   data["tx_bytes"],
		TxPackets: data["tx_packets"],
		TxErrors:  data["tx_errors"],
		TxDropped: data["tx_dropped"],
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
