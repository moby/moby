//+build !go1.6

package libvirt

/*
 * Golang 1.6 doesn't support C pointers to go memory.
 * A hacky-solution might be some multi-threaded approach to support domain events, but let's make it work
 * without domain events for now.
 */

/*
#cgo LDFLAGS: -lvirt 
#include <libvirt/libvirt.h>
#include <libvirt/virterror.h>
#include <stdlib.h>

int domainEventLifecycleCallback_cgo(virConnectPtr c, virDomainPtr d,
                                     int event, int detail, void *data)
{
    return domainEventLifecycleCallback(c, d, event, detail, data);
}

int domainEventGenericCallback_cgo(virConnectPtr c, virDomainPtr d, void *data)
{
    return domainEventGenericCallback(c, d, data);
}

int domainEventRTCChangeCallback_cgo(virConnectPtr c, virDomainPtr d,
                                     long long utcoffset, void *data)
{
    return domainEventRTCChangeCallback(c, d, utcoffset, data);
}

int domainEventWatchdogCallback_cgo(virConnectPtr c, virDomainPtr d,
                                    int action, void *data)
{
    return domainEventWatchdogCallback(c, d, action, data);
}

int domainEventIOErrorCallback_cgo(virConnectPtr c, virDomainPtr d,
                                   const char *srcPath, const char *devAlias,
                                   int action, void *data)
{
    return domainEventIOErrorCallback(c, d, srcPath, devAlias, action, data);
}

int domainEventGraphicsCallback_cgo(virConnectPtr c, virDomainPtr d,
                                    int phase, const virDomainEventGraphicsAddress *local,
                                    const virDomainEventGraphicsAddress *remote,
                                    const char *authScheme,
                                    const virDomainEventGraphicsSubject *subject, void *data)
{
    return domainEventGraphicsCallback(c, d, phase, local, remote, authScheme, subject, data);
}

int domainEventIOErrorReasonCallback_cgo(virConnectPtr c, virDomainPtr d,
                                         const char *srcPath, const char *devAlias,
                                         int action, const char *reason, void *data)
{
    return domainEventIOErrorReasonCallback(c, d, srcPath, devAlias, action, reason, data);
}

int domainEventBlockJobCallback_cgo(virConnectPtr c, virDomainPtr d,
                                    const char *disk, int type, int status, void *data)
{
    return domainEventIOErrorReasonCallback(c, d, disk, type, status, data);
}

int domainEventDiskChangeCallback_cgo(virConnectPtr c, virDomainPtr d,
                                      const char *oldSrcPath, const char *newSrcPath,
                                      const char *devAlias, int reason, void *data)
{
    return domainEventDiskChangeCallback(c, d, oldSrcPath, newSrcPath, devAlias, reason, data);
}

int domainEventTrayChangeCallback_cgo(virConnectPtr c, virDomainPtr d,
                                      const char *devAlias, int reason, void *data)
{
    return domainEventTrayChangeCallback(c, d, devAlias, reason, data);
}

int domainEventReasonCallback_cgo(virConnectPtr c, virDomainPtr d,
                                  int reason, void *data)
{
    return domainEventReasonCallback(c, d, reason, data);
}

int domainEventBalloonChangeCallback_cgo(virConnectPtr c, virDomainPtr d,
                                         unsigned long long actual, void *data)
{
    return domainEventBalloonChangeCallback(c, d, actual, data);
}

int domainEventDeviceRemovedCallback_cgo(virConnectPtr c, virDomainPtr d,
                                         const char *devAlias, void *data)
{
    return domainEventDeviceRemovedCallback(c, d, devAlias, data);
}
*/
import "C"
