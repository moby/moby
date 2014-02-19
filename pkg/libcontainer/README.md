## libcontainer - reference implementation for containers

#### playground


Use the cli package to test out functionality

First setup a container configuration.  You will need a root fs, better go the path to a
stopped docker container and use that.


```json
{
    "id": "koye",
    "namespace_pid": 12265,
    "command": {
        "args": [
            "/bin/bash"
        ],
        "environment": [
            "HOME=/",
            "PATH=PATH=$PATH:/bin:/usr/bin:/sbin:/usr/sbin",
            "container=docker",
            "TERM=xterm"
        ]
    },
    "rootfs": "/root/development/gocode/src/github.com/docker/libcontainer/namespaces/ubuntu",
    "network": null,
    "user": "",
    "working_dir": "",
    "namespaces": [
        "NEWNET",
        "NEWIPC",
        "NEWNS",
        "NEWPID",
        "NEWUTS"
    ],
    "capabilities": [
        "SETPCAP",
        "SYS_MODULE",
        "SYS_RAWIO",
        "SYS_PACCT",
        "SYS_ADMIN",
        "SYS_NICE",
        "SYS_RESOURCE",
        "SYS_TIME",
        "SYS_TTY_CONFIG",
        "MKNOD",
        "AUDIT_WRITE",
        "AUDIT_CONTROL",
        "MAC_OVERRIDE",
        "MAC_ADMIN"
    ]
}
```

After you have a json file and a rootfs path to use just run:
`./cli exec container.json`


If you want to attach to an existing namespace just use the same json
file with the container still running and do:
`./cli execin container.json`
