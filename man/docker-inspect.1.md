% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-inspect - Return low-level information on a container or image

# SYNOPSIS
**docker inspect**
[**--help**]
[**-f**|**--format**[=*FORMAT*]]
CONTAINER|IMAGE [CONTAINER|IMAGE...]

# DESCRIPTION

This displays all the information available in Docker for a given
container or image. By default, this will render all results in a JSON
array. If a format is specified, the given template will be executed for
each result.

# OPTIONS
**--help**
    Print usage statement

**-f**, **--format**=""
    Format the output using the given go template.

# EXAMPLES

## Getting information on a container

To get information on a container use its ID or instance name:

    $ docker inspect 1eb5fabf5a03
    [{
        "AppArmorProfile": "",
        "Args": [],
        "Config": {
            "AttachStderr": false,
            "AttachStdin": false,
            "AttachStdout": false,
            "Cmd": [
                "/usr/sbin/nginx"
            ],
            "Domainname": "",
            "Entrypoint": null,
            "Env": [
                "HOME=/",
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ],
            "ExposedPorts": {
                "80/tcp": {}
            },
            "Hostname": "1eb5fabf5a03",
            "Image": "summit/nginx",
            "Labels": {
                "com.example.vendor": "Acme",
                "com.example.license": "GPL",
                "com.example.version": "1.0"
            },
            "MacAddress": "",
            "NetworkDisabled": false,
            "OnBuild": null,
            "OpenStdin": false,
            "StdinOnce": false,
            "Tty": true,
            "User": "",
            "Volumes": null,
            "WorkingDir": "",
        },
        "Created": "2014-04-04T21:33:52.02361335Z",
        "Driver": "devicemapper",
        "ExecDriver": "native-0.1",
        "ExecIDs": null,
        "HostConfig": {
            "Binds": null,
            "CapAdd": null,
            "CapDrop": null,
            "CgroupParent": "",
            "ContainerIDFile": "",
            "CpuShares": 512,
            "CpusetCpus": "0,1",
            "CpusetMems": "",
            "Devices": [],
            "Dns": null,
            "DnsSearch": null,
            "ExtraHosts": null,
            "IpcMode": "",
            "Links": null,
            "LogConfig": {
                "Config": null,
                "Type": "json-file"
            },
            "LxcConf": null,
            "Memory": 16777216,
            "MemorySwap": -1,
            "NetworkMode": "",
            "PidMode": "",
            "PortBindings": {
                "80/tcp": [
                    {
                        "HostIp": "0.0.0.0",
                        "HostPort": "80"
                    }
                ]
            },
            "Privileged": false,
            "PublishAllPorts": false,
            "ReadonlyRootfs": false,
            "RestartPolicy": {
                "MaximumRetryCount": 0,
                "Name": ""
            },
            "SecurityOpt": null,
            "Ulimits": null,
            "VolumesFrom": null
        }
        "HostnamePath": "/var/lib/docker/containers/1eb5fabf5a03807136561b3c00adcd2992b535d624d5e18b6cdc6a6844d9767b/hostname",
        "HostsPath": "/var/lib/docker/containers/1eb5fabf5a03807136561b3c00adcd2992b535d624d5e18b6cdc6a6844d9767b/hosts",
        "ID": "1eb5fabf5a03807136561b3c00adcd2992b535d624d5e18b6cdc6a6844d9767b",
        "Image": "df53773a4390e25936f9fd3739e0c0e60a62d024ea7b669282b27e65ae8458e6",
        "LogPath": "/var/lib/docker/containers/1eb5fabf5a03807136561b3c00adcd2992b535d624d5e18b6cdc6a6844d9767b/1eb5fabf5a03807136561b3c00adcd2992b535d624d5e18b6cdc6a6844d9767b-json.log",
        "MountLabel": "",
        "Name": "/ecstatic_ptolemy",
        "NetworkSettings": {
            "Bridge": "docker0",
            "Gateway": "172.17.42.1",
            "GlobalIPv6Address": "",
            "GlobalIPv6PrefixLen": 0,
            "IPAddress": "172.17.0.2",
            "IPPrefixLen": 16,
            "IPv6Gateway": "",
            "LinkLocalIPv6Address": "",
            "LinkLocalIPv6PrefixLen": 0,
            "MacAddress": "",
            "PortMapping": null,
            "Ports": {
                "80/tcp": [
                    {
                        "HostIp": "0.0.0.0",
                        "HostPort": "80"
                    }
                ]
            }
        },
        "Path": "/usr/sbin/nginx",
        "ProcessLabel": "",
        "ResolvConfPath": "/etc/resolv.conf",
        "RestartCount": 0,
        "State": {
            "Dead": false,
            "Error": "",
            "ExitCode": 0,
            "FinishedAt": "0001-01-01T00:00:00Z",
            "OOMKilled": false,
            "Paused": false,
            "Pid": 858,
            "Restarting": false,
            "Running": true,
            "StartedAt": "2014-04-04T21:33:54.16259207Z",
        },
        "Volumes": {},
        "VolumesRW": {},
    }

## Getting the IP address of a container instance

To get the IP address of a container use:

    $ docker inspect --format='{{.NetworkSettings.IPAddress}}' 1eb5fabf5a03
    172.17.0.2

## Listing all port bindings

One can loop over arrays and maps in the results to produce simple text
output:

    $ docker inspect --format='{{range $p, $conf := .NetworkSettings.Ports}} \
      {{$p}} -> {{(index $conf 0).HostPort}} {{end}}' 1eb5fabf5a03
      80/tcp -> 80

You can get more information about how to write a go template from:
http://golang.org/pkg/text/template/.

## Getting information on an image

Use an image's ID or name (e.g., repository/name[:tag]) to get information
on it.

    $ docker inspect fc1203419df2
    [{
        "Architecture": "amd64",
        "Author": "",
        "Comment": "",
        "Config": {
            "AttachStderr": false,
            "AttachStdin": false,
            "AttachStdout": false,
            "Cmd": [
                "make",
                "direct-test"
            ],
            "Domainname": "",
            "Entrypoint": [
                "/dind"
            ],
            "Env": [
                "PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            ],
            "ExposedPorts": null,
            "Hostname": "242978536a06",
            "Image": "c2b774c744afc5bea603b5e6c5218539e506649326de3ea0135182f299d0519a",
            "Labels": {},
            "MacAddress": "",
            "NetworkDisabled": false,
            "OnBuild": [],
            "OpenStdin": false,
            "StdinOnce": false,
            "Tty": false,
            "User": "",
            "Volumes": null,
            "WorkingDir": "/go/src/github.com/docker/libcontainer"
        },
        "Container": "1c00417f3812a96d3ebc29e7fdee69f3d586d703ab89c8233fd4678d50707b39",
        "ContainerConfig": {
            "AttachStderr": false,
            "AttachStdin": false,
            "AttachStdout": false,
            "Cmd": [
                "/bin/sh",
                "-c",
                "#(nop) CMD [\"make\" \"direct-test\"]"
            ],
            "Domainname": "",
            "Entrypoint": [
                "/dind"
            ],
            "Env": [
                "PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            ],
            "ExposedPorts": null,
            "Hostname": "242978536a06",
            "Image": "c2b774c744afc5bea603b5e6c5218539e506649326de3ea0135182f299d0519a",
            "Labels": {},
            "MacAddress": "",
            "NetworkDisabled": false,
            "OnBuild": [],
            "OpenStdin": false,
            "StdinOnce": false,
            "Tty": false,
            "User": "",
            "Volumes": null,
            "WorkingDir": "/go/src/github.com/docker/libcontainer"
        },
        "Created": "2015-04-07T05:34:39.079489206Z",
        "DockerVersion": "1.5.0-dev",
        "Id": "fc1203419df26ca82cad1dd04c709cb1b8a8a947bd5bcbdfbef8241a76f031db",
        "Os": "linux",
        "Parent": "c2b774c744afc5bea603b5e6c5218539e506649326de3ea0135182f299d0519a",
        "Size": 0,
        "VirtualSize": 613136466
    }]

# HISTORY
April 2014, originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
April 2015, updated by Qiang Huang <h.huangqiang@huawei.com>
