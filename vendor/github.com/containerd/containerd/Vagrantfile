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

# Vagrantfile for Fedora and EL
Vagrant.configure("2") do |config|
  config.vm.box = ENV["BOX"] ? ENV["BOX"].split("@")[0] : "fedora/39-cloud-base"
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
    v.loader = "/usr/share/OVMF/OVMF_CODE.fd"
  end

  config.vm.synced_folder ".", "/vagrant", type: "rsync"

  config.vm.provision 'shell', path: 'script/resize-vagrant-root.sh'

  # Disabled by default. To run:
  #   vagrant up --provision-with=upgrade-packages
  # To upgrade only specific packages:
  #   UPGRADE_PACKAGES=selinux vagrant up --provision-with=upgrade-packages
  #
  config.vm.provision "upgrade-packages", type: "shell", run: "never" do |sh|
    sh.upload_path = "/tmp/vagrant-upgrade-packages"
    sh.env = {
        'UPGRADE_PACKAGES': ENV['UPGRADE_PACKAGES'],
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        set -eux -o pipefail
        dnf -y upgrade ${UPGRADE_PACKAGES}
    SHELL
  end

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
            container-selinux \
            curl \
            gcc \
            git \
            iptables \
            libseccomp-devel \
            libselinux-devel \
            lsof \
            make \
            strace \
            ${INSTALL_PACKAGES}
    SHELL
  end

  # EL does not have /usr/local/{bin,sbin} in the PATH by default
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
        'GO_VERSION': ENV['GO_VERSION'] || "1.22.8",
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
        ln -fnsv /vagrant ${GOPATH}/src/github.com/containerd/containerd
    SHELL
  end

  config.vm.provision "install-runc", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-install-runc"
    sh.env = {
        'RUNC_FLAVOR': ENV['RUNC_FLAVOR'] || "runc",
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        ${GOPATH}/src/github.com/containerd/containerd/script/setup/install-runc
        type runc
        runc --version
        chcon -v -t container_runtime_exec_t $(type -ap runc)
    SHELL
  end

  config.vm.provision "install-cni", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-install-cni"
    sh.env = {
        'CNI_BINARIES': 'bridge dhcp flannel host-device host-local ipvlan loopback macvlan portmap ptp tuning vlan',
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        cd ${GOPATH}/src/github.com/containerd/containerd
        script/setup/install-cni
        PATH=/opt/cni/bin:$PATH type ${CNI_BINARIES} || true
    SHELL
  end

  config.vm.provision "install-cri-tools", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-install-cri-tools"
    sh.env = {
        'CRI_TOOLS_VERSION': ENV['CRI_TOOLS_VERSION'] || '16911795a3c33833fa0ec83dac1ade3172f6989e',
        'GOBIN': '/usr/local/bin',
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        ${GOPATH}/src/github.com/containerd/containerd/script/setup/install-critools
        type crictl critest
        critest --version
    SHELL
  end

  config.vm.provision "install-containerd", type: "shell", run: "once" do |sh|
    sh.upload_path = "/tmp/vagrant-install-containerd"
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        cd ${GOPATH}/src/github.com/containerd/containerd
        make BUILDTAGS="seccomp selinux no_aufs no_btrfs no_devmapper no_zfs" binaries install
        type containerd
        containerd --version
        chcon -v -t container_runtime_exec_t /usr/local/bin/{containerd,containerd-shim*}
        ./script/setup/config-containerd
    SHELL
  end

  config.vm.provision "install-gotestsum", type: "shell",  run: "once" do |sh|
      sh.upload_path = "/tmp/vagrant-install-gotestsum"
      sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        ${GOPATH}/src/github.com/containerd/containerd/script/setup/install-gotestsum
        sudo cp ${GOPATH}/bin/gotestsum /usr/local/bin/
      SHELL
  end

  config.vm.provision "install-failpoint-binaries", type: "shell",  run: "once" do |sh|
      sh.upload_path = "/tmp/vagrant-install-failpoint-binaries"
      sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        ${GOPATH}/src/github.com/containerd/containerd/script/setup/install-failpoint-binaries
        chcon -v -t container_runtime_exec_t $(type -ap containerd-shim-runc-fp-v1)
        containerd-shim-runc-fp-v1 -v
      SHELL
  end

  # SELinux is Enforcing by default.
  # To set SELinux as Disabled on a VM that has already been provisioned:
  #   SELINUX=Disabled vagrant up --provision-with=selinux
  # To set SELinux as Permissive on a VM that has already been provsioned
  #   SELINUX=Permissive vagrant up --provision-with=selinux
  config.vm.provision "selinux", type: "shell", run: "never" do |sh|
    sh.upload_path = "/tmp/vagrant-selinux"
    sh.env = {
        'SELINUX': ENV['SELINUX'] || "Enforcing"
    }
    sh.inline = <<~SHELL
        /vagrant/script/setup/config-selinux
        /vagrant/script/setup/config-containerd
    SHELL
  end

  # SELinux is Enforcing by default (via provisioning) in this VM. To re-run with SELinux disabled:
  #   SELINUX=Disabled vagrant up --provision-with=selinux,test-integration
  #
  config.vm.provision "test-integration", type: "shell", run: "never" do |sh|
    sh.upload_path = "/tmp/test-integration"
    sh.env = {
        'RUNC_FLAVOR': ENV['RUNC_FLAVOR'] || "runc",
        'GOTEST': ENV['GOTEST'] || "go test",
        'GOTESTSUM_JUNITFILE': ENV['GOTESTSUM_JUNITFILE'],
        'GOTESTSUM_JSONFILE': ENV['GOTESTSUM_JSONFILE'],
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        rm -rf /var/lib/containerd-test /run/containerd-test
        cd ${GOPATH}/src/github.com/containerd/containerd
        go test -v -count=1 -race ./metrics/cgroups
        make integration EXTRA_TESTFLAGS="-timeout 15m -no-criu -test.v" TEST_RUNTIME=io.containerd.runc.v2 RUNC_FLAVOR=$RUNC_FLAVOR
    SHELL
  end

  # SELinux is Enforcing by default (via provisioning) in this VM. To re-run with SELinux disabled:
  #   SELINUX=Disabled vagrant up --provision-with=selinux,test-cri-integration
  #
  config.vm.provision "test-cri-integration", type: "shell", run: "never" do |sh|
    sh.upload_path = "/tmp/test-cri-integration"
    sh.env = {
        'GOTEST': ENV['GOTEST'] || "go test",
        'GOTESTSUM_JUNITFILE': ENV['GOTESTSUM_JUNITFILE'],
        'GOTESTSUM_JSONFILE': ENV['GOTESTSUM_JSONFILE'],
        'GITHUB_WORKSPACE': '',
        'ENABLE_CRI_SANDBOXES': ENV['ENABLE_CRI_SANDBOXES'],
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        cleanup() {
          rm -rf /var/lib/containerd* /run/containerd* /tmp/containerd* /tmp/test* /tmp/failpoint* /tmp/nri*
        }
        cleanup
        cd ${GOPATH}/src/github.com/containerd/containerd
        # cri-integration.sh executes containerd from ./bin, not from $PATH .
        make BUILDTAGS="seccomp selinux no_aufs no_btrfs no_devmapper no_zfs" binaries bin/cri-integration.test
        chcon -v -t container_runtime_exec_t ./bin/{containerd,containerd-shim*}
        CONTAINERD_RUNTIME=io.containerd.runc.v2 ./script/test/cri-integration.sh
        cleanup
    SHELL
  end

  # SELinux is Enforcing by default (via provisioning) in this VM. To re-run with SELinux disabled:
  #   SELINUX=Disabled vagrant up --provision-with=selinux,test-cri
  #
  config.vm.provision "test-cri", type: "shell", run: "never" do |sh|
    sh.upload_path = "/tmp/test-cri"
    sh.env = {
        'GOTEST': ENV['GOTEST'] || "go test",
        'REPORT_DIR': ENV['REPORT_DIR'],
    }
    sh.inline = <<~SHELL
        #!/usr/bin/env bash
        source /etc/environment
        source /etc/profile.d/sh.local
        set -eux -o pipefail
        systemctl disable --now containerd || true
        rm -rf /var/lib/containerd /run/containerd
        function cleanup()
        {
            journalctl -u containerd > /tmp/containerd.log
            cat /tmp/containerd.log
            systemctl stop containerd
        }
        selinux=$(getenforce)
        if [[ $selinux == Enforcing ]]; then
            setenforce 0
        fi
        systemctl enable --now ${GOPATH}/src/github.com/containerd/containerd/containerd.service
        if [[ $selinux == Enforcing ]]; then
            setenforce 1
        fi
        trap cleanup EXIT
        ctr version
        critest --parallel=$[$(nproc)+2] --ginkgo.skip='HostIpc is true' --report-dir="${REPORT_DIR}"
    SHELL
  end

end
