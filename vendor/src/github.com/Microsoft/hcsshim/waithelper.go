package hcsshim

import (
	"syscall"
	"time"
)

type waitable interface {
	waitTimeoutInternal(timeout uint32) (bool, error)
	hcsWait(timeout uint32) (bool, error)
}

type callbackable interface {
	registerCallback(expectedNotification hcsNotification) (uintptr, error)
	unregisterCallback(callbackNumber uintptr) error
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

func processAsyncHcsResult(object callbackable, err error, resultp *uint16, expectedNotification hcsNotification, timeout *time.Duration) error {
	err = processHcsResult(err, resultp)
	if err == ErrVmcomputeOperationPending {
		if timeout != nil {
			err = registerAndWaitForCallbackTimeout(object, expectedNotification, *timeout)
		} else {
			err = registerAndWaitForCallback(object, expectedNotification)
		}
	}

	return err
}

func registerAndWaitForCallbackTimeout(object callbackable, expectedNotification hcsNotification, timeout time.Duration) error {
	callbackNumber, err := object.registerCallback(expectedNotification)
	if err != nil {
		return err
	}
	defer object.unregisterCallback(callbackNumber)

	return waitForNotificationTimeout(callbackNumber, timeout)
}

func registerAndWaitForCallback(object callbackable, expectedNotification hcsNotification) error {
	callbackNumber, err := object.registerCallback(expectedNotification)
	if err != nil {
		return err
	}
	defer object.unregisterCallback(callbackNumber)

	return waitForNotification(callbackNumber)
}

func waitForNotificationTimeout(callbackNumber uintptr, timeout time.Duration) error {
	callbackMapLock.RLock()
	channel := callbackMap[callbackNumber].channel
	callbackMapLock.RUnlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-channel:
		return err
	case <-timer.C:
		return ErrTimeout
	}
}

func waitForNotification(callbackNumber uintptr) error {
	callbackMapLock.RLock()
	channel := callbackMap[callbackNumber].channel
	callbackMapLock.RUnlock()

	select {
	case err := <-channel:
		return err
	}
}
