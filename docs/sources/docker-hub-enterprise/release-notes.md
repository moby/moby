page_title: Docker Hub Enterprise: Release notes
page_description: Release notes for Docker Hub Enterprise
page_keywords: docker, documentation, about, technology, understanding, enterprise, hub, registry, release

# Release Notes

## Docker Hub Enterprise

### DHE 1.0.1
(11 May 2015)

- Addresses compatibility issue with 1.6.1 CS Docker Engine

### DHE 1.0.0
(23 Apr 2015)

- First release

## Commercialy Supported Docker Engine

### CS Docker Engine 1.6.2-cs5

For customers running Docker Engine on [supported versions of RedHat Enterprise
Linux](https://www.docker.com/enterprise/support/) with [SELinux
enabled](https://access.redhat.com/documentation/en-US/Red_Hat_Enterprise_Linux/
6/html/Security-Enhanced_Linux/sect-Security-Enhanced_Linux-Working_with_SELinux
-Enabling_and_Disabling_SELinux.html), the `docker build` and `docker run`
commands will fail because bind mounted volumes or files are not accessible. As
a result, customers with SELinux enabled cannot use these commands in their
environment. By installing Docker Engine 1.6.2-cs5, customers can run with
SELinux enabled and run these commands on their supported operating system.

**Affected Versions**: Docker Engine: 1.6.x-cs1 through 1.6.x-cs4

It is **highly recommended** that all customers running Docker Engine 1.6.x-cs1
through 1.6.x-cs4 update to this release. 

#### How to workaround this issue

Customers who do not install this update have two options. The
first option, is to disable SELinux. This is *not recommended* for production
systems where SELinux is required.

The second option is to pass the following parameter in to `docker run`. 
  
  	     --security-opt=label:type:docker_t

This parameter cannot be passed to the `docker build` command.

#### Upgrade notes 

If you are running with SELinux enabled, previous Docker Engine releases allowed
you to bind mount additional volumes or files inside the container as follows:

		$ docker run -it -v /home/user/foo.txt:/foobar.txt:ro

In the 1.6.2-cs5 release, you must ensure additional bind mounts have the correct
SELinux context. As an example, if you want to mount `foobar.txt` as read only
into the container, do the following to create and test your bind mount:

1. Add the `z` option to the bind mount when you specify `docker run`.

		$ docker run -it -v /home/user/foo.txt:/foobar.txt:ro,z

2. Exec into your new container.  

	For example, if your container is `bashful_curie` open a shell on the
	container:
		
		$ docker exec -it bashful_curie bash

3. Use the `cat` command to check the permissions on the mounted file.

		$ cat /foobar.txt
		the contents of foobar appear

	If you see the file's contents, your mount succeeded. If you receive a
	`Permission denied` message and/or the `/var/log/audit/audit.log` file on your
	Docker host contains an AVC Denial message, the mount did not succeed.

		type=AVC msg=audit(1432145409.197:7570): avc:  denied  { read } for  pid=21167 comm="cat" name="foobar.txt" dev="xvda2" ino=17704136 scontext=system_u:system_r:svirt_lxc_net_t:s0:c909,c965 tcontext=unconfined_u:object_r:user_home_t:s0 tclass=file
	
	Recheck your command line to make sure you passed in the `z` option.

### CS Docker Engine 1.6.2
(13 May 2015)

Fix mount regression for `/sys`.


### CS Docker Engine 1.6.1
(11 May 2015)

Docker Engine version 1.6.1 has been released to address several vulnerabilities
and is immediately available for all supported platforms. Users are advised to
upgrade existing installations of the Docker Engine and use 1.6.1 for new installations.

It should be noted that each of the vulnerabilities allowing privilege escalation
may only be exploited by a malicious Dockerfile or image.  Users are advised to
run their own images and/or images built by trusted parties, such as those in
the official images library.

Please send any questions to security@docker.com.


#### **[CVE-2015-3629](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2015-3629) Symlink traversal on container respawn allows local privilege escalation**

Libcontainer version 1.6.0 introduced changes which facilitated a mount namespace
breakout upon respawn of a container. This allowed malicious images to write
files to the host system and escape containerization.

Libcontainer and Docker Engine 1.6.1 have been released to address this
vulnerability. Users running untrusted images are encouraged to upgrade Docker Engine.

Discovered by Tõnis Tiigi.


#### **[CVE-2015-3627](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2015-3627) Insecure opening of file-descriptor 1 leading to privilege escalation**

The file-descriptor passed by libcontainer to the pid-1 process of a container
has been found to be opened prior to performing the chroot, allowing insecure
open and symlink traversal. This allows malicious container images to trigger
a local privilege escalation.

Libcontainer and Docker Engine 1.6.1 have been released to address this
vulnerability. Users running untrusted images are encouraged to upgrade
Docker Engine.

Discovered by Tõnis Tiigi.

#### **[CVE-2015-3630](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2015-3630) Read/write proc paths allow host modification & information disclosure**

Several paths underneath /proc were writable from containers, allowing global
system manipulation and configuration. These paths included `/proc/asound`,
`/proc/timer_stats`, `/proc/latency_stats`, and `/proc/fs`.

By allowing writes to `/proc/fs`, it has been noted that CIFS volumes could be
forced into a protocol downgrade attack by a root user operating inside of a
container. Machines having loaded the timer_stats module were vulnerable to
having this mechanism enabled and consumed by a container.

We are releasing Docker Engine 1.6.1 to address this vulnerability. All
versions up to 1.6.1 are believed vulnerable. Users running untrusted
images are encouraged to upgrade.

Discovered by Eric Windisch of the Docker Security Team.

#### **[CVE-2015-3631](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2015-3631) Volume mounts allow LSM profile escalation**

By allowing volumes to override files of `/proc` within a mount namespace, a user
could specify arbitrary policies for Linux Security Modules, including setting
an unconfined policy underneath AppArmor, or a `docker_t` policy for processes
managed by SELinux. In all versions of Docker up until 1.6.1, it is possible for
malicious images to configure volume mounts such that files of proc may be overridden.

We are releasing Docker Engine 1.6.1 to address this vulnerability. All versions
up to 1.6.1 are believed vulnerable. Users running untrusted images are encouraged
to upgrade.

Discovered by Eric Windisch of the Docker Security Team.

#### **AppArmor policy improvements**

The 1.6.1 release also marks preventative additions to the AppArmor policy.
Recently, several CVEs against the kernel have been reported whereby mount
namespaces could be circumvented through the use of the sys_mount syscall from
inside of an unprivileged Docker container. In all reported cases, the
AppArmor policy included in libcontainer and shipped with Docker has been
sufficient to deflect these attacks. However, we have deemed it prudent to
proactively tighten the policy further by outright denying the use of the
`sys_mount` syscall.

Because this addition is preventative, no CVE-ID is requested.

### CS Docker Engine 1.6.0
(23 Apr 2015)

- First release, please see the [Docker Engine 1.6.0 Release notes](/release-notes/)
  for more details.
