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
