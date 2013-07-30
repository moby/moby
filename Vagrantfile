# -*- mode: ruby -*-
# vi: set ft=ruby :

BOX_NAME = ENV['BOX_NAME'] || "ubuntu"
BOX_URI = ENV['BOX_URI'] || "http://files.vagrantup.com/precise64.box"
VF_BOX_URI = ENV['BOX_URI'] || "http://files.vagrantup.com/precise64_vmware_fusion.box"
AWS_REGION = ENV['AWS_REGION'] || "us-east-1"
AWS_AMI    = ENV['AWS_AMI']    || "ami-d0f89fb9"
FORWARD_DOCKER_PORTS = ENV['FORWARD_DOCKER_PORTS']

Vagrant::Config.run do |config|
  # Setup virtual machine box. This VM configuration code is always executed.
  config.vm.box = BOX_NAME
  config.vm.box_url = BOX_URI
  config.vm.forward_port 4243, 4243

  # Provision docker and new kernel if deployment was not done
  if Dir.glob("#{File.dirname(__FILE__)}/.vagrant/machines/default/*/id").empty?
    # Add lxc-docker package
    pkg_cmd = "apt-get update -qq; apt-get install -q -y python-software-properties; " \
      "add-apt-repository -y ppa:dotcloud/lxc-docker; apt-get update -qq; " \
      "apt-get install -q -y lxc-docker; "
    # Listen on all interfaces so that the daemon is accessible from the host
    pkg_cmd << "sed -i -E 's|    /usr/bin/docker -d|    /usr/bin/docker -d -H 0.0.0.0|' /etc/init/docker.conf;"
    # Add X.org Ubuntu backported 3.8 kernel
    pkg_cmd << "add-apt-repository -y ppa:ubuntu-x-swat/r-lts-backport; " \
      "apt-get update -qq; apt-get install -q -y linux-image-3.8.0-19-generic; "
    # Add guest additions if local vbox VM
    is_vbox = true
    ARGV.each do |arg| is_vbox &&= !arg.downcase.start_with?("--provider") end
    if is_vbox
      pkg_cmd << "apt-get install -q -y linux-headers-3.8.0-19-generic dkms; " \
        "echo 'Downloading VBox Guest Additions...'; " \
        "wget -q http://dlc.sun.com.edgesuite.net/virtualbox/4.2.12/VBoxGuestAdditions_4.2.12.iso; "
      # Prepare the VM to add guest additions after reboot
      pkg_cmd << "echo -e 'mount -o loop,ro /home/vagrant/VBoxGuestAdditions_4.2.12.iso /mnt\n" \
        "echo yes | /mnt/VBoxLinuxAdditions.run\numount /mnt\n" \
          "rm /root/guest_additions.sh; ' > /root/guest_additions.sh; " \
        "chmod 700 /root/guest_additions.sh; " \
        "sed -i -E 's#^exit 0#[ -x /root/guest_additions.sh ] \\&\\& /root/guest_additions.sh#' /etc/rc.local; " \
        "echo 'Installation of VBox Guest Additions is proceeding in the background.'; " \
        "echo '\"vagrant reload\" can be used in about 2 minutes to activate the new guest additions.'; "
    end
    # Activate new kernel
    pkg_cmd << "shutdown -r +1; "
    config.vm.provision :shell, :inline => pkg_cmd
  end
end


# Providers were added on Vagrant >= 1.1.0
Vagrant::VERSION >= "1.1.0" and Vagrant.configure("2") do |config|
  config.vm.provider :aws do |aws, override|
    aws.access_key_id = ENV["AWS_ACCESS_KEY_ID"]
    aws.secret_access_key = ENV["AWS_SECRET_ACCESS_KEY"]
    aws.keypair_name = ENV["AWS_KEYPAIR_NAME"]
    override.ssh.private_key_path = ENV["AWS_SSH_PRIVKEY"]
    override.ssh.username = "ubuntu"
    aws.region = AWS_REGION
    aws.ami    = AWS_AMI
    aws.instance_type = "t1.micro"
  end

  config.vm.provider :rackspace do |rs|
    config.ssh.private_key_path = ENV["RS_PRIVATE_KEY"]
    rs.username = ENV["RS_USERNAME"]
    rs.api_key  = ENV["RS_API_KEY"]
    rs.public_key_path = ENV["RS_PUBLIC_KEY"]
    rs.flavor   = /512MB/
    rs.image    = /Ubuntu/
  end

  config.vm.provider :vmware_fusion do |f, override|
    override.vm.box = BOX_NAME
    override.vm.box_url = VF_BOX_URI
    override.vm.synced_folder ".", "/vagrant", disabled: true
    f.vmx["displayName"] = "docker"
  end

  config.vm.provider :virtualbox do |vb|
    config.vm.box = BOX_NAME
    config.vm.box_url = BOX_URI
  end
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
