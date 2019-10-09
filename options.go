package kaito

type Options uint16

const (
	DisableGzip  = 1 << iota
	DisableBzip2 = 1 << iota
	DisableXz    = 1 << iota
	ForceNative  = 1 << iota
)

func (o Options) IsDisableGzip() bool {
	return o&DisableGzip != 0
}

func (o Options) IsDisableBzip2() bool {
	return o&DisableBzip2 != 0
}

func (o Options) IsDisableXz() bool {
	return o&DisableXz != 0
}

func (o Options) IsForceNative() bool {
	return o&ForceNative != 0
}
