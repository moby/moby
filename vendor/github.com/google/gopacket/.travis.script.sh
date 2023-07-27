#!/bin/bash

set -ev

go test github.com/google/gopacket
go test github.com/google/gopacket/layers
go test github.com/google/gopacket/tcpassembly
go test github.com/google/gopacket/reassembly
go test github.com/google/gopacket/pcapgo 
go test github.com/google/gopacket/pcap
