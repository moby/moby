package internalapi

type WazeroOnly interface {
	wazeroOnly()
}

type WazeroOnlyType struct{}

func (WazeroOnlyType) wazeroOnly() {}
