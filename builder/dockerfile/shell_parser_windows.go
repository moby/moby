// +build windows

package dockerfile

func (sw *shellWord) lazyExpandedVariableReference(varName string) string {
	return "${env:" + varName + "}"
}
