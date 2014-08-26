// +build !linux

package audit

/*
  The audit package is a go bindings to libaudit that only allows for
  logging audit events.

  Author Steve Grubb <sgrubb@redhat.com>

*/

const (
	AUDIT_VIRT_CONTROL    = 2500
	AUDIT_VIRT_RESOURCE   = 2501
	AUDIT_VIRT_MACHINE_ID = 2502
)

func AuditValueNeedsEncoding(str string) bool {
	return false
}

func AuditEncodeNVString(name string, value string) string {
	return ""
}

func AuditLogUserEvent(event_type int, message string, result bool) error {
	return nil
}
