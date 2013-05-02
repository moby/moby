# -*- mode: ruby -*-
# vi: set ft=ruby :

BOX_NAME = "ubuntu"
BOX_URI = "http://files.vagrantup.com/precise64.box"
PPA_KEY = "E61D797F63561DC6"

Vagrant::Config.run do |config|
  # Setup virtual machine box. This VM configuration code is always executed.
  config.vm.box = BOX_NAME
  config.vm.box_url = BOX_URI
  # Add docker PPA key to the local repository and install docker
  pkg_cmd = "apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys #{PPA_KEY}; "
  pkg_cmd << "echo 'deb http://ppa.launchpad.net/dotcloud/lxc-docker/ubuntu precise main' >>/etc/apt/sources.list; "
  pkg_cmd << "apt-get update -qq; apt-get install -q -y lxc-docker"
  if ARGV.include?("--provider=aws".downcase)
    # Add AUFS dependency to amazon's VM
    pkg_cmd << "; apt-get install linux-image-extra-3.2.0-40-virtual"
  end
  config.vm.provision :shell, :inline => pkg_cmd
end

# Providers were added on Vagrant >= 1.1.0
Vagrant::VERSION >= "1.1.0" and Vagrant.configure("2") do |config|
  config.vm.provider :aws do |aws, override|
    config.vm.box = "dummy"
    config.vm.box_url = "https://github.com/mitchellh/vagrant-aws/raw/master/dummy.box"
    aws.access_key_id = ENV["AWS_ACCESS_KEY_ID"]
    aws.secret_access_key = ENV["AWS_SECRET_ACCESS_KEY"]
    aws.keypair_name = ENV["AWS_KEYPAIR_NAME"]
    override.ssh.private_key_path = ENV["AWS_SSH_PRIVKEY"]
    override.ssh.username = "ubuntu"
    aws.region = "us-east-1"
    aws.ami = "ami-d0f89fb9"
    aws.instance_type = "t1.micro"
  end

  config.vm.provider :rackspace do |rs|
    config.vm.box = "dummy"
    config.vm.box_url = "https://github.com/mitchellh/vagrant-rackspace/raw/master/dummy.box"
    config.ssh.private_key_path = ENV["RS_PRIVATE_KEY"]
    rs.username = ENV["RS_USERNAME"]
    rs.api_key  = ENV["RS_API_KEY"]
    rs.public_key_path = ENV["RS_PUBLIC_KEY"]
    rs.flavor   = /512MB/
    rs.image    = /Ubuntu/
  end

  config.vm.provider :virtualbox do |vb|
    config.vm.box = BOX_NAME
    config.vm.box_url = BOX_URI
  end
end
