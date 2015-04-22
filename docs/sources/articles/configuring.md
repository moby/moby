page_title: Configuring Docker
page_description: Configuring the Docker daemon on various distributions
page_keywords: docker, daemon, configuration

# Configuring Docker on various distributions

After successfully installing Docker, the `docker` daemon runs with it's default
configuration. You can configure the `docker` daemon by passing configuration
flags to it directly when you start it.  

In a production environment, system administrators typically configure the
`docker` daemon to start and stop according to an organization's requirements.  In most
cases, the system administrator configures a process manager such as `SysVinit`, `Upstart`,
or `systemd` to manage the `docker` daemon's start and stop.

Some of the daemon's options are:

| Flag                  | Description                                               |
|-----------------------|-----------------------------------------------------------|
| `-D`, `--debug=false` | Enable or disable debug mode.  By default, this is false. |
| `-H`,`--host=[]`      | Daemon socket(s) to connect to.                           |
| `--tls=false`         | Enable or disable TLS. By default, this is false.         |

The command line reference has the [complete list of daemon flags](/reference/commandline/cli/#daemon).

## Direct Configuration

If you're running the `docker` daemon directly by running `docker -d` instead of using a process manager,
you can append the config options to the run command directly.


Here is a an example of running the `docker` daemon with config options:

    docker -d -D --tls=false -H tcp://0.0.0.0:2375

These options : 

- Enable `-D` (debug) mode 
- Set `tls` to false
- Listen for connections on `tcp://0.0.0.0:2375`


## Ubuntu

After successfully [installing Docker for Ubuntu](/installation/ubuntulinux/), you can check the
running status using Upstart in this way:

    $ sudo status docker
    docker start/running, process 989

You can start/stop/restart `docker` using

    $ sudo start docker

    $ sudo stop docker

    $ sudo restart docker


### Configuring Docker

You configure the `docker` daemon in the `/etc/default/docker` file on your
system.  You do this by specifying values in a `DOCKER_OPTS` variable. 
To configure Docker options:

1. Log into your system as a user with `sudo` or `root` privileges.

2. If you don't have one, create the `/etc/default/docker` file in your system. 

	Depending on how you installed Docker, you may already have this file.

3. Open the file with your favorite editor.

		$ sudo vi /etc/default/docker
		
4. Add a `DOCKER_OPTS` variable with the following options. These options are appended to the
`docker` daemon's run command.

	``` 
	 DOCKER_OPTS=" --dns 8.8.8.8 --dns 8.8.4.4 -D --tls=false -H tcp://0.0.0.0:2375 "
	```
	
These options : 

- Set `dns` server for all containers
- Enable `-D` (debug) mode 
- Set `tls` to false
- Listen for connections on `tcp://0.0.0.0:2375`
  
5. Save and close the file.

6. Restart the `docker` daemon.

 		 $ sudo restart docker

7. Verify that the `docker` daemon is running as specified wit the `ps` command.

		$ ps aux | grep docker | grep -v grep
