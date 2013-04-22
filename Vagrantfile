# -*- mode: ruby -*-
# vi: set ft=ruby :

def v10(config)
  config.vm.box = 'precise64'
  config.vm.box_url = 'http://files.vagrantup.com/precise64.box'

  # Install ubuntu packaging dependencies and create ubuntu packages
  config.vm.provision :shell, :inline => "echo 'deb http://ppa.launchpad.net/dotcloud/lxc-docker/ubuntu precise main' >>/etc/apt/sources.list"
  config.vm.provision :shell, :inline => 'export DEBIAN_FRONTEND=noninteractive; apt-get -qq update; apt-get install -qq -y --force-yes lxc-docker'
end

Vagrant::VERSION < "1.1.0" and Vagrant::Config.run do |config|
  v10(config)
end

Vagrant::VERSION >= "1.1.0" and Vagrant.configure("1") do |config|
  v10(config)
end

Vagrant::VERSION >= "1.1.0" and Vagrant.configure("2") do |config|
  config.vm.provider :aws do |aws|
    config.vm.box = "dummy"
    config.vm.box_url = "https://github.com/mitchellh/vagrant-aws/raw/master/dummy.box"
    aws.access_key_id = ENV["AWS_ACCESS_KEY_ID"]
    aws.secret_access_key =     ENV["AWS_SECRET_ACCESS_KEY"]
    aws.keypair_name = ENV["AWS_KEYPAIR_NAME"]
    aws.ssh_private_key_path = ENV["AWS_SSH_PRIVKEY"]
    aws.region = "us-east-1"
    aws.ami = "ami-d0f89fb9"
    aws.ssh_username = "ubuntu"
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
    config.vm.box = 'precise64'
    config.vm.box_url = 'http://files.vagrantup.com/precise64.box'
  end
end

Vagrant::VERSION >= "1.2.0" and Vagrant.configure("2") do |config|
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
    config.vm.box = 'precise64'
    config.vm.box_url = 'http://files.vagrantup.com/precise64.box'
  end

end
