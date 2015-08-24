<!--[metadata]>
+++
title = "Joyent Triton Elastic Container Service"
description = "Installation instructions for Docker on the Joyent's Triton Elastic Container Service."
keywords = ["Docker, Docker documentation, installation, joyent, Triton, Joyent Public Cloud, Joyent Compute Service, Joyent Container Service"]
[menu.main]
parent = "smn_cloud"
+++
<![end-metadata]-->

Joyent's Triton Elastic Container Service for Docker uses the native Docker API and allows you to securely provision containers on bare metal.

## Using the Triton Elastic Container Service for Docker

1. Sign in or sign up for the [Joyent compute service](https://my.joyent.com/) and add your SSH key.

2. Install the Docker CLI tools on your laptop or wherever you do your work.

3. Configure the Docker CLI tools for use with Joyent:

The 'sdc-docker-setup.sh' script will help pull everything together and configure Docker clients.

Download the script:

```
curl -O https://raw.githubusercontent.com/joyent/sdc-docker/master/tools/sdc-docker-setup.sh
```

Now execute the script, substituting the correct values:

```
bash sdc-docker-setup.sh <CLOUDAPI_URL> <ACCOUNT_USERNAME> ~/.ssh/<PRIVATE_KEY_FILE>
```

Possible values for `<CLOUDAPI_URL>` include many of Joyent's data centers (all data centers will be available soon). The following table shows the CloudAPI URL for available data centers:

| CLOUDAPI_URL | Description |
| ------------ | ----------- |
| https://us-east-1.api.joyent.com | Joyent's us-east-1 data center. |
| https://us-sw-1.api.joyent.com | Joyent's us-sw-1 data center. |
| https://eu-ams-1.api.joyent.com | Joyent's eu-ams-1 (Amsterdam) data center. |

For example, if you created an account on [Joyent's hosted Triton service](https://www.joyent.com/triton), with the username `jill`, SSH key file `~/.ssh/sdc-docker.id_rsa`, and connecting to the US East-1 data center:

```
bash sdc-docker-setup.sh https://us-east-1.api.joyent.com jill ~/.ssh/sdc-docker.id_rsa
```

That should output something like the following:

```
Setting up Docker client for SDC using:
	CloudAPI:        https://us-east-1.api.joyent.com
	Account:         jill
	Key:             /Users/localuser/.ssh/sdc-docker.id_rsa

If you have a pass phrase on your key, the openssl command will
prompt you for your pass phrase now and again later.

Verifying CloudAPI access.
CloudAPI access verified.

Generating client certificate from SSH private key.
writing RSA key
Wrote certificate files to /Users/localuser/.sdc/docker/jill

Get Docker host endpoint from cloudapi.
Docker service endpoint is: tcp://us-east-1.docker.joyent.com:2376

* * *
Success. Set your environment as follows:

	export DOCKER_CERT_PATH=/Users/localuser/.sdc/docker/jill
	export DOCKER_HOST=tcp://us-east-1.docker.joyent.com:2376
	export DOCKER_TLS_VERIFY=1
```

Then you should be able to run 'docker info' and see your account name 'SDCAccount: jill' in the output.

Run those `export` commands in your shell and you should now be able to run `docker info`:

```
$ docker info
Containers: 0
Images: 0
Storage Driver: sdc
 SDCAccount: jill
Execution Driver: sdc-0.1.0
Operating System: SmartDataCenter
Name: us-east-1
```

## Start and manage containers

Using Joyent's Elastic Container Service for Docker is as simple as `docker run` and `docker ps`. Use the Docker CLI commands you already know to start and manage containers across the entire data center.

## Where to go next

Continue with the [Docker user guide](/userguide/), Joyent's [documentation](https://docs.joyent.com/public-cloud/containers/docker), our full [Docker API guide for Triton](https://apidocs.joyent.com/docker), or an example of how to [deploy a clustered database in Docker containers](https://www.joyent.com/blog/couchbase-in-docker-containers).