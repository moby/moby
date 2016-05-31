package hcsshim

import (
	"github.com/Sirupsen/logrus"
	"syscall"
	"time"
)

type waitable interface {
	waitTimeoutInternal(timeout uint32) (bool, error)
	hcsWait(timeout uint32) (bool, error)
}

func waitTimeoutHelper(object waitable, timeout time.Duration) (bool, error) {
	var (
		millis uint32
	)

	for totalMillis := uint64(timeout / time.Millisecond); totalMillis > 0; totalMillis = totalMillis - uint64(millis) {
		if totalMillis >= syscall.INFINITE {
			millis = syscall.INFINITE - 1
		} else {
			millis = uint32(totalMillis)
		}

		result, err := object.waitTimeoutInternal(millis)

		if err != nil {
			return result, err
		}
	}
	return true, nil
}

func waitTimeoutInternalHelper(object waitable, timeout uint32) (bool, error) {
	return object.hcsWait(timeout)
}

func waitForSingleObject(handle syscall.Handle, timeout uint32) (bool, error) {
	s, e := syscall.WaitForSingleObject(handle, timeout)
	switch s {
	case syscall.WAIT_OBJECT_0:
		return true, nil
	case syscall.WAIT_TIMEOUT:
		return false, nil
	default:
		return false, e
	}
}

func processAsyncHcsResult(err error, resultp *uint16, callbackNumber uintptr, expectedNotification hcsNotification, timeout *time.Duration) error {
	err = processHcsResult(err, resultp)
	if err == ErrVmcomputeOperationPending {
		return waitForNotification(callbackNumber, expectedNotification, timeout)
	}

	return err
}

func waitForNotification(callbackNumber uintptr, expectedNotification hcsNotification, timeout *time.Duration) error {
	callbackMapLock.RLock()
	channels := callbackMap[callbackNumber].channels
	callbackMapLock.RUnlock()

	expectedChannel := channels[expectedNotification]
	if expectedChannel == nil {
		logrus.Errorf("unknown notification type in waitForNotification %x", expectedNotification)
	}

	if timeout != nil {
		timer := time.NewTimer(*timeout)
		defer timer.Stop()

		select {
		case err := <-expectedChannel:
			return err
		case err := <-channels[hcsNotificationSystemExited]:
			// If the expected notification is hcsNotificationSystemExited which of the two selects
			// chosen is random. Return the raw error if hcsNotificationSystemExited is expected
			if channels[hcsNotificationSystemExited] == expectedChannel {
				return err
			}
			return ErrUnexpectedContainerExit
		case err := <-channels[hcsNotificationServiceDisconnect]:
			// hcsNotificationServiceDisconnect should never be an expected notification
			// it does not need the same handling as hcsNotificationSystemExited
			logrus.Error(err)
			return ErrUnexpectedProcessAbort
		case <-timer.C:
			return ErrTimeout
		}
	}
	select {
	case err := <-expectedChannel:
		return err
	case err := <-channels[hcsNotificationSystemExited]:
		// If the expected notification is hcsNotificationSystemExited which of the two selects
		// chosen is random. Return the raw error if hcsNotificationSystemExited is expected
		if channels[hcsNotificationSystemExited] == expectedChannel {
			return err
		}
		return ErrUnexpectedContainerExit
	case err := <-channels[hcsNotificationServiceDisconnect]:
		// hcsNotificationServiceDisconnect should never be an expected notification
		// it does not need the same handling as hcsNotificationSystemExited
		logrus.Error(err)
		return ErrUnexpectedProcessAbort
	}
}
