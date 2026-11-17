# socket [![Test Status](https://github.com/mdlayher/socket/workflows/Test/badge.svg)](https://github.com/mdlayher/socket/actions) [![Go Reference](https://pkg.go.dev/badge/github.com/mdlayher/socket.svg)](https://pkg.go.dev/github.com/mdlayher/socket)

Package `socket` provides a low-level network connection type which integrates
with Go's runtime network poller to provide asynchronous I/O and deadline
support. MIT Licensed.

This package focuses on UNIX-like operating systems which make use of BSD
sockets system call APIs. It is meant to be used as a foundation for the
creation of operating system-specific socket packages, for socket families such
as Linux's `AF_NETLINK`, `AF_PACKET`, or `AF_VSOCK`. This package should not be
used directly in end user applications.

Any use of package socket should be guarded by build tags, as one would also
use when importing the `syscall` or `golang.org/x/sys` packages.

## Ecosystem

Over time, an ecosystem of Go packages has developed around package `socket`.
Many of these packages provide access to a specific socket family, such as
`AF_NETLINK` or `AF_PACKET`, and act as building blocks for further packages.

To have your package included in this diagram, please send a pull request!

```mermaid
flowchart LR
    socket["github.com/mdlayher/socket"]
    click socket "https://github.com/mdlayher/socket"

    subgraph af_inet["AF_INET and AF_INET6"]
        direction LR

        go-rosenpass["cunicu.li/go-rosenpass"]
        click go-rosenpass "https://codeberg.org/cunicu/go-rosenpass"

        icmpx["github.com/mdlayher/icmpx"]
        click icmpx "https://github.com/mdlayher/icmpx"

        kquic["github.com/mdlayher/kquic"]
        click kquic "https://github.com/mdlayher/kquic"

        netbird["github.com/netbirdio/netbird"]
        click netbird "https://github.com/netbirdio/netbird"

        wsoding["github.com/shadowy-pycoder/wsoding"]
        click wsoding "https://github.com/shadowy-pycoder/wsoding"
    end

    subgraph "AF_KCM"
        direction LR

        kcm["github.com/mdlayher/kcm"]
        click kcm "https://github.com/mdlayher/kcm"
    end

    subgraph "AF_NETLINK"
        direction LR

        netlink["github.com/mdlayher/netlink"]
        click netlink "https://github.com/mdlayher/netlink"

        ebpf-nftrace["github.com/Morwran/ebpf-nftrace"]
        click ebpf-nftrace "https://github.com/Morwran/ebpf-nftrace"
    end

    subgraph "AF_PACKET"
        direction LR

        packet["github.com/mdlayher/packet"]
        click packet "https://github.com/mdlayher/packet"

        fast_afpacket["github.com/subspace-com/fast_afpacket"]
        click fast_afpacket "https://github.com/subspace-com/fast_afpacket"

        ndn-dpdk["github.com/usnistgov/ndn-dpdk"]
        click ndn-dpdk "https://github.com/usnistgov/ndn-dpdk"

        ovn-kubernetes["github.com/ovn-kubernetes/ovn-kubernetes"]
        click ovn-kubernetes "https://github.com/ovn-kubernetes/ovn-kubernetes"

        tailscale["tailscale.com"]
        click tailscale "https://github.com/tailscale/tailscale"
    end

    subgraph "AF_VSOCK"
        direction LR

        vsock["github.com/mdlayher/vsock"]
        click vsock "https://github.com/mdlayher/vsock"
    end

    subgraph fds["Process file descriptors"]
        direction LR

        pidfd["github.com/mdlayher/pidfd"]
        click pidfd "https://github.com/mdlayher/pidfd"
    end

    af_inet --> socket
    AF_KCM --> socket
    AF_NETLINK --> socket
    AF_PACKET --> socket
    AF_VSOCK --> socket
    fds --> socket
```

## Stability

See the [CHANGELOG](./CHANGELOG.md) file for a description of changes between
releases.

This package only supports the two most recent major versions of Go, mirroring
Go's own release policy. Older versions of Go may lack critical features and bug
fixes which are necessary for this package to function correctly.
