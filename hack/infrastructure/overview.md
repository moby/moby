# Docker Server Overview
This is an overview of the Docker infrastructure

## Docker Git Repo
The Docker source code lives on github.com under the dotCloud account.
https://github.com/dotcloud/docker

## DNS
We are using dyn.com for our DNS server for the docker.io domain. 
It is using the dotCloud account.

### DNS Redirect
We have a DNS redirect in dyn.com that will automatically redirect 
docker.io to www.docker.io

## email
Email is sent via  dotCloud account on MailGun.com

## CDN
We are using a CDN in front of some of the docker.io domains to help improve
proformance. The CDN is Cloudflare, using a Pro account. 

*This is currently disabled due to an issue with slow performance during pull
in some regions of the world.*

### CDN Domains
- www.docker.io
- test.docker.io
- registry-1.docker.io
- debug.docker.io
- cdn-registry-1.docker.io

## edge-docker.dotcloud.com
All of the Docker applications that live on dotCloud go through their own
load balancer, and this is where SSL is terminated as well.

## www.docker.io
This is hosted under the docker account on dotCloud's PaaS. 

### Source Code
The source code for the website lives here:
https://github.com/dotcloud/www.docker.io

## Docker Registry
The registry is where the image data is store.

### URL:
- registry-1.docker.io
- cdn-registry-1.docker.io

There are two urls, one is behind a CDN the other isn't this is because when
you pull, you pull from the CDN url, to help with pull speeds. We don't push
through the CDN as well, because it doesn't help us, so we bypass it.

### Data Store:
The data store for the registry is using Amazon S3 in a bucket under the docker
aws account.

### Source Code
The source code for the registry lives here: https://github.com/dotcloud/docker-registry

### Hosted:
Hosted on the Docker account on dotCloud's PaaS

## index.docker.io
This is the docker index, it stores all of the meta information about the 
docker images, but all data is stored in the registry.

### Source Code:
Not available

### Hosted:
Hosted on the Docker account on dotCloud's PaaS

## blog.docker.io
This is a wordpress based Docker blog.

### URL:
http://blog.docker.io

### Source Code:
https://github.com/dotcloud/blog.docker.io

## docs.docker.io
This is where all of the documentation for docker lives.

### Hosted:
This website is hosted on ReadTheDocs.org.

### Updates
These docs get automatically updated when the Docker repo on github has
new commits. It does this via a webhook.

### Proxy:
This is a simple dotcloud app, with its main and only purpose to forward
http (and https) requests to docker.readthedocs.org.

https://github.com/dotcloud/docker-docs-dotcloud-proxy

## get.docker.io
This is the docker repository where we store images

TODO: need more here. jerome?
