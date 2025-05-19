package systemd

import "github.com/accuknox/spire/pkg/common/catalog"

const (
	pluginName = "systemd"
)

func BuiltIn() catalog.BuiltIn {
	return builtin(New())
}