## nsinit

`nsinit` is a cli application which demonstrates the use of libcontainer.  
It is able to spawn new containers or join existing containers.  

### How to build?

First to add the `libcontainer/vendor` into your GOPATH. It's because something related with this [issue](https://github.com/docker/libcontainer/issues/210).

```
export GOPATH=$GOPATH:/your/path/to/libcontainer/vendor
```

Then get into the nsinit folder and get the imported file. Use `make` command to make the nsinit binary.

```
cd libcontainer/nsinit
go get
make
```

We have finished compiling the nsinit package, but a root filesystem must be provided for use along with a container configuration file.

Choose a proper place to run your container. For example we use `/busybox`.

```
mkdir /busybox 
curl -sSL 'https://github.com/jpetazzo/docker-busybox/raw/buildroot-2014.11/rootfs.tar' | tar -xC /busybox
```

Then you may need to write a configure file named `container.json` in the `/busybox` folder.
Environment, networking, and different capabilities for the container are specified in this file.
The configuration is used for each process executed inside the container
See the `sample_configs` folder for examples of what the container configuration should look like.

```
cp libcontainer/sample_configs/minimal.json /busybox/container.json
cd /busybox
```

Now the nsinit is ready to work.
To execute `/bin/bash` in the current directory as a container just run the following **as root**:
```bash
nsinit exec --tty /bin/bash
```

If you wish to spawn another process inside the container while your 
current bash session is running, run the same command again to 
get another bash shell (or change the command).  If the original 
process (PID 1) dies, all other processes spawned inside the container 
will be killed and the namespace will be removed. 

You can identify if a process is running in a container by 
looking to see if `state.json` is in the root of the directory.
   
You may also specify an alternate root place where 
the `container.json` file is read and where the `state.json` file will be saved.
