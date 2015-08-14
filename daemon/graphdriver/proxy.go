// +build linux

package graphdriver

import (
	"bufio"
	"fmt"
	"github.com/docker/docker/cli"
	"io"
	"os"
	"path"
	"strings"
	"syscall"
)

type ProxyAPI struct {
	driver Driver
	Root   string
	CtName string
	Cli    *cli.Cli
	mounts map[string]string
}

////////////// Init ////////////////

type InitArgs struct {
	DriverName string
	Home       string
	Options    []string
}

type InitReply struct{}

func (p *ProxyAPI) Init(args *InitArgs, reply *InitReply) error {
	var err error
	p.mounts = make(map[string]string)

	if err := os.MkdirAll(p.Root+args.Home, 0755); err != nil {
		return err
	}

	err = p.Cli.Run("exec", "-s", p.CtName, "mkdir", args.Home)
	if err != nil {
		return err
	}
	err = p.Cli.Run("exec", "-s", p.CtName, "mkdir", args.Home+"/"+args.DriverName)
	if err != nil {
		return err
	}
	err = p.Cli.Run("exec", "-s", p.CtName, "mkdir", args.Home+"/"+args.DriverName+"/mnt")
	if err != nil {
		return err
	}

	p.driver, err = GetDriver(args.DriverName, p.Root+args.Home, args.Options)
	*reply = InitReply{}
	return err
}

////////////// Status ////////////////

type StatusArgs struct{}

type StatusReply struct {
	Status [][2]string
}

func (p *ProxyAPI) Status(args *StatusArgs, reply *StatusReply) error {
	if p.driver == nil {
		return fmt.Errorf("driver not initialized")
	}

	*reply = StatusReply{p.driver.Status()}
	return nil
}

////////////// Create ////////////////

type CreateArgs struct {
	Id, Parent string
}

type CreateReply struct{}

func (p *ProxyAPI) Create(args *CreateArgs, reply *CreateReply) error {
	if p.driver == nil {
		return fmt.Errorf("driver not initialized")
	}

	*reply = CreateReply{}
	return p.driver.Create(args.Id, args.Parent)
}

////////////// Remove ////////////////

type RemoveArgs struct {
	Id string
}

type RemoveReply struct{}

func (p *ProxyAPI) Remove(args *RemoveArgs, reply *RemoveReply) error {
	if p.driver == nil {
		return fmt.Errorf("driver not initialized")
	}

	*reply = RemoveReply{}
	return p.driver.Remove(args.Id)
}

////////////// Get ////////////////

type GetArgs struct {
	Id, MountLabel string
}

type GetReply struct {
	Dir string
}

func (p *ProxyAPI) Get(args *GetArgs, reply *GetReply) error {
	if p.driver == nil {
		return fmt.Errorf("driver not initialized")
	}

	pth, err := p.driver.Get(args.Id, args.MountLabel)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(pth, p.Root) {
		p.driver.Put(args.Id)
		return fmt.Errorf("Get(%s) returned path=%s without prefix=%s", args.Id, pth, p.Root)
	}
	client_path := strings.TrimPrefix(pth, p.Root)

	var client_mp string
	dev, mp, err := get_mount_point(pth, p.Root)
	if err != nil {
		goto Error
	}
	client_mp = strings.TrimPrefix(mp, p.Root)

	err = p.Cli.Run("exec", "-s", p.CtName, "mkdir", client_mp)
	if err != nil {
		goto Error
	}
	err = p.Cli.Run("exec", "-s", p.CtName, "mount", "-t", "ext4", dev, client_mp)
	if err != nil {
		goto Error
	}
	p.mounts[args.Id] = client_mp

	*reply = GetReply{client_path}
	return nil

Error:
	p.driver.Put(args.Id)
	return err
}

////////////// Put ////////////////

type PutArgs struct {
	Id string
}

type PutReply struct{}

func (p *ProxyAPI) Put(args *PutArgs, reply *PutReply) error {
	if p.driver == nil {
		return fmt.Errorf("driver not initialized")
	}

	if _, ok := p.mounts[args.Id]; !ok {
		return fmt.Errorf("ProxyAPI.Put mounts[%s] not exists", args.Id)
	}

	err := p.Cli.Run("exec", "-s", p.CtName, "umount", p.mounts[args.Id])
	if err != nil {
		fmt.Println("ProxyAPI.Put dock-umount error:", err)
		return err
	}
	delete(p.mounts, args.Id)

	p.driver.Put(args.Id)
	*reply = PutReply{}
	return nil
}

////////////// Exists ////////////////

type ExistsArgs struct {
	Id string
}

type ExistsReply struct {
	Exists bool
}

func (p *ProxyAPI) Exists(args *ExistsArgs, reply *ExistsReply) error {
	if p.driver == nil {
		return fmt.Errorf("driver not initialized")
	}

	*reply = ExistsReply{p.driver.Exists(args.Id)}
	return nil
}

////////////// Cleanup ////////////////

type CleanupArgs struct{}

type CleanupReply struct{}

func (p *ProxyAPI) Cleanup(args *CleanupArgs, reply *CleanupReply) error {
	if p.driver == nil {
		return fmt.Errorf("driver not initialized")
	}

	*reply = CleanupReply{}
	return p.driver.Cleanup()
}

////////////// GetMetadata ////////////////

type GetMetadataArgs struct {
	Id string
}

type GetMetadataReply struct {
	MInfo map[string]string
}

func (p *ProxyAPI) GetMetadata(args *GetMetadataArgs, reply *GetMetadataReply) error {
	if p.driver == nil {
		return fmt.Errorf("driver not initialized")
	}

	minfo, err := p.driver.GetMetadata(args.Id)
	if err != nil {
		return err
	}

	*reply = GetMetadataReply{minfo}
	return nil
}

////////////// a helper for Get ////////////////

func get_mount_point(pth, root string) (dev, mp string, err error) {
	dir := path.Dir(pth)

	for {
		s1 := syscall.Stat_t{}
		err = syscall.Stat(pth, &s1)
		if err != nil {
			return
		}

		s2 := syscall.Stat_t{}
		err = syscall.Stat(dir, &s2)
		if err != nil {
			return
		}

		if s1.Dev != s2.Dev {
			mp = pth
			break
		}

		if dir == root || !strings.HasPrefix(dir, root) {
			err = fmt.Errorf("Cannot find mount point: path=%s root=%s dir=%s", pth, root, dir)
			return
		}

		pth = dir
		dir = path.Dir(pth)
	}

	mtab, err := os.Open("/etc/mtab")
	if err != nil {
		return
	}
	defer mtab.Close()

	r := bufio.NewReader(mtab)
	for {
		var line []byte
		line, _, err = r.ReadLine()
		if err != nil {
			if err == io.EOF {
				err = fmt.Errorf("Cannot find device for path=%s", mp)
			}
			return
		}
		fields := strings.Fields(string(line))
		if len(fields) > 1 && fields[1] == mp {
			dev = fields[0]
			return
		}
	}
}
