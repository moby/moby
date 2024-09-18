package plugin

// Deprecated: use [Manager].
//
//nolint:revive // exported: type name will be used as plugin.PluginManager by other packages
type PluginManager = Manager

// Deprecated: use [NewManager].
//
//nolint:unused
var NewPluginManager = NewManager
