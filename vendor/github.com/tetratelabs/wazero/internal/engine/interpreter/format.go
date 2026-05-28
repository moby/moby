package interpreter

import (
	"bytes"
)

func format(ops []unionOperation) string {
	buf := bytes.NewBuffer(nil)

	_, _ = buf.WriteString(".entrypoint\n")
	for i := range ops {
		op := &ops[i]
		str := op.String()
		isLabel := op.Kind == operationKindLabel
		if !isLabel {
			const indent = "\t"
			str = indent + str
		}
		_, _ = buf.WriteString(str + "\n")
	}
	return buf.String()
}
