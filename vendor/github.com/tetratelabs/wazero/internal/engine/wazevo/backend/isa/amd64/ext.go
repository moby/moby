package amd64

// extMode represents the mode of extension in movzx/movsx.
type extMode byte

const (
	// extModeBL represents Byte -> Longword.
	extModeBL extMode = iota
	// extModeBQ represents Byte -> Quadword.
	extModeBQ
	// extModeWL represents Word -> Longword.
	extModeWL
	// extModeWQ represents Word -> Quadword.
	extModeWQ
	// extModeLQ represents Longword -> Quadword.
	extModeLQ
)

// String implements fmt.Stringer.
func (e extMode) String() string {
	switch e {
	case extModeBL:
		return "bl"
	case extModeBQ:
		return "bq"
	case extModeWL:
		return "wl"
	case extModeWQ:
		return "wq"
	case extModeLQ:
		return "lq"
	default:
		panic("BUG: invalid ext mode")
	}
}
