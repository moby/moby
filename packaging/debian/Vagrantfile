VM_IP = "192.168.33.31"
PKG_DEP = "git debhelper build-essential autotools-dev devscripts golang"

Vagrant::Config.run do |config|
  config.vm.box = 'debian-7.0.rc1.64'
  config.vm.box_url = 'http://puppet-vagrant-boxes.puppetlabs.com/debian-70rc1-x64-vbox4210-nocm.box'
  config.vm.share_folder 'v-data', '/data/docker', "#{File.dirname(__FILE__)}/../.."
  config.vm.network :hostonly,VM_IP

  # Add kernel cgroup memory limitation boot parameters
  grub_cmd="sed -i 's#DEFAULT=\"quiet\"#DEFAULT=\"cgroup_enable=memory swapaccount=1 quiet\"#' /etc/default/grub"
  config.vm.provision :shell, :inline => "#{grub_cmd};update-grub"

  # Install debian packaging dependencies and create debian packages
  pkg_cmd = "apt-get -qq update; DEBIAN_FRONTEND=noninteractive apt-get install -qq -y #{PKG_DEP}; " \
      "curl -s -o /go.tar.gz https://go.googlecode.com/files/go1.1.1.linux-amd64.tar.gz; " \
      "tar -C /usr/local -xzf /go.tar.gz; rm /usr/bin/go; " \
      "ln -s /usr/local/go/bin/go /usr/bin; "\
      "export GPG_KEY='#{ENV['GPG_KEY']}'; cd /data/docker/packaging/debian; make debian"
  config.vm.provision :shell, :inline => pkg_cmd
end
