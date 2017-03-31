Proposal: Dockerized Application Configuration Management

* Version: 0.1
* Created: 2015-02-10
* Author: [yeasy@github](github.com/yeasy)

This proposal, targets the problems of application configuration information management in Docker, by assuming in future cloud enviroments, all configurations should be managed by a Configuration Information DataBase (CiB), rather than single configuration file per application.

# Background
Before the emergence of cloud computing, applications run in hosts or virtual machines directly. The only way to control the application behavior is the configuration file. For example, if we want to change the service port for an Apache instance, we have to set the port value in the Apache's configuration file, and try to reload or restart the application instance.

During those operations, service errors are easy to happen due to mistakes in the configuration file, even because of improper formatting.

In order to help manage configuration files and reduce the risks, there're several tools such as Ceph and Puppet, who can deploy applications' configuration file in central management with predefined templates. However, this way is still focusing on files management, not the configuration files. Hence, Devopers still need to handle lots of works, such as writing templates for different types or versions of applications, and manage them manually in the central file store.

# Problem Statement

Since more and more applications are deployed and running inside Cloud data centers, there're quite distinct requirements emerging on how to create, deploy and run a cloud-style application. [12 factors](http://12factor.net/) are summarized as a good reference on how to design such cloud-style applications, which suggest storing configuration in the environment.

Thus the main problems for current configuration management can be thought in three-folds:
* 1. How to deliver configuration information to the local application?
* 2. How to store configuration information for all applications?
* 3. How to sync information between the local application and the central store?

For question 1, we believe in future most cloud-style applications will run inside a Docker container, hence we should consider how to set configuration information efficiently to a dockerized application.

For question 2, we believe the current configuration files based storage is not an effective way, especially on the visualization, updating and immigration.

Question 3 can be solved using many existing ways such as RPC or remote API. 

Here we propose a configuration management framework for those problems.

# Proposal

## Centrial configuration Information DB
A DB that at least provides:

* Namespace to store configuration information for each application instance
* RESTful API to access (Create, Read, Update, Delete) the configuration information.

[Etcd](https://github.com/coreos/etcd) is a recommended implementation.

## Docker's `env` Parameter

As all configuration information should be set inside environment variables, as in [12 factors](http://12factor.net/), we also recommend using similar `env` series parameters to set application configurations.

Docker utilizes `--env=...` parameter to pass environment parameters to the inside application before running, also supports a `--env-file=...` parameter to read in a line delimited file of environment variables.

We recommend to add a new parameter, e.g., `--env-db=URI` or `--cfg-db=URI`  to specify where the configuration information store. Docker should read those information and provide it to the inside application.

## Dockerfile's `ENV` instruction
Dockerfile utilizes an `ENV` instruction to set each environment variables implicitly before creating the application images.

We propose to add a new instruction, e.g., `ENV_DB` or `CFG_DB` to specify where the configuration information store. 

An example looks like
```
CFG_DB docker.com/cfg_db/apache/v2.0/instance_xxxx
```
When building the image, Docker should read those information from given URI and use this information to build the image.

# Other issues

## Support Full CRUD Actions
We also consider how to implement the write actions from the client to the central db service. Docker should have the capability to write back those config information to the db service when running containers or building images.

## Support Orchiestration and Managment Tools
Container orchestration and management tools such as Fig can support the new style configuration framework easily.