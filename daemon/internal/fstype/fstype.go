package fstype

// FsMagic unsigned id of the filesystem in use.
type FsMagic uint32

// FsMagicUnsupported is a predefined constant value other than a valid filesystem id.
const FsMagicUnsupported = FsMagic(0x00000000)

// GetFSMagic returns the filesystem id given the path. It returns an error
// when failing to detect the filesystem. it returns [FsMagicUnsupported]
// if detection is not supported by the platform, but no error is returned
// in this case.
func GetFSMagic(rootpath string) (FsMagic, error) {
	return getFSMagic(rootpath)
}
