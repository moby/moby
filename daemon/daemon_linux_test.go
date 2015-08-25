// +build linux

package daemon

import (
	"strings"
	"testing"
)

func TestCleanupMounts(t *testing.T) {
	fixture := `230 138 0:60 / / rw,relatime - overlay overlay rw,lowerdir=/var/lib/docker/overlay/0ef9f93d5d365c1385b09d54bbee6afff3d92002c16f22eccb6e1549b2ff97d8/root,upperdir=/var/lib/docker/overlay/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb/upper,workdir=/var/lib/docker/overlay/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb/work
231 230 0:56 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
232 230 0:57 / /dev rw,nosuid - tmpfs tmpfs rw,mode=755
233 232 0:58 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
234 232 0:59 / /dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
235 232 0:55 / /dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
236 230 0:61 / /sys rw,nosuid,nodev,noexec,relatime - sysfs sysfs rw
237 236 0:62 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - tmpfs tmpfs rw
238 237 0:21 /system.slice/docker.service /sys/fs/cgroup/systemd rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,xattr,release_agent=/lib/systemd/systemd-cgroups-agent,name=systemd
239 237 0:23 /docker/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb /sys/fs/cgroup/perf_event rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,perf_event
240 237 0:24 /docker/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb /sys/fs/cgroup/cpuset rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,cpuset,clone_children
241 237 0:25 /docker/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb /sys/fs/cgroup/devices rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,devices
242 237 0:26 /docker/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb /sys/fs/cgroup/freezer rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,freezer
243 237 0:27 /docker/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb /sys/fs/cgroup/cpu,cpuacct rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,cpu,cpuacct
244 237 0:28 /docker/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb /sys/fs/cgroup/blkio rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,blkio
245 237 0:29 /docker/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb /sys/fs/cgroup/net_cls,net_prio rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,net_cls,net_prio
246 237 0:30 /docker/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb /sys/fs/cgroup/hugetlb rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,hugetlb
247 237 0:31 /docker/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime - cgroup cgroup rw,memory
248 230 253:1 /var/lib/docker/volumes/510cc41ac68c48bd4eac932e3e09711673876287abf1b185312cfbfe6261a111/_data /var/lib/docker rw,relatime - ext4 /dev/disk/by-uuid/ba70ea0c-1a8f-4ee4-9687-cb393730e2b5 rw,errors=remount-ro,data=ordered
250 230 253:1 /var/lib/docker/containers/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb/hostname /etc/hostname rw,relatime - ext4 /dev/disk/by-uuid/ba70ea0c-1a8f-4ee4-9687-cb393730e2b5 rw,errors=remount-ro,data=ordered
251 230 253:1 /var/lib/docker/containers/dfac036ce135a8914e292cb2f6fea114f7339983c186366aa26d0051e93162cb/hosts /etc/hosts rw,relatime - ext4 /dev/disk/by-uuid/ba70ea0c-1a8f-4ee4-9687-cb393730e2b5 rw,errors=remount-ro,data=ordered
252 232 0:13 /1 /dev/console rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=000
139 236 0:11 / /sys/kernel/security rw,relatime - securityfs none rw
140 230 0:54 / /tmp rw,relatime - tmpfs none rw
145 230 0:3 / /run/docker/netns/default rw - nsfs nsfs rw
130 140 0:45 / /tmp/docker_recursive_mount_test312125472/tmpfs rw,relatime - tmpfs tmpfs rw
131 230 0:3 / /run/docker/netns/47903e2e6701 rw - nsfs nsfs rw
133 230 0:55 / /go/src/github.com/docker/docker/bundles/1.9.0-dev/test-integration-cli/d45526097/graph/containers/47903e2e67014246eba27607809d5f5c2437c3bf84c2986393448f84093cc40b/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw`

	d := &Daemon{
		repository: "/go/src/github.com/docker/docker/bundles/1.9.0-dev/test-integration-cli/d45526097/graph/containers/",
	}

	expected := "/go/src/github.com/docker/docker/bundles/1.9.0-dev/test-integration-cli/d45526097/graph/containers/47903e2e67014246eba27607809d5f5c2437c3bf84c2986393448f84093cc40b/mqueue"
	var unmounted bool
	unmount := func(target string) error {
		if target == expected {
			unmounted = true
		}
		return nil
	}

	d.cleanupMountsFromReader(strings.NewReader(fixture), unmount)

	if !unmounted {
		t.Fatalf("Expected to unmount the mqueue")
	}
}

func TestNotCleanupMounts(t *testing.T) {
	d := &Daemon{
		repository: "",
	}
	var unmounted bool
	unmount := func(target string) error {
		unmounted = true
		return nil
	}
	mountInfo := `234 232 0:59 / /dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k`
	d.cleanupMountsFromReader(strings.NewReader(mountInfo), unmount)
	if unmounted {
		t.Fatalf("Expected not to clean up /dev/shm")
	}
}
