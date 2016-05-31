package hcsshim

import (
	"errors"
	"sync"
	"syscall"
)

var (
	nextCallback    uintptr
	callbackMap     = map[uintptr]*notifcationWatcherContext{}
	callbackMapLock = sync.RWMutex{}

	notificationWatcherCallback = syscall.NewCallback(notificationWatcher)

	// Notifications for HCS_SYSTEM handles
	hcsNotificationSystemExited          hcsNotification = 0x00000001
	hcsNotificationSystemCreateCompleted hcsNotification = 0x00000002
	hcsNotificationSystemStartCompleted  hcsNotification = 0x00000003
	hcsNotificationSystemPauseCompleted  hcsNotification = 0x00000004
	hcsNotificationSystemResumeCompleted hcsNotification = 0x00000005

	// Notifications for HCS_PROCESS handles
	hcsNotificationProcessExited hcsNotification = 0x00010000

	// Common notifications
	hcsNotificationInvalid           hcsNotification = 0x00000000
	hcsNotificationServiceDisconnect hcsNotification = 0x01000000

	// ErrUnexpectedContainerExit is the error returned when a container exits while waiting for
	// a different expected notification
	ErrUnexpectedContainerExit = errors.New("unexpected container exit")

	// ErrUnexpectedProcessAbort is the error returned when communication with the compute service
	// is lost while waiting for a notification
	ErrUnexpectedProcessAbort = errors.New("lost communication with compute service")
)

type hcsNotification uint32
type notificationChannel chan error

type notifcationWatcherContext struct {
	channels notificationChannels
	handle   hcsCallback
}

type notificationChannels map[hcsNotification]notificationChannel

func newChannels() notificationChannels {
	channels := make(notificationChannels)

	channels[hcsNotificationSystemExited] = make(notificationChannel, 1)
	channels[hcsNotificationSystemCreateCompleted] = make(notificationChannel, 1)
	channels[hcsNotificationSystemStartCompleted] = make(notificationChannel, 1)
	channels[hcsNotificationSystemPauseCompleted] = make(notificationChannel, 1)
	channels[hcsNotificationSystemResumeCompleted] = make(notificationChannel, 1)
	channels[hcsNotificationProcessExited] = make(notificationChannel, 1)
	channels[hcsNotificationServiceDisconnect] = make(notificationChannel, 1)
	return channels
}

func notificationWatcher(notificationType hcsNotification, callbackNumber uintptr, notificationStatus uintptr, notificationData *uint16) uintptr {
	var result error
	if int32(notificationStatus) < 0 {
		result = syscall.Errno(win32FromHresult(notificationStatus))
	}

	callbackMapLock.RLock()
	channels := callbackMap[callbackNumber].channels
	callbackMapLock.RUnlock()

	channels[notificationType] <- result

	return 0
}
