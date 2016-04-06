// +build pkcs11

// an interface around the pkcs11 library, so that things can be mocked out
// for testing

package yubikey

import "github.com/miekg/pkcs11"

// IPKCS11 is an interface for wrapping github.com/miekg/pkcs11
type pkcs11LibLoader func(module string) IPKCS11Ctx

func defaultLoader(module string) IPKCS11Ctx {
	return pkcs11.New(module)
}

// IPKCS11Ctx is an interface for wrapping the parts of
// github.com/miekg/pkcs11.Ctx that yubikeystore requires
type IPKCS11Ctx interface {
	Destroy()
	Initialize() error
	Finalize() error
	GetSlotList(tokenPresent bool) ([]uint, error)
	OpenSession(slotID uint, flags uint) (pkcs11.SessionHandle, error)
	CloseSession(sh pkcs11.SessionHandle) error
	Login(sh pkcs11.SessionHandle, userType uint, pin string) error
	Logout(sh pkcs11.SessionHandle) error
	CreateObject(sh pkcs11.SessionHandle, temp []*pkcs11.Attribute) (
		pkcs11.ObjectHandle, error)
	DestroyObject(sh pkcs11.SessionHandle, oh pkcs11.ObjectHandle) error
	GetAttributeValue(sh pkcs11.SessionHandle, o pkcs11.ObjectHandle,
		a []*pkcs11.Attribute) ([]*pkcs11.Attribute, error)
	FindObjectsInit(sh pkcs11.SessionHandle, temp []*pkcs11.Attribute) error
	FindObjects(sh pkcs11.SessionHandle, max int) (
		[]pkcs11.ObjectHandle, bool, error)
	FindObjectsFinal(sh pkcs11.SessionHandle) error
	SignInit(sh pkcs11.SessionHandle, m []*pkcs11.Mechanism,
		o pkcs11.ObjectHandle) error
	Sign(sh pkcs11.SessionHandle, message []byte) ([]byte, error)
}
