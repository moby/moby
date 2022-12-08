// +build !linux

package selinux

func setDisabled() {
}

func getEnabled() bool {
	return false
}

func classIndex(class string) (int, error) {
	return -1, nil
}

func setFileLabel(fpath string, label string) error {
	return nil
}

func lSetFileLabel(fpath string, label string) error {
	return nil
}

func fileLabel(fpath string) (string, error) {
	return "", nil
}

func lFileLabel(fpath string) (string, error) {
	return "", nil
}

func setFSCreateLabel(label string) error {
	return nil
}

func fsCreateLabel() (string, error) {
	return "", nil
}

func currentLabel() (string, error) {
	return "", nil
}

func pidLabel(pid int) (string, error) {
	return "", nil
}

func execLabel() (string, error) {
	return "", nil
}

func canonicalizeContext(val string) (string, error) {
	return "", nil
}

func computeCreateContext(source string, target string, class string) (string, error) {
	return "", nil
}

func calculateGlbLub(sourceRange, targetRange string) (string, error) {
	return "", nil
}

func setExecLabel(label string) error {
	return nil
}

func setTaskLabel(label string) error {
	return nil
}

func setSocketLabel(label string) error {
	return nil
}

func socketLabel() (string, error) {
	return "", nil
}

func peerLabel(fd uintptr) (string, error) {
	return "", nil
}

func setKeyLabel(label string) error {
	return nil
}

func keyLabel() (string, error) {
	return "", nil
}

func (c Context) get() string {
	return ""
}

func newContext(label string) (Context, error) {
	c := make(Context)
	return c, nil
}

func clearLabels() {
}

func reserveLabel(label string) {
}

func enforceMode() int {
	return Disabled
}

func setEnforceMode(mode int) error {
	return nil
}

func defaultEnforceMode() int {
	return Disabled
}

func releaseLabel(label string) {
}

func roFileLabel() string {
	return ""
}

func kvmContainerLabels() (string, string) {
	return "", ""
}

func initContainerLabels() (string, string) {
	return "", ""
}

func containerLabels() (processLabel string, fileLabel string) {
	return "", ""
}

func securityCheckContext(val string) error {
	return nil
}

func copyLevel(src, dest string) (string, error) {
	return "", nil
}

func chcon(fpath string, label string, recurse bool) error {
	return nil
}

func dupSecOpt(src string) ([]string, error) {
	return nil, nil
}

func disableSecOpt() []string {
	return []string{"disable"}
}

func getDefaultContextWithLevel(user, level, scon string) (string, error) {
	return "", nil
}

func label(_ string) string {
	return ""
}
