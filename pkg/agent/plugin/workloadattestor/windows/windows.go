package windows

import "github.com/accuknox/spire/pkg/common/catalog"

const (
	pluginName = "windows"
)

func BuiltIn() catalog.BuiltIn {
	return builtin(New())
}
