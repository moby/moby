GOVERSION = "1.17.8"

Vagrant.configure("2") do |config|
  config.vm.define "ubuntu" do |ubuntu|
    ubuntu.vm.box = "generic/ubuntu2004"
  end
  config.vm.define "freebsd" do |freebsd|
    freebsd.vm.box = "generic/freebsd13"
  end
  config.ssh.forward_agent = true
  config.vm.synced_folder ".", "/home/vagrant/go/src/github.com/mistifyio/go-zfs", create: true
  config.vm.provision "shell", inline: <<-EOF
    set -euxo pipefail

    os=$(uname -s|tr '[A-Z]' '[a-z]')
    case $os in
    linux) apt-get update -y && apt-get install -y --no-install-recommends gcc libc-dev zfsutils-linux ;;
    esac

    cd /tmp
    curl -fLO --retry-max-time 30 --retry 10 https://go.dev/dl/go#{GOVERSION}.$os-amd64.tar.gz
    tar -C /usr/local -zxf go#{GOVERSION}.$os-amd64.tar.gz
    ln -nsf /usr/local/go/bin/go /usr/local/bin/go
    rm -rf go*.tar.gz

    chown -R vagrant:vagrant /home/vagrant/go
    cd /home/vagrant/go/src/github.com/mistifyio/go-zfs
    go test -c
    sudo ./go-zfs.test -test.v
    CGO_ENABLED=0 go test -c
    sudo ./go-zfs.test -test.v
  EOF
end
