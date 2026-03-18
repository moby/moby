# -*- mode: ruby -*-
# vi: set ft=ruby :

#   Copyright The containerd Authors.
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

Vagrant.configure("2") do |config|
  config.vm.box = ENV["BOX"] ? ENV["BOX"].split("@")[0] : "almalinux/8"
  # BOX_VERSION is deprecated. Use "BOX=<BOX>@<BOX_VERSION>".
  config.vm.box_version = ENV["BOX_VERSION"] || (ENV["BOX"].split("@")[1] if ENV["BOX"])

  memory = 4096
  cpus = 2
  disk_size = 60
  config.vm.provider :virtualbox do |v, o|
    v.memory = memory
    v.cpus = cpus
    # Needs env var VAGRANT_EXPERIMENTAL="disks"
    o.vm.disk :disk, size: "#{disk_size}GB", primary: true
    v.customize ["modifyvm", :id, "--firmware", "efi"]
  end
  config.vm.provider :libvirt do |v|
    v.memory = memory
    v.cpus = cpus
    v.machine_virtual_size = disk_size
    # https://github.com/vagrant-libvirt/vagrant-libvirt/issues/1725#issuecomment-1454058646
    # Needs `sudo cp /usr/share/OVMF/OVMF_VARS_4M.fd /var/lib/libvirt/qemu/nvram/`
    v.loader = '/usr/share/OVMF/OVMF_CODE_4M.fd'
    v.nvram = '/var/lib/libvirt/qemu/nvram/OVMF_VARS_4M.fd'
  end

  config.vm.synced_folder ".", "/vagrant", type: "rsync"

  config.vm.provision 'shell', path: 'script/resize-vagrant-root.sh'

  # To re-run, installing CNI from RPM:
  #   INSTALL_PACKAGES="containernetworking-plugins" vagrant up --provision-with=install-packages
  #
  config.vm.provision "install-packages", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-install-packages"
    sh.env = {
        'INSTALL_PACKAGES': ENV['INSTALL_PACKAGES'],
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        set -eux -o pipefail
        dnf -y install \
            curl \
            gcc \
            git \
            make \
            ${INSTALL_PACKAGES}
    SHELL
  end

  # AlmaLinux does not have /usr/local/{bin,sbin} in the PATH by default
  config.vm.provision "setup-etc-environment", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-setup-etc-environment"
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        set -eux -o pipefail
        cat >> /etc/environment <<EOF
PATH=/usr/local/go/bin:/usr/local/bin:/usr/local/sbin:$PATH
EOF
        source /etc/environment
        SHELL
  end

  # To re-run this provisioner, installing a different version of go:
  #   GO_VERSION="1.14.6" vagrant up --provision-with=install-golang
  #
  config.vm.provision "install-golang", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-install-golang"
    sh.env = {
        'GO_VERSION': ENV['GO_VERSION'] || "1.24.3",
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        set -eux -o pipefail
        curl -fsSL "https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz" | tar Cxz /usr/local
        cat >> /etc/profile.d/sh.local <<EOF
GOPATH=\\$HOME/go
PATH=\\$GOPATH/bin:\\$PATH
export GOPATH PATH
git config --global --add safe.directory /vagrant
EOF
    source /etc/profile.d/sh.local
    SHELL
  end

  config.vm.provision "setup-gopath", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-setup-gopath"
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        mkdir -p ${GOPATH}/src/github.com/containerd
        ln -fnsv /vagrant ${GOPATH}/src/github.com/containerd/cgroups
    SHELL
  end

  config.vm.provision "test", type: "shell", run: "never" do |sh|
    sh.upload_path = "/tmp/test"
    sh.env = {
        'GOTEST': ENV['GOTEST'] || "go test",
        'GOTESTSUM_JUNITFILE': ENV['GOTESTSUM_JUNITFILE'],
        'GOTESTSUM_JSONFILE': ENV['GOTESTSUM_JSONFILE'],
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        cd ${GOPATH}/src/github.com/containerd/cgroups
        go env -w CGO_ENABLED=1
        go test -exec "sudo" -v -race -coverprofile=coverage.txt -covermode=atomic ./...
    SHELL
  end

  config.vm.provision "build-cgctl", type: "shell", run: "never" do |sh|
    sh.upload_path = "/tmp/build-cgctl"
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        cd ${GOPATH}/src/github.com/containerd/cgroups
        make all
    SHELL
  end
end
