#!/usr/bin/python

import sys, signal, time, os
import docker
import re
import subprocess
import json
import hashlib

ipv4match = re.compile(
    r'(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9]).' +
    r'(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9]).' +
    r'(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9]).' +
    r'(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])'
)

def which(name, defaultPath=""):
    if defaultPath and os.path.exists(defaultPath):
      return defaultPath
    for path in os.getenv("PATH").split(os.path.pathsep):
        fullPath = path + os.sep + name
        if os.path.exists(fullPath):
            return fullPath
        
def check_iptables(name, plist):
    replace = (':', ',')
    ports = []
    for port in plist:
        for r in replace:
            port = port.replace(r, ' ')

        p = port.split()
        ports.append((p[1], p[3]))

    # get the ingress sandbox's docker_gwbridge network IP.
    # published ports get DNAT'ed to this IP.
    ip = subprocess.check_output([ which("nsenter","/usr/bin/nsenter"), '--net=/var/run/docker/netns/ingress_sbox', which("bash", "/bin/bash"), '-c', 'ifconfig eth1 | grep \"inet\\ addr\" | cut -d: -f2 | cut -d\" \" -f1'])
    ip = ip.rstrip()

    for p in ports:
        rule = which("iptables", "/sbin/iptables") + '-t nat -C DOCKER-INGRESS -p tcp --dport {0} -j DNAT --to {1}:{2}'.format(p[1], ip, p[1])
        try:
            subprocess.check_output([which("bash", "/bin/bash"), "-c", rule])
        except subprocess.CalledProcessError as e:
            print "Service {0}: host iptables DNAT rule for port {1} -> ingress sandbox {2}:{3} missing".format(name, p[1], ip, p[1])

def get_namespaces(data, ingress=False):
    if ingress is True:
        return {"Ingress":"/var/run/docker/netns/ingress_sbox"}
    else:
        spaces =[]
        for c in data["Containers"]:
            sandboxes = {str(c) for c in data["Containers"]}

        containers = {}
        for s in sandboxes:
            spaces.append(str(cli.inspect_container(s)["NetworkSettings"]["SandboxKey"]))
            inspect = cli.inspect_container(s)
            containers[str(inspect["Name"])] = str(inspect["NetworkSettings"]["SandboxKey"])
        return containers


def check_network(nw_name, ingress=False):

    print "Verifying LB programming for containers on network %s" % nw_name

    data = cli.inspect_network(nw_name, verbose=True)

    if "Services" in data.keys():
        services = data["Services"]
    else:
        print "Network %s has no services. Skipping check" % nw_name
        return

    fwmarks = {str(service): str(svalue["LocalLBIndex"]) for service, svalue in services.items()}

    stasks = {}
    for service, svalue in services.items():
        if service == "":
            continue
        tasks = []
        for task in svalue["Tasks"]:
            tasks.append(str(task["EndpointIP"]))
        stasks[fwmarks[str(service)]] = tasks

        # for services in ingress network verify the iptables rules
        # that direct ingress (published port) to backend (target port)
        if ingress is True:
            check_iptables(service, svalue["Ports"])

    containers = get_namespaces(data, ingress)
    for container, namespace in containers.items():
        print "Verifying container %s..." % container
        ipvs = subprocess.check_output([which("nsenter","/usr/bin/nsenter"), '--net=%s' % namespace, which("ipvsadm","/usr/sbin/ipvsadm"), '-ln'])

        mark = ""
        realmark = {}
        for line in ipvs.splitlines():
            if "FWM" in line:
                mark = re.findall("[0-9]+", line)[0]
                realmark[str(mark)] = []
            elif "->" in line:
                if mark == "":
                    continue
                ip = ipv4match.search(line)
                if ip is not None:
                    realmark[mark].append(format(ip.group(0)))
            else:
                mark = ""
        for key in realmark.keys():
            if key not in stasks:
                print "LB Index %s" % key, "present in IPVS but missing in docker daemon"
                del realmark[key]

        for key in stasks.keys():
            if key not in realmark:
                print "LB Index %s" % key, "present in docker daemon but missing in IPVS"
                del stasks[key]

        for key in realmark:
            service = "--Invalid--"
            for sname, idx in fwmarks.items():
                if key == idx:
                    service = sname
            if len(set(realmark[key])) != len(set(stasks[key])):
                print "Incorrect LB Programming for service %s" % service
                print "control-plane backend tasks:"
                for task in stasks[key]:
                    print task
                print "kernel IPVS backend tasks:"
                for task in realmark[key]:
                    print task
            else:
                print "service %s... OK" % service

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print 'Usage: ssd.py network-name [gossip-consistency]'
        sys.exit()

    cli = docker.APIClient(base_url='unix://var/run/docker.sock', version='auto')
    if len(sys.argv) == 3:
        command = sys.argv[2]
    else:
        command = 'default'

    if command == 'gossip-consistency':
        cspec = docker.types.ContainerSpec(
            image='sanimej/ssd',
            args=[sys.argv[1], 'gossip-hash'],
            mounts=[docker.types.Mount('/var/run/docker.sock', '/var/run/docker.sock', type='bind')]
        )
        mode = docker.types.ServiceMode(
            mode='global'
        )
        task_template = docker.types.TaskTemplate(cspec)

        cli.create_service(task_template, name='gossip-hash', mode=mode)
        #TODO change to a deterministic way to check if the service is up.
        time.sleep(5)
        output = cli.service_logs('gossip-hash', stdout=True, stderr=True, details=True)
        for line in output:
            print("Node id: %s gossip hash %s" % (line[line.find("=")+1:line.find(",")], line[line.find(" ")+1:]))
        if cli.remove_service('gossip-hash') is not True:
            print("Deleting gossip-hash service failed")
    elif command == 'gossip-hash':
        data = cli.inspect_network(sys.argv[1], verbose=True)
        services = data["Services"]
        md5 = hashlib.md5()
        entries = []
        for service, value in services.items():
            entries.append(service)
            entries.append(value["VIP"])
            for task in value["Tasks"]:
                for key, val in task.items():
                    if isinstance(val, dict):
                        for k, v in val.items():
                            entries.append(v)
                    else:
                        entries.append(val)
        entries.sort()
        for e in entries:
            md5.update(e)
        print(md5.hexdigest())
        sys.stdout.flush()
        while True:
           signal.pause()
    elif command == 'default':
        if sys.argv[1] == "ingress":
            check_network("ingress", ingress=True)
        else:
            check_network(sys.argv[1])
            check_network("ingress", ingress=True)
