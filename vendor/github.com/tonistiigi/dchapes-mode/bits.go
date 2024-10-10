package mode

import "os"

type modet uint16

// Although many of these can be found in the syscall package
// we don't use those to avoid the dependency, add some more
// values, use non-exported Go names, and use octal for better clarity.
//
// Note that Go only uses the the nine least significant bits as "Unix
// permission bits" (os.ModePerm == 0777). We use chmod(1)'s octal
// definitions that include three further bits: isUID, isGID, and
// isTXT (07000). Go has os.ModeSetuid=1<<23, os.ModeSetgid=1<<22,
// and os.ModeSticy=1<<20 for these. We do this so that absolute
// octal values can include those bits as defined by chmod(1).
const (
	//ifDir   = 040000 // directory
	isUID   = 04000 // set user id on execution
	isGID   = 02000 // set group id on execution
	isTXT   = 01000 // sticky bit
	iRWXU   = 00700 // RWX mask for owner
	iRUser  = 00400 // R for owner
	iWUser  = 00200 // W for owner
	iXUser  = 00100 // X for owner
	iRWXG   = 00070 // RWX mask for group
	iRGroup = 00040 // R for group
	iWGroup = 00020 // W for group
	iXGroup = 00010 // X for group
	iRWXO   = 00007 // RWX mask for other
	iROther = 00004 // R for other
	iWOther = 00002 // W for other
	iXOther = 00001 // X for other

	standardBits = isUID | isGID | iRWXU | iRWXG | iRWXO

	// os.FileMode bits we touch
	fmBits = os.ModeSetuid | os.ModeSetgid | os.ModeSticky | os.ModePerm
)

func fileModeToBits(fm os.FileMode) modet {
	m := modet(fm.Perm())
	/*
		if fm&os.ModeSetuid != 0 {
			m |= isUID
		}
		if fm&os.ModeSetgid != 0 {
			m |= isGID
		}
		if fm&os.ModeSticky != 0 {
			m |= isTXT
		}
	*/
	m |= modet(fm & (os.ModeSetuid | os.ModeSetgid) >> 12)
	m |= modet(fm & os.ModeSticky >> 11)
	return m
}

func bitsToFileMode(old os.FileMode, m modet) os.FileMode {
	fm := old &^ fmBits
	fm |= os.FileMode(m) & os.ModePerm
	/*
		if m&isUID != 0 {
			fm |= os.ModeSetuid
		}
		if m&isGID != 0 {
			fm |= os.ModeSetgid
		}
		if m&isTXT != 0 {
			fm |= os.ModeSticky
		}
	*/
	fm |= os.FileMode(m&(isUID|isGID)) << 12
	fm |= os.FileMode(m&isTXT) << 11
	return fm
}
