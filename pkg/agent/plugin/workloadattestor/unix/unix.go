package unix

import "github.com/accuknox/spire/pkg/common/catalog"

const (
	pluginName = "unix"
)

func BuiltIn() catalog.BuiltIn {
	return builtin(New())
}
