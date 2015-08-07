AppArmor security profiles for Docker
--------------------------------------

AppArmor (Application Armor) is a security module that allows a system
administrator to associate a security profile with each program. Docker
expects to find an AppArmor policy loaded and enforced.

Container profiles are loaded automatically by Docker. A profile
for the Docker Engine itself also exists and is installed
with the official *.deb* packages. Advanced users and package
managers may find the profile for */usr/bin/docker* underneath
[contrib/apparmor](https://github.com/docker/docker/tree/master/contrib/apparmor)
in the Docker Engine source repository.


Understand the policies
------------------------

The `docker-default` profile the default for running
containers. It is moderately protective while
providing wide application compatibility.

The system's standard `unconfined` profile inherits all
system-wide policies, applying path-based policies
intended for the host system inside of containers.
This was the default for privileged containers
prior to Docker 1.8.


Overriding the profile for a container
---------------------------------------

Users may override the AppArmor profile using the
`security-opt` option (per-container).

For example, the following explicitly specifies the default policy:

```
$ docker run --rm -it --security-opt apparmor:docker-default hello-world
```

