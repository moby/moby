package main

// SystemdTemplateOpts is opts struct for executing systemdTemplate
type SystemdTemplateOpts struct {
	BinDir            string
	DockerdRootlessSh string
	Flags             string
}

const systemdTemplate = `[Unit]
Description=Docker Application Container Engine (Rootless)
Documentation=https://docs.docker.com/engine/security/rootless/

[Service]
Environment=PATH={{ .BinDir }}:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ExecStart={{ .BinDir }}/{{ .DockerdRootlessSh }} {{ .Flags }}
ExecReload=/bin/kill -s HUP $MAINPID
TimeoutSec=0
RestartSec=2
Restart=always
StartLimitBurst=3
StartLimitInterval=60s
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
Delegate=yes
Type=simple

[Install]
WantedBy=default.target
`

const systemdUnit = "docker.service"
