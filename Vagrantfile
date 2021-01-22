# -*- mode: ruby -*-
# vi: set ft=ruby :

# Vagrant box for testing Moby with cgroup v2
Vagrant.configure("2") do |config|
  config.vm.box = "fedora/33-cloud-base"
  memory = 4096
  cpus = 2
  config.vm.provider :virtualbox do |v|
    v.memory = memory
    v.cpus = cpus
  end
  config.vm.provider :libvirt do |v|
    v.memory = memory
    v.cpus = cpus
  end
  config.vm.provision "install-packages", type: "shell", run: "once" do |sh|
    sh.inline = <<~SHELL
    set -eux -o pipefail
    dnf install -y git make
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
    usermod -aG docker vagrant
    SHELL
  end
end
