# Docker project infrastructure

This is an overview of the Docker infrastructure.

**Note: obviously, credentials should not be stored in this repository.**
However, when there are credentials, we should list how to obtain them
(e.g. who has them).


## Providers

This should be the list of all the entities providing some kind of
infrastructure service to the Docker project (either for free,
or paid by dotCloud).


Provider      | Service
--------------|-------------------------------------------------
AWS           | packages (S3 bucket), dotCloud PAAS, dev-env, ci
CloudFlare    | cdn
Digital Ocean | ci
dotCloud PAAS | website, index, registry, ssl, blog
DynECT        | dns (docker.io)            
GitHub        | repository
Linode        | dev-env
Mailgun       | outgoing e-mail            
ReadTheDocs   | docs

*Ordered-by: lexicographic*


## URLs

This should be the list of all the infrastructure-related URLs
and which service is handling them.

URL                                         | Service
--------------------------------------------|---------------------------------
 http://blog.docker.io/                     | blog
*http://cdn-registry-1.docker.io/           | registry (pull)
 http://debug.docker.io/                    | debug tool
 http://docs.docker.io/                     | docsproxy (proxy to readthedocs)
 http://docker-ci.dotcloud.com/             | ci
 http://docker.io/                          | redirect to www.docker.io (dynect)
 http://docker.readthedocs.org/             | docs
*http://get.docker.io/                      | packages
 https://github.com/dotcloud/docker         | repository
*https://index.docker.io/                   | index
 http://registry-1.docker.io/               | registry (push)
 http://staging-docker-ci.dotcloud.com/     | ci
*http://test.docker.io/                     | packages
*http://www.docker.io/                      | website

*Ordered-by: lexicographic*

**Note:** an asterisk in front of the URL means that it is cached by CloudFlare.


## Services

This should be the list of all services referenced above.

Service             | Maintainer(s)      | How to update    | Source
--------------------|--------------------|------------------|-------
blog                | @jbarbier          | dotcloud push    | https://github.com/dotcloud/blog.docker.io
cdn                 | @jpetazzo @samalba | cloudflare panel | N/A
ci                  | @mzdaniel          | See [docker-ci]  | See [docker-ci]
docs                | @metalivedev       | github webhook   | docker repo
docsproxy           | @dhrp              | dotcloud push    | https://github.com/dotcloud/docker-docs-dotcloud-proxy
index               | @kencochrane       | dotcloud push    | private
packages            | @jpetazzo          | hack/release     | docker repo
registry            | @samalba           | dotcloud push    | https://github.com/dotcloud/docker-registry
repository (github) | N/A                | N/A              | N/A
ssl (dotcloud)      | @jpetazzo          | dotcloud ops     | N/A
ssl (cloudflare)    | @jpetazzo          | cloudflare panel | N/A
website             | @dhrp              | dotcloud push    | https://github.com/dotcloud/www.docker.io

*Ordered-by: lexicographic*


[docker-ci]: docker-ci.rst
