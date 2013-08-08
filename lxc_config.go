package docker

import (
  "fmt"
  "strconv"
  "strings"
)

func getMemorySwap(config *Config) int64 {
  // By default, MemorySwap is set to twice the size of RAM.
  // If you want to omit MemorySwap, set it to `-1'.
  if config.MemorySwap < 0 {
    return 0
  }
  return config.Memory * 2
}

func getLxcConfig (container *Container) string {
  lines := []string{}
  usedKeys := map[string]bool{}
  
  // Since there's a lot of code in this function, we use closures to
  // maximize terseness 
  iappend := func(s string) {
    lines = append(lines, s)
    return
  }
  spacer := func() {
    iappend("\n")
  }
  override := func (key string, val string) {
    var winningValue string
    usedKeys[key] = true
    userVal, userOverride := container.Config.LxcOptions[key]
    if userOverride {
      iappend(fmt.Sprintf("# %s overridden by -lxc-conf option", key))
      winningValue = userVal
    } else {
      winningValue = val
    }
    iappend(fmt.Sprintf("%s = %s", key, winningValue))
    return
  }

  // populate confMap
  
  // Hostname
  iappend("# hostname")
  if len(container.Config.Hostname) > 0 {
    override("lxc.utsname", container.Config.Hostname)
  } else {
    override("lxc.utsname", container.ID)
  }
  iappend("# lxc.aa_profile = unconfined")

  // Configure network
  spacer()
  if (container.Config.NetworkDisabled) {
    iappend("# network is disabled (-n=false)")
    override("lxc.network.type", "empty")
  } else {
    iappend("# network configuration")
    override("lxc.network.type", "veth")
    override("lxc.network.flags", "up")
    override("lxc.network.link", container.NetworkSettings.Bridge)
    override("lxc.network.name", "eth0")
    override("lxc.network.mtu", "1500")
    override("lxc.network.ipv4", fmt.Sprintf("%s/%s", container.NetworkSettings.IPAddress, strconv.Itoa(container.NetworkSettings.IPPrefixLen)))
  }

  // Configure root filesystem
  rootfsPath := container.RootfsPath()
  iappend(`
#root filesystem`)
  override("lxc.rootfs", rootfsPath)

  // Configure tty etc.
  iappend(`
# use a dedicated pts for the container (and limit the number of pseudo terminal
# available`)
  override("lxc.pts", "1024")
  iappend("# disable the main console")
  override("lxc.console", "none")
  iappend("# no controlling tty at all")
  override("lxc.tty", "1")

  // Configure device access. Note, we don't allow these to be overridden.
iappend(`
# no implicit access to devices
lxc.cgroup.devices.deny = a
# /dev/null and zero
lxc.cgroup.devices.allow = c 1:3 rwm
lxc.cgroup.devices.allow = c 1:5 rwm

# consoles
lxc.cgroup.devices.allow = c 5:1 rwm
lxc.cgroup.devices.allow = c 5:0 rwm
lxc.cgroup.devices.allow = c 4:0 rwm
lxc.cgroup.devices.allow = c 4:1 rwm

# /dev/urandom, /dev/random
lxc.cgroup.devices.allow = c 1:9 rwm
lxc.cgroup.devices.allow = c 1:8 rwm

# /dev/pts/* - pts namespaces are "coming soon"
lxc.cgroup.devices.allow = c 136:* rwm
lxc.cgroup.devices.allow = c 5:2 rwm

# tuntap
lxc.cgroup.devices.allow = c 10:200 rwm

# fuse
# lxc.cgroup.devices.allow = c 10:229 rwm
# rtc
# lxc.cgroup.devices.allow = c 254:0 rwm
`)

  // Mounts. We don't allow these to be overridden either.

  iappend(`
# standard mount point
#  WARNING: procfs is a known attack vector and should probably be disabled
#           if your userspace allows it. eg. see http://blog.zx2c4.com/749`)
  
  iappend(fmt.Sprintf("lxc.mount.entry = proc %s/proc proc nosuid,nodev,noexec 0 0", rootfsPath))
  
  iappend(`
#  WARNING: sysfs is a known attack vector and should probably be disabled
#           if your userspace allows it. eg. see http://bit.ly/T9CkqJ`)
  
  iappend(fmt.Sprintf("lxc.mount.entry = sysfs %s/sys sysfs nosuid,nodev,noexec 0 0", rootfsPath))
  iappend(fmt.Sprintf("lxc.mount.entry = devpts %s/dev/pts devpts newinstance,ptmxmode=0666,nosuid,noexec 0 0", rootfsPath))
  
  iappend(fmt.Sprintf("#lxc.mount.entry = varrun %s/var/run tmpfs mode=755,size=4096k,nosuid,nodev,noexec 0 0", rootfsPath))
  iappend(fmt.Sprintf("#lxc.mount.entry = varlock %s/var/lock tmpfs size=1024k,nosuid,nodev,noexec 0 0", rootfsPath))
  iappend(fmt.Sprintf("#lxc.mount.entry = shm %s/dev/shm tmpfs size=65536k,nosuid,nodev,noexec 0 0", rootfsPath))

  iappend("# Inject docker-init")
  iappend(fmt.Sprintf("lxc.mount.entry = %s %s/sbin/init none bind,ro 0 0", container.SysInitPath, rootfsPath)) 

  iappend("# In order to get a working DNS environment, mount bind (ro) the host's /etc/resolv.conf into the container")
  iappend(fmt.Sprintf("lxc.mount.entry = %s %s/etc/resolv.conf none bind,ro 0 0", container.ResolvConfPath, rootfsPath))

  // Mount user-requested volumes
  spacer()
  iappend("# Volumes requested with the -v option")
  if len(container.Volumes) > 0 {
    for virtualPath, realPath := range(container.Volumes) {
      var perms string
      if container.VolumesRW[virtualPath] {
        perms = "rw"
      } else {
        perms = "ro"
      }
      iappend(fmt.Sprintf("lxc.mount.entry = %s %s/%s none bind,%s 0 0", realPath, rootfsPath, virtualPath, perms))
    }
  }

  // Drop unsecure capabilities
  iappend(`
# drop linux capabilities (apply mainly to the user root in the container)
#  (Note: 'lxc.cap.keep' is coming soon and should replace this under the
#         security principle 'deny all unless explicitly permitted', see
#         http://sourceforge.net/mailarchive/message.php?msg_id=31054627 )`)
  override("lxc.cap.drop", "audit_control audit_write mac_admin mac_override mknod setfcap setpcap sys_admin sys_boot sys_module sys_nice sys_pacct sys_rawio sys_resource sys_time sys_tty_config")

  // Limit resources available to container
  iappend(`
# limits`)
  if container.Config.Memory > 0 {
    override("lxc.cgroup.memory.limit_in_bytes", strconv.FormatInt(container.Config.Memory, 10))
    override("lxc.cgroup.memory.soft_limit_in_bytes", strconv.FormatInt(container.Config.Memory, 10))
    override("lxc.cgroup.memory.memsw.limit_in_bytes", strconv.FormatInt(getMemorySwap(container.Config), 10))
  }
  if container.Config.CpuShares > 0 {
    override("lxc.cgroup.cpu.shares", strconv.FormatInt(container.Config.CpuShares, 10))
  }
  
  // Append all the user-defined options that haven't already been used.
  iappend(`

# User-defined config options`)
  for key, val := range(container.Config.LxcOptions) {
    if _, ok := usedKeys[key]; !ok {
      iappend(fmt.Sprintf("%s = %s", key, val))
    }
  }

  return strings.Join(lines, "\n")

}
