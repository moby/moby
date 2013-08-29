package docker

type DaemonConfig struct {
	Pidfile        string
	GraphPath      string
	ProtoAddresses []string
	AutoRestart    bool
	EnableCors     bool
	Dns            []string
	EnableIptables bool
	BridgeIface    string
}
