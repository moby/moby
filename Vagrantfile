# -*- mode: ruby -*-
# vi: set ft=ruby :

BOX_NAME = ENV['BOX_NAME'] || "ubuntu"
BOX_URI = ENV['BOX_URI'] || "http://files.vagrantup.com/precise64.box"
VF_BOX_URI = ENV['BOX_URI'] || "http://files.vagrantup.com/precise64_vmware_fusion.box"
AWS_BOX_URI = ENV['BOX_URI'] || "https://github.com/mitchellh/vagrant-aws/raw/master/dummy.box"
AWS_REGION = ENV['AWS_REGION'] || "us-east-1"
AWS_AMI = ENV['AWS_AMI'] || "ami-69f5a900"
AWS_INSTANCE_TYPE = ENV['AWS_INSTANCE_TYPE'] || 't1.micro'

FORWARD_DOCKER_PORTS = ENV['FORWARD_DOCKER_PORTS']

SSH_PRIVKEY_PATH = ENV["SSH_PRIVKEY_PATH"]

# A script to upgrade from the 12.04 kernel to the raring backport kernel (3.8)
# and install docker.
$script = <<SCRIPT
# The username to add to the docker group will be passed as the first argument
# to the script.  If nothing is passed, default to "vagrant".
user="$1"
if [ -z "$user" ]; then
    user=vagrant
fi

# Adding an apt gpg key is idempotent.
wget -q -O - https://get.docker.io/gpg | apt-key add -

# Creating the docker.list file is idempotent, but it may overwrite desired
# settings if it already exists.  This could be solved with md5sum but it
# doesn't seem worth it.
echo 'deb http://get.docker.io/ubuntu docker main' > \
    /etc/apt/sources.list.d/docker.list

# Update remote package metadata.  'apt-get update' is idempotent.
apt-get update -q

# Install docker.  'apt-get install' is idempotent.
apt-get install -q -y lxc-docker

usermod -a -G docker "$user"

tmp=`mktemp -q` && {
    # Only install the backport kernel, don't bother upgrading if the backport is
    # already installed.  We want parse the output of apt so we need to save it
    # with 'tee'.  NOTE: The installation of the kernel will trigger dkms to
    # install vboxguest if needed.
    apt-get install -q -y --no-upgrade linux-image-generic-lts-raring | \
        tee "$tmp"

    # Parse the number of installed packages from the output
    NUM_INST=`awk '$2 == "upgraded," && $4 == "newly" { print $3 }' "$tmp"`
    rm "$tmp"
}

# If the number of installed packages is greater than 0, we want to reboot (the
# backport kernel was installed but is not running).
if [ "$NUM_INST" -gt 0 ];
then
    echo "Rebooting down to activate new kernel."
    echo "/vagrant will not be mounted.  Use 'vagrant halt' followed by"
    echo "'vagrant up' to ensure /vagrant is mounted."
    shutdown -r now
fi
SCRIPT

# We need to install the virtualbox guest additions *before* we do the normal
# docker installation.  As such this script is prepended to the common docker
# install script above.  This allows the install of the backport kernel to
# trigger dkms to build the virtualbox guest module install.
$vbox_script = <<VBOX_SCRIPT + $script
# Install the VirtualBox guest additions if they aren't already installed.
if [ ! -d /opt/VBoxGuestAdditions-4.3.2/ ]; then
    # Update remote package metadata.  'apt-get update' is idempotent.
    apt-get update -q

    # Kernel Headers and dkms are required to build the vbox guest kernel
    # modules.
    apt-get install -q -y linux-headers-generic-lts-raring dkms

    echo 'Downloading VBox Guest Additions...'
    wget -cq http://dlc.sun.com.edgesuite.net/virtualbox/4.3.2/VBoxGuestAdditions_4.3.2.iso

    mount -o loop,ro /home/vagrant/VBoxGuestAdditions_4.3.2.iso /mnt
    /mnt/VBoxLinuxAdditions.run --nox11
    umount /mnt
fi
VBOX_SCRIPT

Vagrant::Config.run do |config|
  # Setup virtual machine box. This VM configuration code is always executed.
  config.vm.box = BOX_NAME
  config.vm.box_url = BOX_URI

  # Use the specified private key path if it is specified and not empty.
  if SSH_PRIVKEY_PATH
      config.ssh.private_key_path = SSH_PRIVKEY_PATH
  end

  config.ssh.forward_agent = true
end

# Providers were added on Vagrant >= 1.1.0
#
# NOTE: The vagrant "vm.provision" appends its arguments to a list and executes
# them in order.  If you invoke "vm.provision :shell, :inline => $script"
# twice then vagrant will run the script two times.  Unfortunately when you use
# providers and the override argument to set up provisioners (like the vbox
# guest extensions) they 1) don't replace the other provisioners (they append
# to the end of the list) and 2) you can't control the order the provisioners
# are executed (you can only append to the list).  If you want the virtualbox
# only script to run before the other script, you have to jump through a lot of
# hoops.
#
# Here is my only repeatable solution: make one script that is common ($script)
# and another script that is the virtual box guest *prepended* to the common
# script.  Only ever use "vm.provision" *one time* per provider.  That means
# every single provider has an override, and every single one configures
# "vm.provision".  Much saddness, but such is life.
Vagrant::VERSION >= "1.1.0" and Vagrant.configure("2") do |config|
  config.vm.provider :aws do |aws, override|
    username = "ubuntu"
    override.vm.box_url = AWS_BOX_URI
    override.vm.provision :shell, :inline => $script, :args => username
    aws.access_key_id = ENV["AWS_ACCESS_KEY"]
    aws.secret_access_key = ENV["AWS_SECRET_KEY"]
    aws.keypair_name = ENV["AWS_KEYPAIR_NAME"]
    override.ssh.username = username
    aws.region = AWS_REGION
    aws.ami    = AWS_AMI
    aws.instance_type = AWS_INSTANCE_TYPE
  end

  config.vm.provider :rackspace do |rs, override|
    override.vm.provision :shell, :inline => $script
    rs.username = ENV["RS_USERNAME"]
    rs.api_key  = ENV["RS_API_KEY"]
    rs.public_key_path = ENV["RS_PUBLIC_KEY"]
    rs.flavor   = /512MB/
    rs.image    = /Ubuntu/
  end

  config.vm.provider :vmware_fusion do |f, override|
    override.vm.box_url = VF_BOX_URI
    override.vm.synced_folder ".", "/vagrant", disabled: true
    override.vm.provision :shell, :inline => $script
    f.vmx["displayName"] = "docker"
  end

  config.vm.provider :virtualbox do |vb, override|
    override.vm.provision :shell, :inline => $vbox_script
    vb.customize ["modifyvm", :id, "--natdnshostresolver1", "on"]
    vb.customize ["modifyvm", :id, "--natdnsproxy1", "on"]
  end
end

# If this is a version 1 config, virtualbox is the only option.  A version 2
# config would have already been set in the above provider section.
Vagrant::VERSION < "1.1.0" and Vagrant::Config.run do |config|
  config.vm.provision :shell, :inline => $vbox_script
end

if !FORWARD_DOCKER_PORTS.nil?
  Vagrant::VERSION < "1.1.0" and Vagrant::Config.run do |config|
    (49000..49900).each do |port|
      config.vm.forward_port port, port
    end
  end

  Vagrant::VERSION >= "1.1.0" and Vagrant.configure("2") do |config|
    (49000..49900).each do |port|
      config.vm.network :forwarded_port, :host => port, :guest => port
    end
  end
end
