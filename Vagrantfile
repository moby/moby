# -*- mode: ruby -*-
# vi: set ft=ruby :

def v10(config)
  config.vm.box = "quantal64_3.5.0-25"
  config.vm.box_url = "http://get.docker.io/vbox/ubuntu/12.10/quantal64_3.5.0-25.box"

  config.vm.share_folder "v-data", "/opt/go/src/github.com/dotcloud/docker", File.dirname(__FILE__)

  # Ensure puppet is installed on the instance
  config.vm.provision :shell, :inline => "apt-get -qq update; apt-get install -y puppet"

  config.vm.provision :puppet do |puppet|
    puppet.manifests_path = "puppet/manifests"
    puppet.manifest_file  = "quantal64.pp"
    puppet.module_path = "puppet/modules"
  end
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
    aws.secret_access_key = ENV["AWS_SECRET_ACCESS_KEY"]
    aws.keypair_name = ENV["AWS_KEYPAIR_NAME"]
    aws.ssh_private_key_path = ENV["AWS_SSH_PRIVKEY"]
    aws.region = "us-east-1"
    aws.ami = "ami-ae9806c7"
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
    config.vm.box = "quantal64_3.5.0-25"
    config.vm.box_url = "http://get.docker.io/vbox/ubuntu/12.10/quantal64_3.5.0-25.box"
  end
end
