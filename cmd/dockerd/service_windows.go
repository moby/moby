package main

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/mgr"
)

var (
	flServiceName       *string
	flRegisterService   *bool
	flUnregisterService *bool
	flRunService        *bool

	oldStderr windows.Handle
	panicFile *os.File

	service *handler
)

func installServiceFlags(flags *pflag.FlagSet) {
	flServiceName = flags.String("service-name", "docker", "Set the Windows service name")
	flRegisterService = flags.Bool("register-service", false, "Register the service and exit")
	flUnregisterService = flags.Bool("unregister-service", false, "Unregister the service and exit")
	flRunService = flags.Bool("run-service", false, "")
	_ = flags.MarkHidden("run-service")
}

type handler struct {
	tosvc     chan bool
	fromsvc   chan error
	daemonCli *DaemonCli
}

func getServicePath() (string, error) {
	p, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "", err
	}
	return filepath.Abs(p)
}

func registerService() error {
	p, err := getServicePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	c := mgr.Config{
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
		Dependencies: []string{},
		DisplayName:  "Docker Engine",
	}

	// Configure the service to launch with the arguments that were just passed.
	args := []string{"--run-service"}
	for _, a := range os.Args[1:] {
		if a != "--register-service" && a != "--unregister-service" {
			args = append(args, a)
		}
	}

	s, err := m.CreateService(*flServiceName, p, c, args...)
	if err != nil {
		return err
	}
	defer s.Close()

	return s.SetRecoveryActions(
		[]mgr.RecoveryAction{
			{Type: mgr.ServiceRestart, Delay: 15 * time.Second},
			{Type: mgr.ServiceRestart, Delay: 15 * time.Second},
			{Type: mgr.NoAction},
		},
		uint32(24*time.Hour/time.Second),
	)
}

func unregisterService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(*flServiceName)
	if err != nil {
		return err
	}
	defer s.Close()

	return s.Delete()
}

// initService is the entry point for running the daemon as a Windows
// service. It returns an indication to stop (if registering/un-registering);
// an indication of whether it is running as a service; and an error.
func initService(daemonCli *DaemonCli) (bool, bool, error) {
	if *flUnregisterService {
		if *flRegisterService {
			return true, false, errors.New("--register-service and --unregister-service cannot be used together")
		}
		return true, false, unregisterService()
	}

	if *flRegisterService {
		return true, false, registerService()
	}

	if !*flRunService {
		return false, false, nil
	}

	// Check if we're running as a Windows service or interactively.
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false, false, err
	}

	h := &handler{
		tosvc:     make(chan bool),
		fromsvc:   make(chan error),
		daemonCli: daemonCli,
	}

	logrus.SetOutput(io.Discard) // TODO(thaJeztah): should this be moved to initLogging() ?

	service = h
	go func() {
		if isService {
			err = svc.Run(*flServiceName, h)
		} else {
			err = debug.Run(*flServiceName, h)
		}

		h.fromsvc <- err
	}()

	// Wait for the first signal from the service handler.
	err = <-h.fromsvc
	if err != nil {
		return false, false, err
	}
	return false, true, nil
}

func (h *handler) started() error {
	// This must be delayed until daemonCli initializes Config.Root
	err := initPanicFile(filepath.Join(h.daemonCli.Config.Root, "panic.log"))
	if err != nil {
		return err
	}

	h.tosvc <- false
	return nil
}

func (h *handler) stopped(err error) {
	logrus.Debugf("Stopping service: %v", err)
	h.tosvc <- err != nil
	<-h.fromsvc
}

func (h *handler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	s <- svc.Status{State: svc.StartPending, Accepts: 0}
	// Unblock initService()
	h.fromsvc <- nil

	// Wait for initialization to complete.
	failed := <-h.tosvc
	if failed {
		logrus.Debug("Aborting service start due to failure during initialization")
		return true, 1
	}

	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown | svc.Accepted(windows.SERVICE_ACCEPT_PARAMCHANGE)}
	logrus.Debug("Service running")
Loop:
	for {
		select {
		case failed = <-h.tosvc:
			break Loop
		case c := <-r:
			switch c.Cmd {
			case svc.Cmd(windows.SERVICE_CONTROL_PARAMCHANGE):
				h.daemonCli.reloadConfig()
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending, Accepts: 0}
				h.daemonCli.stop()
			}
		}
	}

	removePanicFile()
	if failed {
		return true, 1
	}
	return false, 0
}

func initPanicFile(path string) error {
	var err error
	panicFile, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o200)
	if err != nil {
		return err
	}

	st, err := panicFile.Stat()
	if err != nil {
		return err
	}

	// If there are contents in the file already, move the file out of the way
	// and replace it.
	if st.Size() > 0 {
		panicFile.Close()
		os.Rename(path, path+".old")
		panicFile, err = os.Create(path)
		if err != nil {
			return err
		}
	}

	// Update STD_ERROR_HANDLE to point to the panic file so that Go writes to
	// it when it panics. Remember the old stderr to restore it before removing
	// the panic file.
	h, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE)
	if err != nil {
		return err
	}
	oldStderr = h

	err = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(panicFile.Fd()))
	if err != nil {
		return err
	}

	// Reset os.Stderr to the panic file (so fmt.Fprintf(os.Stderr,...) actually gets redirected)
	os.Stderr = os.NewFile(panicFile.Fd(), "/dev/stderr")

	// Force threads that panic to write to stderr (the panicFile handle now), otherwise it will go into the ether
	log.SetOutput(os.Stderr)

	return nil
}

func removePanicFile() {
	if st, err := panicFile.Stat(); err == nil {
		if st.Size() == 0 {
			windows.SetStdHandle(windows.STD_ERROR_HANDLE, oldStderr)
			panicFile.Close()
			os.Remove(panicFile.Name())
		}
	}
}
