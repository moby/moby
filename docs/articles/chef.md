<!--[metadata]>
+++
title = "Using Chef"
description = "Installation and using Docker via Chef"
keywords = ["chef, installation, usage, docker,  documentation"]
[menu.main]
parent = "smn_third_party"
+++
<![end-metadata]-->

# Using Chef

> **Note**:
> Please note this is a community contributed installation path.

## Requirements

To use this guide you'll need a working installation of
[Chef](https://www.chef.io/). This cookbook supports a variety of
operating systems.

## Installation

The cookbook is available on the [Chef Supermarket](https://supermarket.chef.io/cookbooks/docker) and can be
installed using your favorite cookbook dependency manager.

The source can be found on
[GitHub](https://github.com/someara/chef-docker).

Usage
-----
- Add ```depends 'docker', '~> 2.0'``` to your cookbook's metadata.rb
- Use resources shipped in cookbook in a recipe, the same way you'd
  use core Chef resources (file, template, directory, package, etc).

```ruby
docker_service 'default' do
  action [:create, :start]
end

docker_image 'busybox' do
  action :pull
end

docker_container 'an echo server' do
  repo 'busybox'
  port '1234:1234'
  command "nc -ll -p 1234 -e /bin/cat"
end
```

## Getting Started
Here's a quick example of pulling the latest image and running a
container with exposed ports.

```ruby
# Pull latest image
docker_image 'nginx' do
  tag 'latest'
  action :pull
end

# Run container exposing ports
docker_container 'my_nginx' do
  repo 'nginx'
  tag 'latest'
  port '80:80'
  binds [ '/some/local/files/:/etc/nginx/conf.d' ]
  host_name 'www'
  domain_name 'computers.biz'
  env 'FOO=bar'
  subscribes :redeploy, 'docker_image[nginx]'
end
```
