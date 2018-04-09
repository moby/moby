package logger

var externalValidators []LogOptValidator

// RegisterExternalValidator adds the validator to the list of external validators.
// External validators are used by packages outside this package that need to add their own validation logic.
// This should only be called on package initialization.
func RegisterExternalValidator(v LogOptValidator) {
	externalValidators = append(externalValidators, v)
}

// AddBuiltinLogOpts updates the list of built-in log opts. This allows other packages to supplement additional log options
// without having to register an actual log driver. This is used by things that are more proxy log drivers and should
// not be exposed as a usable log driver to the API.
// This should only be called on package initialization.
func AddBuiltinLogOpts(opts map[string]bool) {
	for k, v := range opts {
		builtInLogOpts[k] = v
	}
}

func validateExternal(cfg map[string]string) error {
	for _, v := range externalValidators {
		if err := v(cfg); err != nil {
			return err
		}
	}
	return nil
}
