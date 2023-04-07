package streams

type Protocol string

const (
	ProtocolPipe        Protocol = "pipe"
	ProtocolUnixConnect Protocol = "unix-connect"
	ProtocolTCPConnect  Protocol = "tcp-connect"
)

type PipeConfig struct {
	Path string
}

type UnixConnectConfig struct {
	Addr string
}

type TCPConnectConfig struct {
	Addr string
	Port int
}

type Stream struct {
	ID   string
	Spec Spec
}

type Spec struct {
	Protocol Protocol `json:"protocol"`

	PipeConfig        *PipeConfig        `json:"pipeConfig,omitempty"`
	UnixConnectConfig *UnixConnectConfig `json:"unixConnectConfig,omitempty"`
	TCPConnectConfig  *TCPConnectConfig  `json:"tcpConnectConfig,omitempty"`
}
